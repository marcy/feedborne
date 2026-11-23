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
	Theme      Theme
	Filter     string
	SavedCount int
	Notion     bool
}

type fragData struct {
	Entries    []Entry
	HasMore    bool
	NextOffset int
	Msg        Messages
	Filter     string
	Notion     bool
}

// entriesURL builds the load-more fragment URL, preserving the filter.
func entriesURL(offset int, filter string) string {
	u := "/entries?offset=" + strconv.Itoa(offset)
	if filter == "saved" {
		u += "&filter=saved"
	}
	return u
}

// bookmarkTitle is the bookmark button tooltip: the action label, or the
// error text when the last Notion request failed.
func bookmarkTitle(msg Messages, errText string) string {
	if errText != "" {
		return errText
	}
	return msg.SaveToNotion + " (b)"
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
	notionCfg, notionOK := notionFromEnv()
	renderEntries := func(offset int, msg Messages, filter string) (fragData, error) {
		var entries []Entry
		var err error
		if filter == "saved" {
			entries, err = savedEntries(db, pageSize, offset)
		} else {
			entries, err = latestEntries(db, pageSize, offset)
		}
		if err != nil {
			return fragData{}, err
		}
		return fragData{
			Entries:    entries,
			HasMore:    len(entries) == pageSize,
			NextOffset: offset + len(entries),
			Msg:        msg,
			Filter:     filter,
			Notion:     notionOK,
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
		filter := r.URL.Query().Get("filter")
		if filter != "saved" {
			filter = "all"
		}
		frag, err := renderEntries(0, messages(lang), filter)
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
		saved, err := savedCount(db)
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
			Theme:      detectTheme(r),
			Filter:     filter,
			SavedCount: saved,
			Notion:     notionOK,
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
		filter := r.URL.Query().Get("filter")
		if filter != "saved" {
			filter = "all"
		}
		frag, err := renderEntries(offset, messages(detectLang(r)), filter)
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
	mux.HandleFunc("POST /save", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		entry, err := getEntry(db, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := setSaved(db, id, !entry.Saved); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		entry.Saved = !entry.Saved
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := SaveButton(entry, messages(detectLang(r))).Render(r.Context(), w); err != nil {
			log.Printf("render save button: %v", err)
		}
	})
	mux.HandleFunc("POST /bookmark", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		if !notionOK {
			http.Error(w, "notion bookmark is not configured", http.StatusBadRequest)
			return
		}
		entry, err := getEntry(db, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		msg := messages(detectLang(r))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if entry.Bookmarked {
			if err := BookmarkButton(entry, msg, "").Render(r.Context(), w); err != nil {
				log.Printf("render bookmark button: %v", err)
			}
			return
		}
		if err := notionCfg.createBookmarkPage(r.Context(), entry.Title, entry.URL); err != nil {
			log.Printf("bookmark entry %d: %v", id, err)
			if rerr := BookmarkButton(entry, msg, err.Error()).Render(r.Context(), w); rerr != nil {
				log.Printf("render bookmark button: %v", rerr)
			}
			return
		}
		if err := setBookmarked(db, id, true); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		entry.Bookmarked = true
		if err := BookmarkButton(entry, msg, "").Render(r.Context(), w); err != nil {
			log.Printf("render bookmark button: %v", err)
		}
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
		saved, err := savedCount(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		filter := r.FormValue("filter")
		if filter != "saved" {
			filter = "all"
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := FeedsSidebar(feeds, messages(detectLang(r)), filter, saved, notionOK).Render(r.Context(), w); err != nil {
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
	mux.HandleFunc("GET /theme", func(w http.ResponseWriter, r *http.Request) {
		theme := Theme(r.URL.Query().Get("set"))
		if !theme.valid() {
			theme = ThemeDark
		}
		http.SetCookie(w, &http.Cookie{
			Name:   "theme",
			Value:  string(theme),
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
