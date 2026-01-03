# BlogWatcher

A Go CLI tool to track blog articles, detect new posts, and manage read/unread status. Supports both RSS/Atom feeds and HTML scraping as fallback.

## Features

-   **Dual Source Support** - Tries RSS feeds first, falls back to HTML scraping
-   **Automatic Feed Discovery** - Detects RSS/Atom URLs from blog homepages
-   **Read/Unread Management** - Track which articles you've read
-   **Blog Filtering** - View articles from specific blogs
-   **Duplicate Prevention** - Never tracks the same article twice
-   **Colored CLI Output** - User-friendly terminal interface

## Installation

```bash
# Homebrew (Linux)
brew install Hyaxia/tap/blogwatcher

# Install the CLI
go install github.com/Hyaxia/blogwatcher/cmd/blogwatcher@latest

# Or build locally
go build ./cmd/blogwatcher
```

Windows and Linux binaries are also available on the GitHub Releases page.

## Usage

### Adding Blogs

```bash
# Add a blog (auto-discovers RSS feed)
blogwatcher add "My Favorite Blog" https://example.com/blog

# Add with explicit feed URL
blogwatcher add "Tech Blog" https://techblog.com --feed-url https://techblog.com/rss.xml

# Add with HTML scraping selector (for blogs without feeds)
blogwatcher add "No-RSS Blog" https://norss.com --scrape-selector "article h2 a"
```

### Managing Blogs

```bash
# List all tracked blogs
blogwatcher blogs

# Remove a blog (and all its articles)
blogwatcher remove "My Favorite Blog"

# Remove without confirmation
blogwatcher remove "My Favorite Blog" -y
```

### Scanning for New Articles

```bash
# Scan all blogs for new articles
blogwatcher scan

# Scan a specific blog
blogwatcher scan "Tech Blog"
```

### Viewing Articles

```bash
# List unread articles
blogwatcher articles

# List all articles (including read)
blogwatcher articles --all

# List articles from a specific blog
blogwatcher articles --blog "Tech Blog"
```

### Managing Read Status

```bash
# Mark an article as read (use article ID from articles list)
blogwatcher read 42

# Mark an article as unread
blogwatcher unread 42
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

-   Go 1.22+

### Running Tests

```bash
# Run all tests
go test ./...
```

## License

MIT
