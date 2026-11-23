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
	for _, want := range []string{"feedborne", "Hello Entry", "/static/htmx.min.js", "/static/style.css", "/static/app.js", "未読 1 件", `lang="ja"`, `data-theme="dark"`, "フィード追加", "次へ", "フィード", "表示切替"} {
		if !strings.Contains(body, want) {
			t.Errorf("ja index should contain %q", want)
		}
	}

	_, body = getWith(t, srv.URL+"/", "en-US")
	for _, want := range []string{"1 unread", `lang="en"`, "Add feed", "next", "switch view"} {
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

func postForm(t *testing.T, target string) (int, string) {
	t.Helper()
	resp, err := http.PostForm(target, url.Values{})
	if err != nil {
		t.Fatalf("POST %s: %v", target, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func TestSaveEndpointAndSavedFilter(t *testing.T) {
	srv, entryID := testServer(t)
	id := strconv.FormatInt(entryID, 10)

	code, body := postForm(t, srv.URL+"/save?id="+id)
	if code != http.StatusOK {
		t.Fatalf("POST /save: expected 200, got %d", code)
	}
	if !strings.Contains(body, "save-btn") || !strings.Contains(body, "on") {
		t.Errorf("save button should be on after save, got %q", body)
	}

	_, body = getWith(t, srv.URL+"/?filter=saved", "en-US")
	if !strings.Contains(body, "Hello Entry") {
		t.Error("saved view should list the saved entry")
	}

	code, body = postForm(t, srv.URL+"/save?id="+id)
	if code != http.StatusOK {
		t.Fatalf("POST /save toggle off: expected 200, got %d", code)
	}
	_, body = getWith(t, srv.URL+"/?filter=saved", "ja-JP")
	if strings.Contains(body, "Hello Entry") {
		t.Error("saved view should be empty after unsave")
	}
	if !strings.Contains(body, "まだ保存した記事がありません") {
		t.Error("saved view should show the saved empty-state text")
	}

	if code, _ := postForm(t, srv.URL+"/save?id=abc"); code != http.StatusBadRequest {
		t.Errorf("POST /save bad id: expected 400, got %d", code)
	}
}

func TestBookmarkEndpoint(t *testing.T) {
	var calls int
	var lastBody string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		b, _ := io.ReadAll(r.Body)
		lastBody = string(b)
		if r.URL.Path != "/v1/pages" {
			t.Errorf("notion path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("notion auth header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"object":"page","id":"abc"}`))
	}))
	t.Cleanup(fake.Close)
	t.Setenv("FEEDBORNE_NOTION_TOKEN", "test-token")
	t.Setenv("FEEDBORNE_NOTION_DATA_SOURCE_ID", "ds-123")
	t.Setenv("FEEDBORNE_NOTION_API_BASE", fake.URL)

	srv, entryID := testServer(t)
	id := strconv.FormatInt(entryID, 10)

	_, body := get(t, srv.URL+"/")
	if !strings.Contains(body, "bm-btn") {
		t.Error("index should show the Notion bookmark button when configured")
	}

	code, body := postForm(t, srv.URL+"/bookmark?id="+id)
	if code != http.StatusOK {
		t.Fatalf("POST /bookmark: expected 200, got %d (%s)", code, body)
	}
	if calls != 1 {
		t.Fatalf("expected 1 notion call, got %d", calls)
	}
	for _, want := range []string{"ds-123", "Hello Entry", "https://example.com/1", "userDefined:URL"} {
		if !strings.Contains(lastBody, want) {
			t.Errorf("notion payload should contain %q, got %s", want, lastBody)
		}
	}
	if !strings.Contains(body, "on") {
		t.Errorf("bookmark button should be on after bookmarking, got %q", body)
	}

	// Second bookmark is idempotent: no extra API call.
	if _, _ = postForm(t, srv.URL+"/bookmark?id="+id); calls != 1 {
		t.Errorf("second bookmark should not call notion again, calls=%d", calls)
	}
}

func TestBookmarkNotConfigured(t *testing.T) {
	srv, entryID := testServer(t)
	_, body := get(t, srv.URL+"/")
	if strings.Contains(body, "bm-btn") {
		t.Error("index should hide the Notion bookmark button when not configured")
	}
	code, _ := postForm(t, srv.URL+"/bookmark?id="+strconv.FormatInt(entryID, 10))
	if code != http.StatusBadRequest {
		t.Errorf("POST /bookmark unconfigured: expected 400, got %d", code)
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
