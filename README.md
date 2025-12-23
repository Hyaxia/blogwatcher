# BlogWatcher

A Python CLI tool to track blog articles, detect new posts, and manage read/unread status. Supports both RSS/Atom feeds and HTML scraping as fallback.

## Features

-   **Dual Source Support** - Tries RSS feeds first, falls back to HTML scraping
-   **Automatic Feed Discovery** - Detects RSS/Atom URLs from blog homepages
-   **Read/Unread Management** - Track which articles you've read
-   **Blog Filtering** - View articles from specific blogs
-   **Duplicate Prevention** - Never tracks the same article twice
-   **Colored CLI Output** - User-friendly terminal interface

## Installation

```bash
# Create virtual environment (optional but recommended)
python -m venv venv
source venv/bin/activate

# Install dependencies
pip install -r requirements.txt
```

## Usage

### Adding Blogs

```bash
# Add a blog (auto-discovers RSS feed)
python -m blogwatcher.cli add "My Favorite Blog" https://example.com/blog

# Add with explicit feed URL
python -m blogwatcher.cli add "Tech Blog" https://techblog.com --feed-url https://techblog.com/rss.xml

# Add with HTML scraping selector (for blogs without feeds)
python -m blogwatcher.cli add "No-RSS Blog" https://norss.com --scrape-selector "article h2 a"
```

### Managing Blogs

```bash
# List all tracked blogs
python -m blogwatcher.cli blogs

# Remove a blog (and all its articles)
python -m blogwatcher.cli remove "My Favorite Blog"

# Remove without confirmation
python -m blogwatcher.cli remove "My Favorite Blog" -y
```

### Scanning for New Articles

```bash
# Scan all blogs for new articles
python -m blogwatcher.cli scan

# Scan a specific blog
python -m blogwatcher.cli scan "Tech Blog"
```

### Viewing Articles

```bash
# List unread articles
python -m blogwatcher.cli articles

# List all articles (including read)
python -m blogwatcher.cli articles --all

# List articles from a specific blog
python -m blogwatcher.cli articles --blog "Tech Blog"
```

### Managing Read Status

```bash
# Mark an article as read (use article ID from articles list)
python -m blogwatcher.cli read 42

# Mark an article as unread
python -m blogwatcher.cli unread 42
```

## How It Works

### Scanning Process

1. For each tracked blog, BlogWatcher first attempts to parse the RSS/Atom feed
2. If no feed URL is configured, it tries to auto-discover one from the blog homepage
3. If RSS parsing fails and a `scrape_selector` is configured, it falls back to HTML scraping
4. New articles are saved to the database as unread
5. Already-tracked articles are skipped

### Feed Auto-Discovery

BlogWatcher searches for feeds in two ways:

-   Looking for `<link rel="alternate">` tags with RSS/Atom types
-   Checking common feed paths: `/feed`, `/rss`, `/feed.xml`, `/atom.xml`, etc.

### HTML Scraping

When RSS isn't available, provide a CSS selector that matches article links:

```bash
# Example selectors
--scrape-selector "article h2 a"      # Links inside article h2 tags
--scrape-selector ".post-title a"     # Links with post-title class
--scrape-selector "#blog-posts a"     # Links inside blog-posts ID
```

## Database

BlogWatcher stores data in SQLite at `~/.blogwatcher/blogwatcher.db`:

-   **blogs** - Tracked blogs (name, URL, feed URL, scrape selector)
-   **articles** - Discovered articles (title, URL, dates, read status)

## Development

### Requirements

-   Python 3.9+
-   Dependencies: click, feedparser, beautifulsoup4, requests, pyyaml

### Running Tests

```bash
# Run all tests
pytest

# Run with verbose output
pytest -v

# Run a specific test
pytest tests/test_db.py::TestDatabaseInitialization::test_database_file_created
```

## License

MIT
