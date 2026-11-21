package main

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

var tagRe = regexp.MustCompile(`<[^>]*>`)

func stripTags(s string) string {
	s = tagRe.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

func runCrawl(dbPath string) error {
	db, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	feeds, err := listFeeds(db)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	parser := gofeed.NewParser()
	totalNew := 0
	for _, f := range feeds {
		n, err := crawlFeed(client, parser, db, f)
		if err != nil {
			fmt.Printf("%s: error: %v\n", f.URL, err)
			continue
		}
		fmt.Printf("%s: %d new entries\n", f.URL, n)
		totalNew += n
	}
	fmt.Printf("done: %d new entries total\n", totalNew)
	return nil
}

func crawlFeed(client *http.Client, parser *gofeed.Parser, db *sql.DB, f Feed) (int, error) {
	req, err := http.NewRequest(http.MethodGet, f.URL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "feedborne/0.1 (+https://github.com/marcy/feedborne)")
	if f.ETag != "" {
		req.Header.Set("If-None-Match", f.ETag)
	}
	if f.LastModified != "" {
		req.Header.Set("If-Modified-Since", f.LastModified)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return 0, nil
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	feed, err := parser.ParseString(string(body))
	if err != nil {
		return 0, err
	}
	newCount := 0
	// Insert oldest first so ids roughly follow publication order.
	for i := len(feed.Items) - 1; i >= 0; i-- {
		item := feed.Items[i]
		guid := item.GUID
		if guid == "" {
			guid = item.Link
		}
		var published *time.Time
		if item.PublishedParsed != nil {
			published = item.PublishedParsed
		} else if item.UpdatedParsed != nil {
			published = item.UpdatedParsed
		}
		desc := item.Description
		if desc == "" {
			desc = item.Content
		}
		inserted, err := insertEntry(db, f.ID, guid, item.Link, item.Title, stripTags(desc), published)
		if err != nil {
			return newCount, err
		}
		if inserted {
			newCount++
		}
	}
	title := feed.Title
	if title == "" {
		title = f.Title
	}
	if err := updateFeedAfterFetch(db, f, title, resp.Header.Get("ETag"), resp.Header.Get("Last-Modified")); err != nil {
		return newCount, err
	}
	return newCount, nil
}

func runFeedAdd(dbPath, url string) error {
	db, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := addFeed(db, url); err != nil {
		return err
	}
	fmt.Printf("added: %s\n", url)
	return nil
}

func runFeedList(dbPath string) error {
	db, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	feeds, err := listFeeds(db)
	if err != nil {
		return err
	}
	for _, f := range feeds {
		fmt.Printf("%d\t%s\t%s\n", f.ID, f.Title, f.URL)
	}
	return nil
}
