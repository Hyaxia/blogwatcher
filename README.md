# BlogWatcher

A Go CLI tool to track blog articles, detect new posts, and manage read/unread status. Supports both RSS/Atom feeds and HTML scraping as fallback.

## Features

-   **Dual Source Support** - Tries RSS feeds first, falls back to HTML scraping
-   **Automatic Feed Discovery** - Detects RSS/Atom URLs from blog homepages
-   **Read/Unread Management** - Track which articles you've read
-   **Blog Filtering** - View articles from specific blogs
-   **Duplicate Prevention** - Never tracks the same article twice
-   **Optional Feed Metadata** - Store and display descriptions or keywords only when explicitly enabled
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

# Add with per-blog User-Agent override
blogwatcher add "Blocked Blog" https://blocked.example --feed-url https://blocked.example/feed --user-agent "Mozilla/5.0 ..."
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

# Per-blog User-Agent is configured at add time via --user-agent
# Example above: blogwatcher add ... --user-agent "Mozilla/5.0 ..."

# Opt in to storing article descriptions (limited to 1,000 characters by default)
blogwatcher scan --store-descriptions

# Choose another character limit, or use 0 for no upper bound
blogwatcher scan --store-descriptions --description-max-chars 500

# Store feed keywords independently
blogwatcher scan --store-keywords

# The equivalent environment settings are useful for scheduled scans
BLOGWATCHER_STORE_DESCRIPTIONS=true BLOGWATCHER_DESCRIPTION_MAX_CHARS=500 blogwatcher scan
BLOGWATCHER_STORE_KEYWORDS=true blogwatcher scan
```

Description and keyword storage are disabled by default. These settings affect only newly discovered articles: existing rows are not backfilled, and disabling a setting does not remove values already stored. `BLOGWATCHER_STORE_DESCRIPTIONS` and `BLOGWATCHER_STORE_KEYWORDS` use Go boolean syntax; `true` and `false` are recommended, while standard forms such as `1`, `0`, `TRUE`, and `FALSE` are also accepted. Command-line flags override environment settings.

`BLOGWATCHER_DESCRIPTION_MAX_CHARS` defaults to `1000` after description storage is enabled. A value of `0` removes the upper bound; negative values are invalid. Supplying a description limit without enabling description storage is treated as incomplete configuration.

### Viewing Articles

```bash
# List unread articles
blogwatcher articles

# List all articles (including read)
blogwatcher articles --all

# List articles from a specific blog
blogwatcher articles --blog "Tech Blog"

# Opt in to displaying stored metadata (independently or together)
blogwatcher articles --show-descriptions
blogwatcher articles --show-keywords
blogwatcher articles --show-descriptions --show-keywords
```

Without either display flag, `blogwatcher articles` retains its original output format. Requested fields are omitted on rows where no value was stored.

### Managing Read Status

```bash
# Mark an article as read (use article ID from articles list)
blogwatcher read 42

# Mark an article as unread
blogwatcher unread 42

# Mark all unread articles as read
blogwatcher read-all

# Mark all unread articles as read for a blog (skip prompt)
blogwatcher read-all --blog "Tech Blog" --yes
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

By default, BlogWatcher stores data in SQLite at `~/.blogwatcher/blogwatcher.db`.

To isolate independent blog lists (for example, to avoid accidental `read-all` across unrelated sets), set `BLOGWATCHER_DB`:

```bash
BLOGWATCHER_DB="$HOME/.blogwatcher/work.db" blogwatcher scan
```

With `BLOGWATCHER_DB` unset or empty, BlogWatcher falls back to the default path.

Database tables:

-   **blogs** - Tracked blogs (name, URL, feed URL, scrape selector)
-   **articles** - Discovered articles (title, URL, dates, read status, and nullable description and keyword fields)

The nullable metadata columns are always added through an additive migration, but remain `NULL` for new articles unless their corresponding scan option is enabled. Older BlogWatcher binaries safely ignore these columns.

## Development

### Requirements

-   Go 1.24+

### Running Tests

```bash
# Run all tests
go test ./...
```

### Publishing

in addition to publishing to main a new tag should be published so homebrew will get the updated version:
```
  git tag vX.Y.Z
  git push origin vX.Y.Z
```

## License

MIT
