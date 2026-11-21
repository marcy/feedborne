package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mmcdole/gofeed"
)

const testRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel>
<title>Test Feed</title><link>https://example.com/</link><description>d</description>
<item><title>First</title><link>https://example.com/1</link><guid>g1</guid><pubDate>Fri, 21 Nov 2026 10:00:00 +0900</pubDate><description>hello &lt;b&gt;world&lt;/b&gt;</description></item>
<item><title>Second</title><link>https://example.com/2</link><guid>g2</guid><pubDate>Fri, 21 Nov 2026 11:00:00 +0900</pubDate><description>two</description></item>
</channel></rss>`

func TestStripTags(t *testing.T) {
	if got := stripTags("hello <b>world</b>\n  again"); got != "hello world again" {
		t.Errorf("stripTags: got %q", got)
	}
	if got := stripTags("plain"); got != "plain" {
		t.Errorf("stripTags plain: got %q", got)
	}
}

func TestCrawlFeed(t *testing.T) {
	db := testDB(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(testRSS))
	}))
	defer srv.Close()

	if err := addFeed(db, srv.URL); err != nil {
		t.Fatalf("addFeed: %v", err)
	}
	feeds, err := listFeeds(db)
	if err != nil || len(feeds) != 1 {
		t.Fatalf("listFeeds: %v (%d feeds)", err, len(feeds))
	}

	n, err := crawlFeed(srv.Client(), gofeed.NewParser(), db, feeds[0])
	if err != nil {
		t.Fatalf("crawlFeed: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 new entries, got %d", n)
	}

	entries, err := latestEntries(db, 10, 0)
	if err != nil {
		t.Fatalf("latestEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 stored entries, got %d", len(entries))
	}
	if entries[0].Title != "Second" {
		t.Errorf("expected newest entry first, got %q", entries[0].Title)
	}
	var first *Entry
	for i := range entries {
		if entries[i].GUID == "g1" {
			first = &entries[i]
		}
	}
	if first == nil || first.Description != "hello world" {
		t.Errorf("expected stripped description %q, got %+v", "hello world", first)
	}

	// Feed metadata (title, ETag) was stored.
	feeds, err = listFeeds(db)
	if err != nil {
		t.Fatalf("listFeeds after crawl: %v", err)
	}
	if feeds[0].Title != "Test Feed" {
		t.Errorf("expected feed title stored, got %q", feeds[0].Title)
	}
	if feeds[0].ETag != `"v1"` {
		t.Errorf("expected etag stored, got %q", feeds[0].ETag)
	}

	// Second crawl hits the 304 path and adds nothing.
	n, err = crawlFeed(srv.Client(), gofeed.NewParser(), db, feeds[0])
	if err != nil {
		t.Fatalf("crawlFeed second: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 new entries on 304, got %d", n)
	}
	if !strings.Contains(testRSS, "Test Feed") {
		t.Fatal("fixture sanity")
	}
}
