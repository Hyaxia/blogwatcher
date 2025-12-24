(left this file so that it can use as a reference in the future for what Claude Code
has generated and later on acted on to create most of the project)

# BlogWatcher - Project Plan

## Overview

A Python CLI tool to track blog articles, detect new posts, and manage read/unread status.

## Tech Stack

-   **Language**: Python 3.11+
-   **Storage**: SQLite (via sqlite3 standard library)
-   **RSS Parsing**: feedparser
-   **HTML Scraping**: beautifulsoup4 + requests
-   **CLI Framework**: click

## Project Structure

```
blogwatcher/
├── blogwatcher/
│   ├── __init__.py
│   ├── cli.py           # CLI commands
│   ├── scanner.py       # Blog scanning logic
│   ├── rss.py           # RSS/Atom feed parsing
│   ├── scraper.py       # HTML scraping fallback
│   ├── db.py            # SQLite database operations
│   └── models.py        # Data models
├── blogs.yaml           # Blog configuration (user-maintained)
├── requirements.txt
└── README.md
```

## Database Schema

```sql
-- Blogs table
CREATE TABLE blogs (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    url TEXT NOT NULL UNIQUE,
    feed_url TEXT,          -- RSS/Atom feed URL if available
    scrape_selector TEXT,   -- CSS selector for scraping fallback
    last_scanned TIMESTAMP
);

-- Articles table
CREATE TABLE articles (
    id INTEGER PRIMARY KEY,
    blog_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    url TEXT NOT NULL UNIQUE,
    published_date TIMESTAMP,
    discovered_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_read BOOLEAN DEFAULT FALSE,
    FOREIGN KEY (blog_id) REFERENCES blogs(id)
);
```

## Blog Configuration Format (blogs.yaml)

```yaml
blogs:
    - name: 'Example Blog'
      url: 'https://example.com/blog'
      feed_url: 'https://example.com/feed.xml' # optional
      scrape_selector: 'article h2 a' # optional fallback
```

## CLI Commands

| Command                           | Description                     |
| --------------------------------- | ------------------------------- |
| `blogwatcher add <name> <url>`    | Add a new blog to track         |
| `blogwatcher remove <name>`       | Remove a blog                   |
| `blogwatcher blogs`               | List all tracked blogs          |
| `blogwatcher scan`                | Scan all blogs for new articles |
| `blogwatcher scan <blog-name>`    | Scan specific blog              |
| `blogwatcher articles`            | List all unread articles        |
| `blogwatcher articles --all`      | List all articles               |
| `blogwatcher read <article-id>`   | Mark article as read            |
| `blogwatcher unread <article-id>` | Mark article as unread          |

## Scanning Logic

1. For each blog:
    - Try RSS/Atom feed first (if feed_url configured or auto-discover)
    - Fall back to HTML scraping if RSS fails and scrape_selector is configured
2. Compare found articles against database
3. Insert new articles (unread by default)
4. Report summary of new articles found

## Implementation Checklist

### Step 1: Project Setup

-   [ ] Create project structure
-   [ ] Initialize `requirements.txt` with dependencies
-   [ ] Set up basic `__init__.py`

### Step 2: Database Layer (`db.py`)

-   [ ] Initialize SQLite database
-   [ ] Create tables if not exist
-   [ ] CRUD operations for blogs and articles

### Step 3: RSS Parser (`rss.py`)

-   [ ] Parse RSS/Atom feeds using feedparser
-   [ ] Auto-discover feed URLs from blog homepage
-   [ ] Return list of articles with title, url, date

### Step 4: HTML Scraper (`scraper.py`)

-   [ ] Fetch blog page with requests
-   [ ] Extract article links using BeautifulSoup + CSS selector
-   [ ] Return list of articles

### Step 5: Scanner (`scanner.py`)

-   [ ] Orchestrate RSS and scraping
-   [ ] Deduplicate articles
-   [ ] Insert new articles to database

### Step 6: CLI (`cli.py`)

-   [ ] Implement all commands using click
-   [ ] Pretty-print output with colors
-   [ ] Handle errors gracefully

### Step 7: Testing & Polish

-   [ ] Test with real blogs
-   [ ] Add error handling for network failures
-   [ ] Add helpful output messages

## Files to Create

1. `blogwatcher/__init__.py`
2. `blogwatcher/db.py`
3. `blogwatcher/rss.py`
4. `blogwatcher/scraper.py`
5. `blogwatcher/scanner.py`
6. `blogwatcher/cli.py`
7. `requirements.txt`
8. `blogs.yaml` (example config)
