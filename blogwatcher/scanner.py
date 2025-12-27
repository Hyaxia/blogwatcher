"""Blog scanning logic for BlogWatcher."""

from dataclasses import dataclass
from datetime import datetime
from typing import Optional

from .db import Database
from .models import Article, Blog
from .rss import FeedArticle, FeedParseError, discover_feed_url, parse_feed
from .scraper import ScrapedArticle, ScrapeError, scrape_blog


@dataclass
class ScanResult:
    """Result of scanning a single blog."""

    blog_name: str
    new_articles: int
    total_found: int
    source: str  # "rss", "scraper", or "none"
    error: Optional[str] = None


def scan_blog(db: Database, blog: Blog) -> ScanResult:
    """Scan a single blog for new articles.

    Tries RSS/Atom feed first, then falls back to HTML scraping if configured.

    Args:
        db: Database instance
        blog: Blog to scan

    Returns:
        ScanResult with summary of what was found
    """
    articles: list[FeedArticle | ScrapedArticle] = []
    source = "none"
    error = None

    # Try RSS/Atom feed first
    feed_url = blog.feed_url
    if not feed_url:
        feed_url = discover_feed_url(blog.url)
        if feed_url:
            # Save discovered feed URL for future scans
            blog.feed_url = feed_url
            db.update_blog(blog)

    if feed_url:
        try:
            articles = parse_feed(feed_url)
            source = "rss"
        except FeedParseError as e:
            error = str(e)

    # Fall back to HTML scraping if RSS failed and selector is configured
    if not articles and blog.scrape_selector:
        try:
            articles = scrape_blog(blog.url, blog.scrape_selector)
            source = "scraper"
            error = None  # Clear RSS error if scraping succeeded
        except ScrapeError as e:
            if error:
                error = f"RSS: {error}; Scraper: {e}"
            else:
                error = str(e)

    # Deduplicate and insert new articles
    seen_urls: set[str] = set()
    unique_articles: list[FeedArticle | ScrapedArticle] = []

    for article in articles:
        # Skip duplicates within the same scan
        if article.url in seen_urls:
            continue
        seen_urls.add(article.url)
        unique_articles.append(article)

    existing_urls = db.get_existing_article_urls(seen_urls)
    discovered_at = datetime.now()
    new_articles = []

    for article in unique_articles:
        # Skip if already in database
        if article.url in existing_urls:
            continue

        # Insert new article
        new_articles.append(
            Article(
                id=None,
                blog_id=blog.id,
                title=article.title,
                url=article.url,
                published_date=article.published_date,
                discovered_date=discovered_at,
                is_read=False,
            )
        )

    new_count = db.add_articles_bulk(new_articles)

    # Update last_scanned timestamp
    db.update_blog_last_scanned(blog.id, datetime.now())

    return ScanResult(
        blog_name=blog.name,
        new_articles=new_count,
        total_found=len(seen_urls),
        source=source,
        error=error,
    )


def scan_all_blogs(db: Database) -> list[ScanResult]:
    """Scan all tracked blogs for new articles.

    Args:
        db: Database instance

    Returns:
        List of ScanResult for each blog
    """
    blogs = db.list_blogs()
    results = []

    for blog in blogs:
        result = scan_blog(db, blog)
        results.append(result)

    return results


def scan_blog_by_name(db: Database, name: str) -> Optional[ScanResult]:
    """Scan a specific blog by name.

    Args:
        db: Database instance
        name: Blog name to scan

    Returns:
        ScanResult if blog found, None otherwise
    """
    blog = db.get_blog_by_name(name)
    if not blog:
        return None

    return scan_blog(db, blog)
