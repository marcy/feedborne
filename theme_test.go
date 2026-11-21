package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestThemeValidAndNames(t *testing.T) {
	for _, th := range themes {
		if !th.valid() {
			t.Errorf("%q should be valid", th)
		}
		if th.Name() == "" {
			t.Errorf("%q should have a name", th)
		}
	}
	if Theme("neon").valid() {
		t.Error("unknown theme should be invalid")
	}
	if len(themes) != 3 {
		t.Errorf("expected 3 themes (dark + 2 added), got %d", len(themes))
	}
}

func TestDetectTheme(t *testing.T) {
	req := func(cookie *http.Cookie) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if cookie != nil {
			r.AddCookie(cookie)
		}
		return r
	}
	if got := detectTheme(req(nil)); got != ThemeDark {
		t.Errorf("no cookie should default to dark, got %q", got)
	}
	if got := detectTheme(req(&http.Cookie{Name: "theme", Value: "sepia"})); got != ThemeSepia {
		t.Errorf("sepia cookie: got %q", got)
	}
	if got := detectTheme(req(&http.Cookie{Name: "theme", Value: "neon"})); got != ThemeDark {
		t.Errorf("invalid cookie should fall back to dark, got %q", got)
	}
}
