package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed static/*
var staticFiles embed.FS

func RegisterRoutes(mux *http.ServeMux, apiTestEnabled bool) error {
	content, err := staticContent()
	if err != nil {
		return err
	}

	fileServer := http.FileServer(http.FS(content))
	mux.Handle("/assets/", assetHandler(content, fileServer))
	if apiTestEnabled {
		mux.HandleFunc("/api-test", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api-test" {
				ServeNotFound(w, r)
				return
			}
			http.ServeFileFS(w, r, content, "api-test.html")
		})
	}
	mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/orders" {
			ServeNotFound(w, r)
			return
		}
		http.ServeFileFS(w, r, content, "orders.html")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			ServeNotFound(w, r)
			return
		}
		http.ServeFileFS(w, r, content, "index.html")
	})
	return nil
}

func ServeLogin(w http.ResponseWriter, r *http.Request) {
	content, err := staticContent()
	if err != nil {
		http.Error(w, "static content unavailable", http.StatusInternalServerError)
		return
	}
	http.ServeFileFS(w, r, content, "login.html")
}

func ServeNotFound(w http.ResponseWriter, r *http.Request) {
	content, err := staticContent()
	if err != nil {
		http.Error(w, "page not found", http.StatusNotFound)
		return
	}
	data, err := fs.ReadFile(content, "404.html")
	if err != nil {
		http.Error(w, "page not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	if r.Method != http.MethodHead {
		_, _ = w.Write(data)
	}
}

func assetHandler(content fs.FS, fileServer http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assetPath := strings.TrimPrefix(r.URL.Path, "/assets/")
		info, err := fs.Stat(content, assetPath)
		if assetPath == "" || err != nil || info.IsDir() {
			ServeNotFound(w, r)
			return
		}
		http.StripPrefix("/assets/", fileServer).ServeHTTP(w, r)
	})
}

func staticContent() (fs.FS, error) {
	content, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, err
	}
	return content, nil
}
