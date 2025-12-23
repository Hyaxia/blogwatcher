# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

BlogWatcher is a Python CLI tool to track blog articles, detect new posts, and manage read/unread status. It supports both RSS/Atom feeds and HTML scraping as fallback.

## Commands

```bash
# if needed, use venv
source venv/bin/activate

# Install dependencies
pip install -r requirements.txt

# Run tests
pytest

# Run a single test
pytest tests/test_db.py::TestDatabaseInitialization::test_database_file_created

# Run tests with verbose output
pytest -v
```

## Architecture

### Database
SQLite database stored at `~/.blogwatcher/blogwatcher.db` with two tables:
- `blogs` - Tracked blogs (name, url, feed_url, scrape_selector, last_scanned)
- `articles` - Discovered articles (blog_id, title, url, published_date, discovered_date, is_read)


## Tech Stack
- Python 3.11+
- SQLite (standard library)
- feedparser (RSS/Atom)
- beautifulsoup4 + requests (HTML scraping)
- click (CLI)
- pytest (testing)
