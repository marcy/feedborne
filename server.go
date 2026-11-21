package main

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

//go:embed static
var staticFS embed.FS

const pageSize = 50

type pageData struct {
	Entries    []Entry
	Feeds      []Feed
	Unread     int
	HasMore    bool
	NextOffset int
	Lang       Lang
	Msg        Messages
}

type fragData struct {
	Entries    []Entry
	HasMore    bool
	NextOffset int
	Msg        Messages
}

func formatTime(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Local().Format("01/02 15:04")
}

func shortText(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func newHandler(db *sql.DB) http.Handler {
	renderEntries := func(offset int, msg Messages) (fragData, error) {
		entries, err := latestEntries(db, pageSize, offset)
		if err != nil {
			return fragData{}, err
		}
		return fragData{
			Entries:    entries,
			HasMore:    len(entries) == pageSize,
			NextOffset: offset + len(entries),
			Msg:        msg,
		}, nil
	}
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.FileServerFS(staticFS))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		lang := detectLang(r)
		frag, err := renderEntries(0, messages(lang))
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
			Lang:       lang,
			Msg:        messages(lang),
		}
		if err := IndexPage(data).Render(r.Context(), w); err != nil {
			log.Printf("render index: %v", err)
		}
	})
	mux.HandleFunc("GET /entries", func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if offset < 0 {
			offset = 0
		}
		frag, err := renderEntries(offset, messages(detectLang(r)))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := EntriesFrag(frag).Render(r.Context(), w); err != nil {
			log.Printf("render entries: %v", err)
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
		fmt.Fprint(w, messages(detectLang(r)).UnreadText(unread))
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
		if err := FeedsBar(feeds, messages(detectLang(r))).Render(r.Context(), w); err != nil {
			log.Printf("render feedsbar: %v", err)
		}
	})
	mux.HandleFunc("GET /lang", func(w http.ResponseWriter, r *http.Request) {
		lang := Lang(r.URL.Query().Get("set"))
		if lang != LangEN && lang != LangJA {
			lang = LangEN
		}
		http.SetCookie(w, &http.Cookie{
			Name:   "lang",
			Value:  string(lang),
			Path:   "/",
			MaxAge: 365 * 24 * 60 * 60,
		})
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
	return mux
}

func runServe(dbPath, addr string) error {
	db, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	srv := &http.Server{Addr: addr, Handler: newHandler(db), ReadHeaderTimeout: 10 * time.Second}
	fmt.Printf("feedborne serving at http://%s\n", addr)
	return srv.ListenAndServe()
}
