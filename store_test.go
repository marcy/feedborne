package main

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := openDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestFeedAndEntryFlow(t *testing.T) {
	db := testDB(t)

	if err := addFeed(db, "https://example.com/feed.rss"); err != nil {
		t.Fatalf("addFeed: %v", err)
	}
	// Duplicate registration is ignored.
	if err := addFeed(db, "https://example.com/feed.rss"); err != nil {
		t.Fatalf("addFeed duplicate: %v", err)
	}
	feeds, err := listFeeds(db)
	if err != nil {
		t.Fatalf("listFeeds: %v", err)
	}
	if len(feeds) != 1 {
		t.Fatalf("expected 1 feed, got %d", len(feeds))
	}
	feedID := feeds[0].ID

	pub := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)
	inserted, err := insertEntry(db, feedID, "g1", "https://example.com/1", "First", "body", &pub)
	if err != nil {
		t.Fatalf("insertEntry: %v", err)
	}
	if !inserted {
		t.Fatal("first insert should report inserted=true")
	}
	inserted, err = insertEntry(db, feedID, "g1", "https://example.com/1", "First", "body", &pub)
	if err != nil {
		t.Fatalf("insertEntry duplicate: %v", err)
	}
	if inserted {
		t.Fatal("duplicate guid insert should report inserted=false")
	}
	earlier := pub.Add(-time.Hour)
	if _, err := insertEntry(db, feedID, "g2", "https://example.com/2", "Second", "", &earlier); err != nil {
		t.Fatalf("insertEntry g2: %v", err)
	}

	entries, err := latestEntries(db, 10, 0)
	if err != nil {
		t.Fatalf("latestEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	// Entries sort newest first by published time.
	if entries[0].GUID != "g1" {
		t.Errorf("expected newest entry g1 first, got %q", entries[0].GUID)
	}
	if entries[0].FeedTitle != "" && entries[0].FeedTitle != feeds[0].Title {
		t.Errorf("unexpected feed title %q", entries[0].FeedTitle)
	}

	page, err := latestEntries(db, 1, 1)
	if err != nil {
		t.Fatalf("latestEntries page: %v", err)
	}
	if len(page) != 1 || page[0].GUID != "g2" {
		t.Errorf("expected second page to be g2, got %+v", page)
	}

	unread, err := unreadCount(db)
	if err != nil {
		t.Fatalf("unreadCount: %v", err)
	}
	if unread != 2 {
		t.Fatalf("expected 2 unread, got %d", unread)
	}
	if err := markRead(db, entries[0].ID); err != nil {
		t.Fatalf("markRead: %v", err)
	}
	unread, err = unreadCount(db)
	if err != nil {
		t.Fatalf("unreadCount after read: %v", err)
	}
	if unread != 1 {
		t.Fatalf("expected 1 unread after markRead, got %d", unread)
	}
	entries, err = latestEntries(db, 10, 0)
	if err != nil {
		t.Fatalf("latestEntries after read: %v", err)
	}
	if !entries[0].Read {
		t.Error("entry should be marked read")
	}
}

func TestShortTextAndFormatTime(t *testing.T) {
	if got := shortText("あいうえお", 3); got != "あいう…" {
		t.Errorf("shortText multibyte: got %q", got)
	}
	if got := shortText("abc", 5); got != "abc" {
		t.Errorf("shortText short input: got %q", got)
	}
	if got := formatTime(sql.NullTime{}); got != "" {
		t.Errorf("formatTime invalid: got %q", got)
	}
}
