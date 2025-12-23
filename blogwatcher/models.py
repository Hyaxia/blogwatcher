"""Data models for BlogWatcher."""

from dataclasses import dataclass
from datetime import datetime
from typing import Optional


@dataclass
class Blog:
    """Represents a tracked blog."""

    id: Optional[int]
    name: str
    url: str
    feed_url: Optional[str] = None
    scrape_selector: Optional[str] = None
    last_scanned: Optional[datetime] = None


@dataclass
class Article:
    """Represents a blog article."""

    id: Optional[int]
    blog_id: int
    title: str
    url: str
    published_date: Optional[datetime] = None
    discovered_date: Optional[datetime] = None
    is_read: bool = False
