package main

import (
	"context"
	"database/sql"
	_ "embed"
	"time"

	"feedborne/internal/db"

	_ "modernc.org/sqlite"
)

//go:embed db/schema.sql
var schemaSQL string

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
	database, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	if _, err := database.Exec(schemaSQL); err != nil {
		return nil, err
	}
	return database, nil
}

func queries(database *sql.DB) *db.Queries {
	return db.New(database)
}

func listFeeds(database *sql.DB) ([]Feed, error) {
	rows, err := queries(database).ListFeeds(context.Background())
	if err != nil {
		return nil, err
	}
	feeds := make([]Feed, 0, len(rows))
	for _, r := range rows {
		feeds = append(feeds, Feed{
			ID:           r.ID,
			URL:          r.Url,
			Title:        r.Title,
			ETag:         r.Etag,
			LastModified: r.LastModified,
			LastFetched:  r.LastFetchedAt,
		})
	}
	return feeds, nil
}

func addFeed(database *sql.DB, url string) error {
	return queries(database).InsertFeed(context.Background(), url)
}

func insertEntry(database *sql.DB, feedID int64, guid, url, title, desc string, published *time.Time) (bool, error) {
	var pub sql.NullTime
	if published != nil {
		pub = sql.NullTime{Time: *published, Valid: true}
	}
	n, err := queries(database).InsertEntry(context.Background(), db.InsertEntryParams{
		FeedID:      feedID,
		Guid:        guid,
		Url:         url,
		Title:       title,
		Description: desc,
		PublishedAt: pub,
	})
	return n > 0, err
}

func updateFeedAfterFetch(database *sql.DB, f Feed, title, etag, lastMod string) error {
	return queries(database).UpdateFeedAfterFetch(context.Background(), db.UpdateFeedAfterFetchParams{
		Title:        title,
		Etag:         etag,
		LastModified: lastMod,
		ID:           f.ID,
	})
}

func latestEntries(database *sql.DB, limit, offset int) ([]Entry, error) {
	rows, err := queries(database).LatestEntries(context.Background(), db.LatestEntriesParams{
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(rows))
	for _, r := range rows {
		entries = append(entries, Entry{
			ID:          r.ID,
			FeedID:      r.FeedID,
			FeedTitle:   r.FeedTitle,
			GUID:        r.Guid,
			URL:         r.Url,
			Title:       r.Title,
			Description: r.Description,
			PublishedAt: r.PublishedAt,
			Read:        r.Read != 0,
		})
	}
	return entries, nil
}

func markRead(database *sql.DB, id int64) error {
	return queries(database).MarkRead(context.Background(), id)
}

func unreadCount(database *sql.DB) (int, error) {
	n, err := queries(database).UnreadCount(context.Background())
	return int(n), err
}
