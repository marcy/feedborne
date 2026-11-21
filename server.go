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

//go:embed static
var staticFS embed.FS

const pageSize = 50

type pageData struct {
	Entries    []Entry
	Feeds      []Feed
	Unread     int
	HasMore    bool
	NextOffset int
}

type fragData struct {
	Entries    []Entry
	HasMore    bool
	NextOffset int
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

	renderEntries := func(offset int) (fragData, error) {
		entries, err := latestEntries(db, pageSize, offset)
		if err != nil {
			return fragData{}, err
		}
		return fragData{
			Entries:    entries,
			HasMore:    len(entries) == pageSize,
			NextOffset: offset + len(entries),
		}, nil
	}

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.FileServerFS(staticFS))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		frag, err := renderEntries(0)
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
		data := pageData{
			Entries:    frag.Entries,
			Feeds:      feeds,
			Unread:     unread,
			HasMore:    frag.HasMore,
			NextOffset: frag.NextOffset,
		}
		if err := tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
			log.Printf("template: %v", err)
		}
	})
	mux.HandleFunc("GET /entries", func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if offset < 0 {
			offset = 0
		}
		frag, err := renderEntries(offset)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "entries", frag); err != nil {
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
		unread, err := unreadCount(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "未読 %d 件", unread)
	})
	mux.HandleFunc("POST /feeds", func(w http.ResponseWriter, r *http.Request) {
		url := strings.TrimSpace(r.FormValue("url"))
		if url != "" {
			if err := addFeed(db, url); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		if r.Header.Get("HX-Request") != "true" {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		feeds, err := listFeeds(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "feedsbar", feeds); err != nil {
			log.Printf("template: %v", err)
		}
	})

	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	fmt.Printf("feedborne serving at http://%s\n", addr)
	return srv.ListenAndServe()
}
