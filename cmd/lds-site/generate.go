package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/lstoll/lds.li/internal/email"
)

func runGenerate(ctx context.Context, logger *slog.Logger, args []string) {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	outDir := fs.String("out", "build", "Output directory")
	emailAddr := fs.String("email", os.Getenv("EMAIL_ADDRESS"), "Email address to encrypt")
	fs.Parse(args)
	
	if err := parseEnvFlags(fs); err != nil {
		logger.Error("Failed to parse env flags", "error", err)
		os.Exit(1)
	}

	if *emailAddr == "" {
		logger.Error("Email address is required (via -email or EMAIL_ADDRESS)")
		os.Exit(1)
	}

	if err := generateSite(ctx, logger, *outDir, *emailAddr); err != nil {
		logger.Error("Generation failed", "error", err)
		os.Exit(1)
	}
}

func generateSite(ctx context.Context, logger *slog.Logger, outDir, emailAddr string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate Email Data
	emailData := email.GenerateData(emailAddr)
	logger.Info("Generated email data", "email", emailAddr)

	// Render Index
	tmpl, err := template.ParseFiles("templates/index.tmpl.html")
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	outFile, err := os.Create(filepath.Join(outDir, "index.html"))
	if err != nil {
		return fmt.Errorf("failed to create index.html: %w", err)
	}
	defer outFile.Close()

	if err := tmpl.Execute(outFile, emailData); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}
	logger.Info("Generated index.html")

	// Copy Static
	if err := copyDir("static", filepath.Join(outDir, "static")); err != nil {
		return fmt.Errorf("failed to copy static assets: %w", err)
	}
	logger.Info("Copied static assets")

	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		if _, err := io.Copy(dstFile, srcFile); err != nil {
			return err
		}

		return os.Chmod(dstPath, info.Mode())
	})
}
