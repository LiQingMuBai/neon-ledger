package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var staticFiles embed.FS

func RegisterRoutes(mux *http.ServeMux, apiTestEnabled bool) error {
	content, err := staticContent()
	if err != nil {
		return err
	}

	fileServer := http.FileServer(http.FS(content))
	mux.Handle("/assets/", http.StripPrefix("/assets/", fileServer))
	if apiTestEnabled {
		mux.HandleFunc("/api-test", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api-test" {
				http.NotFound(w, r)
				return
			}
			http.ServeFileFS(w, r, content, "api-test.html")
		})
	}
	mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/orders" {
			http.NotFound(w, r)
			return
		}
		http.ServeFileFS(w, r, content, "orders.html")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
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

func staticContent() (fs.FS, error) {
	content, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, err
	}
	return content, nil
}
