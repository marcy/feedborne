package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func testServer(t *testing.T) (*httptest.Server, int64) {
	t.Helper()
	db := testDB(t)
	if err := addFeed(db, "https://example.com/feed.rss"); err != nil {
		t.Fatalf("addFeed: %v", err)
	}
	feeds, err := listFeeds(db)
	if err != nil || len(feeds) != 1 {
		t.Fatalf("listFeeds: %v", err)
	}
	if _, err := insertEntry(db, feeds[0].ID, "g1", "https://example.com/1", "Hello Entry", "body", nil); err != nil {
		t.Fatalf("insertEntry: %v", err)
	}
	entries, err := latestEntries(db, 1, 0)
	if err != nil || len(entries) != 1 {
		t.Fatalf("latestEntries: %v", err)
	}
	srv := httptest.NewServer(newHandler(db))
	t.Cleanup(srv.Close)
	return srv, entries[0].ID
}

func get(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, string(body)
}

func getWith(t *testing.T, url, acceptLang string, cookies ...*http.Cookie) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if acceptLang != "" {
		req.Header.Set("Accept-Language", acceptLang)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, string(body)
}

func TestIndexPage(t *testing.T) {
	srv, _ := testServer(t)
	code, body := getWith(t, srv.URL+"/", "ja-JP")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	for _, want := range []string{"FEEDBORNE", "Hello Entry", "/static/htmx.min.js", "/static/style.css", "/static/app.js", "未読 1 件", `lang="ja"`, `data-theme="dark"`, "フィード追加", "次へ"} {
		if !strings.Contains(body, want) {
			t.Errorf("ja index should contain %q", want)
		}
	}

	_, body = getWith(t, srv.URL+"/", "en-US")
	for _, want := range []string{"1 unread", `lang="en"`, "Add feed", "next"} {
		if !strings.Contains(body, want) {
			t.Errorf("en index should contain %q", want)
		}
	}
}

func TestStaticFiles(t *testing.T) {
	srv, _ := testServer(t)
	for _, path := range []string{"/static/htmx.min.js", "/static/style.css", "/static/app.js"} {
		code, body := get(t, srv.URL+path)
		if code != http.StatusOK {
			t.Errorf("%s: expected 200, got %d", path, code)
		}
		if len(body) == 0 {
			t.Errorf("%s: empty body", path)
		}
	}
}

func TestEntriesFragment(t *testing.T) {
	srv, _ := testServer(t)
	code, body := get(t, srv.URL+"/entries?offset=0")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if !strings.Contains(body, "Hello Entry") {
		t.Error("fragment should contain the entry")
	}
	if strings.Contains(body, "<html") {
		t.Error("fragment should not be a full page")
	}
}

func postRead(t *testing.T, srvURL string, entryID int64, lang Lang) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, srvURL+"/read",
		strings.NewReader(url.Values{"id": {strconv.FormatInt(entryID, 10)}}.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if lang != "" {
		req.AddCookie(&http.Cookie{Name: "lang", Value: string(lang)})
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /read: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func TestReadEndpoint(t *testing.T) {
	srv, entryID := testServer(t)
	code, body := postRead(t, srv.URL, entryID, LangJA)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if body != "未読 0 件" {
		t.Errorf("expected ja unread count text, got %q", body)
	}
	code, body = postRead(t, srv.URL, entryID, LangEN)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if body != "0 unread" {
		t.Errorf("expected en unread count text, got %q", body)
	}

	resp2, err := http.PostForm(srv.URL+"/read", url.Values{"id": {"abc"}})
	if err != nil {
		t.Fatalf("POST /read bad id: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for bad id, got %d", resp2.StatusCode)
	}
}

func TestFeedsEndpointHX(t *testing.T) {
	srv, _ := testServer(t)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/feeds",
		strings.NewReader(url.Values{"url": {"https://example.org/new.rss"}}.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /feeds: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), `id="feedsbar"`) || !strings.Contains(string(body), "https://example.org/new.rss") {
		t.Errorf("expected updated feedsbar fragment, got %q", string(body))
	}
}

func TestLangSwitch(t *testing.T) {
	srv, _ := testServer(t)
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(srv.URL + "/lang?set=ja")
	if err != nil {
		t.Fatalf("GET /lang: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", resp.StatusCode)
	}
	var langCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "lang" {
			langCookie = c
		}
	}
	if langCookie == nil || langCookie.Value != "ja" {
		t.Fatalf("expected lang=ja cookie, got %v", resp.Cookies())
	}
	_, body := getWith(t, srv.URL+"/", "", langCookie)
	if !strings.Contains(body, "未読 1 件") {
		t.Error("index with lang cookie should be Japanese")
	}

	resp2, err := client.Get(srv.URL + "/lang?set=fr")
	if err != nil {
		t.Fatalf("GET /lang bad value: %v", err)
	}
	defer resp2.Body.Close()
	for _, c := range resp2.Cookies() {
		if c.Name == "lang" && c.Value != "en" {
			t.Errorf("invalid set value should fall back to en, got %q", c.Value)
		}
	}
}

func TestThemeSwitch(t *testing.T) {
	srv, _ := testServer(t)
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(srv.URL + "/theme?set=sepia")
	if err != nil {
		t.Fatalf("GET /theme: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", resp.StatusCode)
	}
	var themeCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "theme" {
			themeCookie = c
		}
	}
	if themeCookie == nil || themeCookie.Value != "sepia" {
		t.Fatalf("expected theme=sepia cookie, got %v", resp.Cookies())
	}
	_, body := getWith(t, srv.URL+"/", "", themeCookie)
	if !strings.Contains(body, `data-theme="sepia"`) {
		t.Error("index with theme cookie should render the sepia theme")
	}

	resp2, err := client.Get(srv.URL + "/theme?set=neon")
	if err != nil {
		t.Fatalf("GET /theme bad value: %v", err)
	}
	defer resp2.Body.Close()
	for _, c := range resp2.Cookies() {
		if c.Name == "theme" && c.Value != "dark" {
			t.Errorf("invalid set value should fall back to dark, got %q", c.Value)
		}
	}
}
