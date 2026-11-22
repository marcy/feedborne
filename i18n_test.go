package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCatalogComplete(t *testing.T) {
	for _, lang := range []Lang{LangEN, LangJA} {
		m, ok := catalog[lang]
		if !ok {
			t.Fatalf("catalog missing %q", lang)
		}
		fields := map[string]string{
			"UnreadFormat": m.UnreadFormat,
			"KeyNext":      m.KeyNext,
			"KeyPrev":      m.KeyPrev,
			"KeyOpen":      m.KeyOpen,
			"EmptyLead":    m.EmptyLead,
			"EmptyTail":    m.EmptyTail,
			"LoadMore":     m.LoadMore,
			"AddFeed":      m.AddFeed,
		}
		for name, v := range fields {
			if v == "" {
				t.Errorf("%s: %s is empty", lang, name)
			}
		}
	}
}

func TestUnreadText(t *testing.T) {
	if got := messages(LangJA).UnreadText(3); got != "未読 3 件" {
		t.Errorf("ja unread: got %q", got)
	}
	if got := messages(LangEN).UnreadText(3); got != "3 unread" {
		t.Errorf("en unread: got %q", got)
	}
	if got := messages(Lang("fr")).UnreadText(0); got != "0 unread" {
		t.Errorf("unknown lang should fall back to en, got %q", got)
	}
}

func TestParseAcceptLanguage(t *testing.T) {
	cases := []struct {
		header string
		want   Lang
	}{
		{"ja-JP,ja;q=0.9,en;q=0.8", LangJA},
		{"en-US,en;q=0.9", LangEN},
		{"en-GB", LangEN},
		{"ja", LangJA},
		{"fr-FR,fr;q=0.9", LangEN},
		{"fr-FR,ja;q=0.5", LangJA},
		{"", LangEN},
		{"*", LangEN},
	}
	for _, c := range cases {
		if got := parseAcceptLanguage(c.header); got != c.want {
			t.Errorf("parseAcceptLanguage(%q) = %q, want %q", c.header, got, c.want)
		}
	}
}

func TestDetectLang(t *testing.T) {
	req := func(accept string, cookie *http.Cookie) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if accept != "" {
			r.Header.Set("Accept-Language", accept)
		}
		if cookie != nil {
			r.AddCookie(cookie)
		}
		return r
	}
	if got := detectLang(req("ja-JP", nil)); got != LangJA {
		t.Errorf("header ja: got %q", got)
	}
	if got := detectLang(req("en-US", &http.Cookie{Name: "lang", Value: "ja"})); got != LangJA {
		t.Errorf("cookie should win over header: got %q", got)
	}
	if got := detectLang(req("ja-JP", &http.Cookie{Name: "lang", Value: "fr"})); got != LangJA {
		t.Errorf("invalid cookie should fall through to header: got %q", got)
	}
	if got := detectLang(req("", nil)); got != LangEN {
		t.Errorf("default: got %q", got)
	}
}
