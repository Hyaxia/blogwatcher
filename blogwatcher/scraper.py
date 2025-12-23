"""HTML scraping for BlogWatcher."""

from dataclasses import dataclass
from datetime import datetime
from typing import Optional
from urllib.parse import urljoin

import requests
from bs4 import BeautifulSoup


@dataclass
class ScrapedArticle:
    """Represents an article scraped from HTML."""

    title: str
    url: str
    published_date: Optional[datetime] = None


def scrape_blog(
    blog_url: str, selector: str, timeout: int = 30
) -> list[ScrapedArticle]:
    """Scrape a blog page for article links using a CSS selector.

    Args:
        blog_url: URL of the blog page to scrape
        selector: CSS selector to find article links (should select <a> tags)
        timeout: Request timeout in seconds

    Returns:
        List of ScrapedArticle objects found on the page

    Raises:
        ScrapeError: If the page cannot be fetched or parsed
    """
    try:
        response = requests.get(blog_url, timeout=timeout)
        response.raise_for_status()
    except requests.RequestException as e:
        raise ScrapeError(f"Failed to fetch page: {e}") from e

    soup = BeautifulSoup(response.content, "html.parser")

    articles = []
    seen_urls = set()

    elements = soup.select(selector)

    for element in elements:
        # Find the anchor tag - either the element itself or a descendant
        if element.name == "a":
            link = element
        else:
            link = element.find("a")

        if not link:
            continue

        href = link.get("href", "").strip()
        if not href:
            continue

        # Convert relative URLs to absolute
        url = urljoin(blog_url, href)

        # Skip duplicates
        if url in seen_urls:
            continue
        seen_urls.add(url)

        # Extract title from link text or parent element
        title = _extract_title(link, element)
        if not title:
            continue

        articles.append(ScrapedArticle(title=title, url=url))

    return articles


def _extract_title(link: BeautifulSoup, parent: BeautifulSoup) -> Optional[str]:
    """Extract article title from link or parent element.

    Tries multiple strategies to find a meaningful title:
    1. Link text content
    2. Link title attribute
    3. Parent element text content (if different from link)

    Args:
        link: The anchor element
        parent: The parent element selected by CSS selector

    Returns:
        Title string if found, None otherwise
    """
    # Try link text first
    title = link.get_text(strip=True)
    if title:
        return title

    # Try title attribute
    title = link.get("title", "").strip()
    if title:
        return title

    # Try parent text if parent is different from link
    if parent != link:
        title = parent.get_text(strip=True)
        if title:
            return title

    return None


class ScrapeError(Exception):
    """Raised when a page cannot be fetched or scraped."""

    pass
