package main

import (
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

//go:embed templates
var templatesFS embed.FS

type pageData struct {
	Entries []Entry
	Feeds   []Feed
	Unread  int
}

func runServe(dbPath, addr string) error {
	db, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"fmtTime": func(t sql.NullTime) string {
			if !t.Valid {
				return ""
			}
			return t.Time.Local().Format("01/02 15:04")
		},
		"short": func(s string, n int) string {
			r := []rune(s)
			if len(r) <= n {
				return s
			}
			return string(r[:n]) + "…"
		},
	}).ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		entries, err := latestEntries(db, 200)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		feeds, err := listFeeds(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		unread, err := unreadCount(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data := pageData{Entries: entries, Feeds: feeds, Unread: unread}
		if err := tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
			log.Printf("template: %v", err)
		}
	})
	mux.HandleFunc("POST /read", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		if err := markRead(db, id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /feeds", func(w http.ResponseWriter, r *http.Request) {
		url := strings.TrimSpace(r.FormValue("url"))
		if url == "" {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		if err := addFeed(db, url); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	fmt.Printf("feedborne serving at http://%s\n", addr)
	return srv.ListenAndServe()
}
