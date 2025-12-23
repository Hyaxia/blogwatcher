"""RSS/Atom feed parsing for BlogWatcher."""

from dataclasses import dataclass
from datetime import datetime
from time import mktime
from typing import Optional
from urllib.parse import urljoin

import feedparser
import requests
from bs4 import BeautifulSoup


@dataclass
class FeedArticle:
    """Represents an article parsed from an RSS/Atom feed."""

    title: str
    url: str
    published_date: Optional[datetime] = None


def parse_feed(feed_url: str, timeout: int = 30) -> list[FeedArticle]:
    """Parse an RSS/Atom feed and return articles.

    Args:
        feed_url: URL of the RSS/Atom feed
        timeout: Request timeout in seconds

    Returns:
        List of FeedArticle objects parsed from the feed

    Raises:
        FeedParseError: If the feed cannot be fetched or parsed
    """
    try:
        response = requests.get(feed_url, timeout=timeout)
        response.raise_for_status()
    except requests.RequestException as e:
        raise FeedParseError(f"Failed to fetch feed: {e}") from e

    feed = feedparser.parse(response.content)

    if feed.bozo and not feed.entries:
        raise FeedParseError(f"Failed to parse feed: {feed.bozo_exception}")

    articles = []
    for entry in feed.entries:
        title = entry.get("title", "").strip()
        link = entry.get("link", "").strip()

        if not title or not link:
            continue

        published_date = _parse_entry_date(entry)

        articles.append(
            FeedArticle(
                title=title,
                url=link,
                published_date=published_date,
            )
        )

    return articles


def discover_feed_url(blog_url: str, timeout: int = 30) -> Optional[str]:
    """Auto-discover RSS/Atom feed URL from a blog homepage.

    Looks for <link> tags with rel="alternate" and type="application/rss+xml"
    or "application/atom+xml".

    Args:
        blog_url: URL of the blog homepage
        timeout: Request timeout in seconds

    Returns:
        Feed URL if discovered, None otherwise
    """
    try:
        response = requests.get(blog_url, timeout=timeout)
        response.raise_for_status()
    except requests.RequestException:
        return None

    soup = BeautifulSoup(response.content, "html.parser")

    # Look for RSS/Atom feed links in order of preference
    feed_types = [
        "application/rss+xml",
        "application/atom+xml",
        "application/feed+json",
        "application/xml",
        "text/xml",
    ]

    for feed_type in feed_types:
        link = soup.find("link", rel="alternate", type=feed_type)
        if link and link.get("href"):
            href = link["href"]
            # Handle relative URLs
            return urljoin(blog_url, href)

    # Try common feed URL patterns as fallback
    common_paths = [
        "/feed",
        "/feed/",
        "/rss",
        "/rss/",
        "/feed.xml",
        "/rss.xml",
        "/atom.xml",
        "/index.xml",
    ]

    for path in common_paths:
        feed_url = urljoin(blog_url, path)
        if _is_valid_feed(feed_url, timeout):
            return feed_url

    return None


def _is_valid_feed(url: str, timeout: int = 10) -> bool:
    """Check if a URL points to a valid RSS/Atom feed.

    Args:
        url: URL to check
        timeout: Request timeout in seconds

    Returns:
        True if the URL is a valid feed, False otherwise
    """
    try:
        response = requests.get(url, timeout=timeout)
        if response.status_code != 200:
            return False

        feed = feedparser.parse(response.content)
        # A valid feed should have entries or at least a title
        return bool(feed.entries or feed.feed.get("title"))
    except Exception:
        return False


def _parse_entry_date(entry: dict) -> Optional[datetime]:
    """Parse publication date from a feed entry.

    Tries various date fields that feedparser might populate.

    Args:
        entry: feedparser entry dict

    Returns:
        datetime if a date was found and parsed, None otherwise
    """
    # feedparser normalizes dates to *_parsed tuple
    date_fields = ["published_parsed", "updated_parsed", "created_parsed"]

    for field in date_fields:
        parsed_time = entry.get(field)
        if parsed_time:
            try:
                return datetime.fromtimestamp(mktime(parsed_time))
            except (ValueError, OverflowError, OSError):
                continue

    return None


class FeedParseError(Exception):
    """Raised when a feed cannot be fetched or parsed."""

    pass
