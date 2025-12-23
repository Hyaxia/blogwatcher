"""Business logic controllers for BlogWatcher."""

from typing import Optional

from .db import Database
from .models import Article, Blog


class BlogNotFoundError(Exception):
    """Raised when a blog is not found."""

    def __init__(self, name: str):
        self.name = name
        super().__init__(f"Blog '{name}' not found")


class BlogAlreadyExistsError(Exception):
    """Raised when trying to add a blog that already exists."""

    def __init__(self, field: str, value: str):
        self.field = field
        self.value = value
        super().__init__(f"Blog with {field} '{value}' already exists")


class ArticleNotFoundError(Exception):
    """Raised when an article is not found."""

    def __init__(self, article_id: int):
        self.article_id = article_id
        super().__init__(f"Article {article_id} not found")


def add_blog(
    db: Database,
    name: str,
    url: str,
    feed_url: Optional[str] = None,
    scrape_selector: Optional[str] = None,
) -> Blog:
    """Add a new blog to track.

    Args:
        db: Database instance
        name: Blog name
        url: Blog URL
        feed_url: Optional RSS/Atom feed URL
        scrape_selector: Optional CSS selector for HTML scraping

    Returns:
        The created Blog

    Raises:
        BlogAlreadyExistsError: If blog with same name or URL exists
    """
    if db.get_blog_by_name(name):
        raise BlogAlreadyExistsError("name", name)

    if db.get_blog_by_url(url):
        raise BlogAlreadyExistsError("URL", url)

    blog = Blog(
        id=None,
        name=name,
        url=url,
        feed_url=feed_url,
        scrape_selector=scrape_selector,
    )
    db.add_blog(blog)
    return blog


def remove_blog(db: Database, name: str) -> None:
    """Remove a blog from tracking.

    Args:
        db: Database instance
        name: Blog name to remove

    Raises:
        BlogNotFoundError: If blog not found
    """
    blog = db.get_blog_by_name(name)
    if not blog:
        raise BlogNotFoundError(name)

    db.remove_blog(blog.id)


def get_articles(
    db: Database,
    show_all: bool = False,
    blog_name: Optional[str] = None,
) -> tuple[list[Article], dict[int, str]]:
    """Get articles with optional filters.

    Args:
        db: Database instance
        show_all: If True, include read articles
        blog_name: Optional blog name to filter by

    Returns:
        Tuple of (articles list, blog_id -> blog_name mapping)

    Raises:
        BlogNotFoundError: If blog_name provided but not found
    """
    blog_id = None
    if blog_name:
        blog = db.get_blog_by_name(blog_name)
        if not blog:
            raise BlogNotFoundError(blog_name)
        blog_id = blog.id

    articles = db.list_articles(unread_only=not show_all, blog_id=blog_id)
    blog_names = {b.id: b.name for b in db.list_blogs()}

    return articles, blog_names


def mark_article_read(db: Database, article_id: int) -> Article:
    """Mark an article as read.

    Args:
        db: Database instance
        article_id: Article ID to mark

    Returns:
        The article (before marking)

    Raises:
        ArticleNotFoundError: If article not found
    """
    article = db.get_article(article_id)
    if not article:
        raise ArticleNotFoundError(article_id)

    if not article.is_read:
        db.mark_article_read(article_id)

    return article


def mark_all_articles_read(
    db: Database,
    blog_name: Optional[str] = None,
) -> list[Article]:
    """Mark all unread articles as read.

    Args:
        db: Database instance
        blog_name: Optional blog name to filter by

    Returns:
        List of articles that were marked as read

    Raises:
        BlogNotFoundError: If blog_name provided but not found
    """
    blog_id = None
    if blog_name:
        blog = db.get_blog_by_name(blog_name)
        if not blog:
            raise BlogNotFoundError(blog_name)
        blog_id = blog.id

    articles = db.list_articles(unread_only=True, blog_id=blog_id)

    for article in articles:
        db.mark_article_read(article.id)

    return articles


def mark_article_unread(db: Database, article_id: int) -> Article:
    """Mark an article as unread.

    Args:
        db: Database instance
        article_id: Article ID to mark

    Returns:
        The article (before marking)

    Raises:
        ArticleNotFoundError: If article not found
    """
    article = db.get_article(article_id)
    if not article:
        raise ArticleNotFoundError(article_id)

    if article.is_read:
        db.mark_article_unread(article_id)

    return article
