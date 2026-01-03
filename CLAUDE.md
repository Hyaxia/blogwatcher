# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

BlogWatcher is a Go CLI tool to track blog articles, detect new posts, and manage read/unread status. It supports both RSS/Atom feeds and HTML scraping as fallback.

## Commands

```bash
# Run tests
go test ./...

# Running the project
go run ./cmd/blogwatcher ...
```

## Architecture

### Database
SQLite database stored at `~/.blogwatcher/blogwatcher.db` with two tables:
- `blogs` - Tracked blogs (name, url, feed_url, scrape_selector, last_scanned)
- `articles` - Discovered articles (blog_id, title, url, published_date, discovered_date, is_read)


## Tech Stack
- Go 1.24+
- SQLite (modernc.org/sqlite)
- gofeed (RSS/Atom)
- goquery + net/http (HTML scraping)
- cobra (CLI)
