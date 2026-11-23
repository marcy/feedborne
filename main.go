package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "crawl":
		fs := flag.NewFlagSet("crawl", flag.ExitOnError)
		dbPath := fs.String("db", "feedborne.db", "SQLite database path")
		fs.Parse(os.Args[2:])
		if err := runCrawl(*dbPath); err != nil {
			log.Fatal(err)
		}
	case "serve":
		fs := flag.NewFlagSet("serve", flag.ExitOnError)
		dbPath := fs.String("db", "feedborne.db", "SQLite database path")
		addr := fs.String("addr", "127.0.0.1:8080", "listen address")
		fs.Parse(os.Args[2:])
		if err := runServe(*dbPath, *addr); err != nil {
			log.Fatal(err)
		}
	case "feed":
		if len(os.Args) < 3 {
			usage()
			os.Exit(2)
		}
		fs := flag.NewFlagSet("feed", flag.ExitOnError)
		dbPath := fs.String("db", "feedborne.db", "SQLite database path")
		switch os.Args[2] {
		case "add":
			fs.Parse(os.Args[3:])
			if fs.NArg() != 1 {
				log.Fatal("usage: feedborne feed add [-db path] <feed-url>")
			}
			if err := runFeedAdd(*dbPath, fs.Arg(0)); err != nil {
				log.Fatal(err)
			}
		case "list":
			fs.Parse(os.Args[3:])
			if err := runFeedList(*dbPath); err != nil {
				log.Fatal(err)
			}
		default:
			usage()
			os.Exit(2)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `feedborne - a small RSS reader

usage:
  feedborne crawl [-db feedborne.db]          fetch all feeds into SQLite
  feedborne serve [-db feedborne.db] [-addr 127.0.0.1:8080]
  feedborne feed add [-db feedborne.db] <url> register a feed
  feedborne feed list [-db feedborne.db]      list registered feeds

web ui keys: j/k move selection, o open in new tab, s read later, l switch view, b bookmark to Notion

optional: set FEEDBORNE_NOTION_TOKEN and FEEDBORNE_NOTION_DATA_SOURCE_ID
to enable the "bookmark to Notion" button (b key)`)
}
