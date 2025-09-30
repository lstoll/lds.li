package main

import (
	"context"
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lds.li/web/requestlog"
)

//go:embed static/*
var staticFS embed.FS

//go:embed templates/*
var templateFS embed.FS

func main() {
	logh := requestlog.RequestLogger{
		Logger: slog.With("component", "requestlog"),
	}

	// Calculate email data
	email := os.Getenv("EMAIL_ADDRESS")
	emailData := generateEmailData(email)
	slog.Info("Generated email data", "email", email, "challenge", emailData.Challenge)

	mux := http.NewServeMux()

	templateFiles, err := fs.Sub(templateFS, "templates")
	if err != nil {
		slog.Error("Failed to create template filesystem", "error", err)
		os.Exit(1)
	}

	tmpl, err := template.ParseFS(templateFiles, "*.tmpl.html")
	if err != nil {
		slog.Error("Failed to parse template", "error", err)
		os.Exit(1)
	}

	staticFiles, err := fs.Sub(staticFS, "static")
	if err != nil {
		slog.Error("Failed to create static filesystem", "error", err)
		os.Exit(1)
	}

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFiles))))
	mux.HandleFunc("GET /.well-known/webfinger", webfingerHandler)
	registerModuleRoutes(mux)

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if len(r.Host) > 4 && r.Host[:4] == "www." {
			redirectURL := "https://" + r.Host[4:] + r.URL.Path
			http.Redirect(w, r, redirectURL, http.StatusMovedPermanently)
			return
		}
		if err := tmpl.Execute(w, emailData); err != nil {
			slog.Error("Failed to execute template", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: logh.Handler(mux),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("Starting HTTP server", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("Server stopped")
}
