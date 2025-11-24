package main

import (
	"html/template"
	"log/slog"
	"net/http"
)

// ModuleInfo represents metadata for a Go module
type ModuleInfo struct {
	Path       string // e.g., "lds.li/oauth2ext"
	Title      string // e.g., "lds.li/oauth2ext"
	GitURL     string // e.g., "https://github.com/lstoll/oauth2ext"
	RedirectTo string // e.g., "https://pkg.go.dev/lds.li/oauth2ext"
}

// ModuleRegistry holds all module metadata
var ModuleRegistry = map[string]ModuleInfo{
	"oauth2ext": {
		Path:       "lds.li/oauth2ext",
		Title:      "lds.li/oauth2ext",
		GitURL:     "https://github.com/lstoll/oauth2ext",
		RedirectTo: "https://pkg.go.dev/lds.li/oauth2ext",
	},
	"web": {
		Path:       "lds.li/web",
		Title:      "lds.li/web",
		GitURL:     "https://github.com/lstoll/web",
		RedirectTo: "https://pkg.go.dev/lds.li/web",
	},
	"oidccli": {
		Path:       "lds.li/oidccli",
		Title:      "lds.li/oidccli",
		GitURL:     "https://github.com/lstoll/oidccli",
		RedirectTo: "https://github.com/lstoll/oidccli",
	},
}

// moduleHandler handles requests for Go module metadata
func moduleHandler(moduleInfo ModuleInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		htmlTemplate := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>{{.Title}}</title>
  <meta name="go-import" content="{{.Path}} git {{.GitURL}}">
  <meta http-equiv="refresh" content="0; url={{.RedirectTo}}">
</head>
<body>
  <p>Redirecting to <a href="{{.RedirectTo}}">{{.RedirectTo}}</a>...</p>
</body>
</html>`

		tmpl, err := template.New("module").Parse(htmlTemplate)
		if err != nil {
			slog.Error("Failed to parse module template", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, moduleInfo); err != nil {
			slog.Error("Failed to execute module template", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}
}

// registerModuleRoutes adds routes for all modules in the registry
func registerModuleRoutes(mux *http.ServeMux) {
	for moduleName, moduleInfo := range ModuleRegistry {
		route := "/" + moduleName
		mux.HandleFunc("GET "+route, moduleHandler(moduleInfo))
		slog.Info("Registered module route", "route", route, "module", moduleInfo.Path)
	}
}
