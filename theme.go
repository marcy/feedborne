package main

import "net/http"

// Theme is a color scheme for the web UI.
type Theme string

const (
	ThemeDark  Theme = "dark"
	ThemeLight Theme = "light"
	ThemeSepia Theme = "sepia"
)

// themes lists every selectable theme, in switcher order.
var themes = []Theme{ThemeDark, ThemeLight, ThemeSepia}

func (t Theme) valid() bool {
	for _, x := range themes {
		if t == x {
			return true
		}
	}
	return false
}

// Name returns the switcher label. Theme names are proper nouns and stay
// in English for every UI language.
func (t Theme) Name() string {
	switch t {
	case ThemeLight:
		return "Light"
	case ThemeSepia:
		return "Sepia"
	default:
		return "Dark"
	}
}

// detectTheme reads the "theme" cookie set by the /theme switcher.
// The original dark scheme is the default.
func detectTheme(r *http.Request) Theme {
	if c, err := r.Cookie("theme"); err == nil {
		if t := Theme(c.Value); t.valid() {
			return t
		}
	}
	return ThemeDark
}
