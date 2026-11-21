package main

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type Feed struct {
	ID           int64
	URL          string
	Title        string
	ETag         string
	LastModified string
	LastFetched  sql.NullTime
}

type Entry struct {
	ID          int64
	FeedID      int64
	FeedTitle   string
	GUID        string
	URL         string
	Title       string
	Description string
	PublishedAt sql.NullTime
	Read        bool
}

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS feeds (
  id INTEGER PRIMARY KEY,
  url TEXT NOT NULL UNIQUE,
  title TEXT NOT NULL DEFAULT '',
  etag TEXT NOT NULL DEFAULT '',
  last_modified TEXT NOT NULL DEFAULT '',
  last_fetched_at TIMESTAMP
);
CREATE TABLE IF NOT EXISTS entries (
  id INTEGER PRIMARY KEY,
  feed_id INTEGER NOT NULL REFERENCES feeds(id),
  guid TEXT NOT NULL,
  url TEXT NOT NULL,
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  published_at TIMESTAMP,
  fetched_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  read INTEGER NOT NULL DEFAULT 0,
  UNIQUE(feed_id, guid)
);
CREATE INDEX IF NOT EXISTS idx_entries_published ON entries(published_at DESC, id DESC);
`)
	if err != nil {
		return nil, err
	}
	return db, nil
}

const defaultFeedURL = "https://b.hatena.ne.jp/hotentry/it.rss"

func ensureDefaultFeed(db *sql.DB) error {
	_, err := db.Exec(`INSERT OR IGNORE INTO feeds(url) VALUES(?)`, defaultFeedURL)
	return err
}

func listFeeds(db *sql.DB) ([]Feed, error) {
	rows, err := db.Query(`SELECT id, url, title, etag, last_modified, last_fetched_at FROM feeds ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var feeds []Feed
	for rows.Next() {
		var f Feed
		if err := rows.Scan(&f.ID, &f.URL, &f.Title, &f.ETag, &f.LastModified, &f.LastFetched); err != nil {
			return nil, err
		}
		feeds = append(feeds, f)
	}
	return feeds, rows.Err()
}

func addFeed(db *sql.DB, url string) error {
	_, err := db.Exec(`INSERT OR IGNORE INTO feeds(url) VALUES(?)`, url)
	return err
}

func insertEntry(db *sql.DB, feedID int64, guid, url, title, desc string, published *time.Time) (bool, error) {
	var pub any
	if published != nil {
		pub = *published
	}
	res, err := db.Exec(`INSERT OR IGNORE INTO entries(feed_id, guid, url, title, description, published_at) VALUES(?,?,?,?,?,?)`,
		feedID, guid, url, title, desc, pub)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func updateFeedAfterFetch(db *sql.DB, f Feed, title, etag, lastMod string) error {
	_, err := db.Exec(`UPDATE feeds SET title=?, etag=?, last_modified=?, last_fetched_at=CURRENT_TIMESTAMP WHERE id=?`,
		title, etag, lastMod, f.ID)
	return err
}

func latestEntries(db *sql.DB, limit int) ([]Entry, error) {
	rows, err := db.Query(`
SELECT e.id, e.feed_id, f.title, e.guid, e.url, e.title, e.description, e.published_at, e.read
FROM entries e JOIN feeds f ON f.id = e.feed_id
ORDER BY COALESCE(e.published_at, e.fetched_at) DESC, e.id DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []Entry
	for rows.Next() {
		var e Entry
		var read int
		if err := rows.Scan(&e.ID, &e.FeedID, &e.FeedTitle, &e.GUID, &e.URL, &e.Title, &e.Description, &e.PublishedAt, &read); err != nil {
			return nil, err
		}
		e.Read = read != 0
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func markRead(db *sql.DB, id int64) error {
	_, err := db.Exec(`UPDATE entries SET read=1 WHERE id=?`, id)
	return err
}

func unreadCount(db *sql.DB) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM entries WHERE read=0`).Scan(&n)
	return n, err
}
