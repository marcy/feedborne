package main

import (
	"fmt"
	"net/http"
	"strings"
)

// Lang is a supported UI language.
type Lang string

const (
	LangEN Lang = "en"
	LangJA Lang = "ja"
)

// Messages holds every user-visible UI string for one language.
type Messages struct {
	UnreadFormat string // fmt format with one %d, e.g. "未読 %d 件"
	KeyNext      string
	KeyPrev      string
	KeyOpen      string
	EmptyLead    string // empty-state text before <code>feedborne crawl</code>
	EmptyTail    string // empty-state text after it
	LoadMore     string
	AddFeed      string
	FeedsLabel   string
	AllLabel     string
	ReadLater    string
	SaveToNotion string
	KeySave      string
	KeyBookmark  string
	KeyView      string
	EmptySaved   string
}

// UnreadText formats the unread counter, e.g. "未読 3 件" / "3 unread".
func (m Messages) UnreadText(n int) string {
	return fmt.Sprintf(m.UnreadFormat, n)
}

var catalog = map[Lang]Messages{
	LangEN: {
		UnreadFormat: "%d unread",
		KeyNext:      "next",
		KeyPrev:      "prev",
		KeyOpen:      "open in a new tab",
		EmptyLead:    "No entries yet. Add a feed and run ",
		EmptyTail:    ".",
		LoadMore:     "Load more",
		AddFeed:      "Add feed",
		FeedsLabel:   "Feeds",
		AllLabel:     "All entries",
		ReadLater:    "Read later",
		SaveToNotion: "Save to Notion",
		KeySave:      "read later",
		KeyBookmark:  "bookmark to Notion",
		KeyView:      "switch view",
		EmptySaved:   "Nothing saved yet. Select an entry and press s to save it for later.",
	},
	LangJA: {
		UnreadFormat: "未読 %d 件",
		KeyNext:      "次へ",
		KeyPrev:      "前へ",
		KeyOpen:      "別タブで開く",
		EmptyLead:    "まだ記事がありません。フィードを追加して ",
		EmptyTail:    " を実行してください。",
		LoadMore:     "もっと読む",
		AddFeed:      "フィード追加",
		FeedsLabel:   "フィード",
		AllLabel:     "すべての記事",
		ReadLater:    "あとで読む",
		SaveToNotion: "Notionに保存",
		KeySave:      "あとで読む",
		KeyBookmark:  "Notionに保存",
		KeyView:      "表示切替",
		EmptySaved:   "まだ保存した記事がありません。記事を選んで s キーで保存できます。",
	},
}

// messages returns the catalog entry for l, falling back to English.
func messages(l Lang) Messages {
	if m, ok := catalog[l]; ok {
		return m
	}
	return catalog[LangEN]
}

// detectLang picks the UI language for a request: the "lang" cookie set by
// the /lang switcher wins, then Accept-Language, then English.
func detectLang(r *http.Request) Lang {
	if c, err := r.Cookie("lang"); err == nil {
		if l := Lang(c.Value); l == LangEN || l == LangJA {
			return l
		}
	}
	return parseAcceptLanguage(r.Header.Get("Accept-Language"))
}

// parseAcceptLanguage returns the first supported language in an
// Accept-Language header value, or English when none matches.
func parseAcceptLanguage(header string) Lang {
	for _, part := range strings.Split(header, ",") {
		tag := strings.TrimSpace(strings.Split(part, ";")[0])
		switch {
		case strings.HasPrefix(tag, "ja"):
			return LangJA
		case strings.HasPrefix(tag, "en"):
			return LangEN
		}
	}
	return LangEN
}
