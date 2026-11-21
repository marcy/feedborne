# feedborne

A small RSS reader written in Go.

- Single binary with subcommands: `crawl` fetches feeds, `serve` runs the web UI
- Storage: SQLite (pure-Go driver, no cgo)
- Frontend: server-rendered HTML (`html/template`) + a pinch of vanilla JS. No framework.
- Keyboard-driven reading: `j` / `k` move the selection, `o` opens the entry in a new tab (and marks it read)

## Usage

```sh
go build -o feedborne .

# fetch all registered feeds (a default feed is registered on first run)
./feedborne crawl

# add another feed
./feedborne feed add https://example.com/feed.rss

# read
./feedborne serve -addr 127.0.0.1:8080
```

The default feed is Hatena Bookmark's popular tech entries
(`https://b.hatena.ne.jp/hotentry/it.rss`), the tech feed that posts to our
Slack `#feed` channel.

## Layout

- `main.go` — subcommand dispatch
- `crawl.go` — fetching (ETag / Last-Modified aware) and parsing via gofeed
- `store.go` — SQLite schema and queries
- `server.go` — HTTP handlers
- `templates/index.html` — the single page, styles and key handling included
