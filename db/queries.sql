-- name: ListFeeds :many
SELECT id, url, title, etag, last_modified, last_fetched_at
FROM feeds
ORDER BY id;

-- name: InsertFeed :exec
INSERT OR IGNORE INTO feeds (url) VALUES (?);

-- name: UpdateFeedAfterFetch :exec
UPDATE feeds
SET title = ?, etag = ?, last_modified = ?, last_fetched_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: InsertEntry :execrows
INSERT OR IGNORE INTO entries (feed_id, guid, url, title, description, published_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: LatestEntries :many
SELECT e.id, e.feed_id, f.title AS feed_title, e.guid, e.url, e.title,
       e.description, e.published_at, e.read
FROM entries e
JOIN feeds f ON f.id = e.feed_id
ORDER BY COALESCE(e.published_at, e.fetched_at) DESC, e.id DESC
LIMIT ? OFFSET ?;

-- name: MarkRead :exec
UPDATE entries SET read = 1 WHERE id = ?;

-- name: UnreadCount :one
SELECT COUNT(*) FROM entries WHERE read = 0;
