"""SQLite database operations for BlogWatcher."""

import sqlite3
from datetime import datetime
from pathlib import Path
from typing import Optional

from .models import Article, Blog

DEFAULT_DB_PATH = Path.home() / ".blogwatcher" / "blogwatcher.db"


class Database:
    """SQLite database interface for BlogWatcher."""

    def __init__(self, db_path: Optional[Path] = None):
        """Initialize database connection.

        Args:
            db_path: Path to the SQLite database file. Defaults to ~/.blogwatcher/blogwatcher.db
        """
        self.db_path = db_path or DEFAULT_DB_PATH
        self.db_path.parent.mkdir(parents=True, exist_ok=True)
        self._conn: Optional[sqlite3.Connection] = None
        self._init_db()

    def _get_conn(self) -> sqlite3.Connection:
        """Get or create database connection."""
        if self._conn is None:
            self._conn = sqlite3.connect(self.db_path)
            self._conn.row_factory = sqlite3.Row
        return self._conn

    def _init_db(self) -> None:
        """Create tables if they don't exist."""
        conn = self._get_conn()
        conn.executescript("""
            CREATE TABLE IF NOT EXISTS blogs (
                id INTEGER PRIMARY KEY,
                name TEXT NOT NULL,
                url TEXT NOT NULL UNIQUE,
                feed_url TEXT,
                scrape_selector TEXT,
                last_scanned TIMESTAMP
            );

            CREATE TABLE IF NOT EXISTS articles (
                id INTEGER PRIMARY KEY,
                blog_id INTEGER NOT NULL,
                title TEXT NOT NULL,
                url TEXT NOT NULL UNIQUE,
                published_date TIMESTAMP,
                discovered_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                is_read BOOLEAN DEFAULT FALSE,
                FOREIGN KEY (blog_id) REFERENCES blogs(id)
            );
        """)
        conn.commit()

    def close(self) -> None:
        """Close database connection."""
        if self._conn:
            self._conn.close()
            self._conn = None

    # Blog CRUD operations

    def add_blog(self, blog: Blog) -> Blog:
        """Add a new blog to track.

        Args:
            blog: Blog object to add (id will be ignored)

        Returns:
            Blog object with assigned id
        """
        conn = self._get_conn()
        cursor = conn.execute(
            """
            INSERT INTO blogs (name, url, feed_url, scrape_selector, last_scanned)
            VALUES (?, ?, ?, ?, ?)
            """,
            (blog.name, blog.url, blog.feed_url, blog.scrape_selector, blog.last_scanned),
        )
        conn.commit()
        blog.id = cursor.lastrowid
        return blog

    def get_blog(self, blog_id: int) -> Optional[Blog]:
        """Get a blog by id.

        Args:
            blog_id: The blog's id

        Returns:
            Blog object or None if not found
        """
        conn = self._get_conn()
        row = conn.execute("SELECT * FROM blogs WHERE id = ?", (blog_id,)).fetchone()
        return self._row_to_blog(row) if row else None

    def get_blog_by_name(self, name: str) -> Optional[Blog]:
        """Get a blog by name.

        Args:
            name: The blog's name

        Returns:
            Blog object or None if not found
        """
        conn = self._get_conn()
        row = conn.execute("SELECT * FROM blogs WHERE name = ?", (name,)).fetchone()
        return self._row_to_blog(row) if row else None

    def get_blog_by_url(self, url: str) -> Optional[Blog]:
        """Get a blog by URL.

        Args:
            url: The blog's URL

        Returns:
            Blog object or None if not found
        """
        conn = self._get_conn()
        row = conn.execute("SELECT * FROM blogs WHERE url = ?", (url,)).fetchone()
        return self._row_to_blog(row) if row else None

    def list_blogs(self) -> list[Blog]:
        """List all tracked blogs.

        Returns:
            List of Blog objects
        """
        conn = self._get_conn()
        rows = conn.execute("SELECT * FROM blogs ORDER BY name").fetchall()
        return [self._row_to_blog(row) for row in rows]

    def update_blog(self, blog: Blog) -> None:
        """Update an existing blog.

        Args:
            blog: Blog object with updated fields
        """
        conn = self._get_conn()
        conn.execute(
            """
            UPDATE blogs
            SET name = ?, url = ?, feed_url = ?, scrape_selector = ?, last_scanned = ?
            WHERE id = ?
            """,
            (blog.name, blog.url, blog.feed_url, blog.scrape_selector, blog.last_scanned, blog.id),
        )
        conn.commit()

    def update_blog_last_scanned(self, blog_id: int, last_scanned: datetime) -> None:
        """Update the last_scanned timestamp for a blog.

        Args:
            blog_id: The blog's id
            last_scanned: The timestamp to set
        """
        conn = self._get_conn()
        conn.execute(
            "UPDATE blogs SET last_scanned = ? WHERE id = ?",
            (last_scanned, blog_id),
        )
        conn.commit()

    def remove_blog(self, blog_id: int) -> bool:
        """Remove a blog and its articles.

        Args:
            blog_id: The blog's id

        Returns:
            True if blog was removed, False if not found
        """
        conn = self._get_conn()
        # Delete associated articles first
        conn.execute("DELETE FROM articles WHERE blog_id = ?", (blog_id,))
        cursor = conn.execute("DELETE FROM blogs WHERE id = ?", (blog_id,))
        conn.commit()
        return cursor.rowcount > 0

    def _row_to_blog(self, row: sqlite3.Row) -> Blog:
        """Convert a database row to a Blog object."""
        return Blog(
            id=row["id"],
            name=row["name"],
            url=row["url"],
            feed_url=row["feed_url"],
            scrape_selector=row["scrape_selector"],
            last_scanned=self._parse_datetime(row["last_scanned"]),
        )

    # Article CRUD operations

    def add_article(self, article: Article) -> Article:
        """Add a new article.

        Args:
            article: Article object to add (id will be ignored)

        Returns:
            Article object with assigned id
        """
        conn = self._get_conn()
        cursor = conn.execute(
            """
            INSERT INTO articles (blog_id, title, url, published_date, discovered_date, is_read)
            VALUES (?, ?, ?, ?, ?, ?)
            """,
            (
                article.blog_id,
                article.title,
                article.url,
                article.published_date,
                article.discovered_date or datetime.now(),
                article.is_read,
            ),
        )
        conn.commit()
        article.id = cursor.lastrowid
        return article

    def get_article(self, article_id: int) -> Optional[Article]:
        """Get an article by id.

        Args:
            article_id: The article's id

        Returns:
            Article object or None if not found
        """
        conn = self._get_conn()
        row = conn.execute("SELECT * FROM articles WHERE id = ?", (article_id,)).fetchone()
        return self._row_to_article(row) if row else None

    def get_article_by_url(self, url: str) -> Optional[Article]:
        """Get an article by URL.

        Args:
            url: The article's URL

        Returns:
            Article object or None if not found
        """
        conn = self._get_conn()
        row = conn.execute("SELECT * FROM articles WHERE url = ?", (url,)).fetchone()
        return self._row_to_article(row) if row else None

    def article_exists(self, url: str) -> bool:
        """Check if an article with the given URL already exists.

        Args:
            url: The article's URL

        Returns:
            True if article exists, False otherwise
        """
        conn = self._get_conn()
        row = conn.execute("SELECT 1 FROM articles WHERE url = ?", (url,)).fetchone()
        return row is not None

    def list_articles(self, unread_only: bool = False, blog_id: Optional[int] = None) -> list[Article]:
        """List articles with optional filters.

        Args:
            unread_only: If True, only return unread articles
            blog_id: If provided, only return articles from this blog

        Returns:
            List of Article objects
        """
        conn = self._get_conn()
        query = "SELECT * FROM articles WHERE 1=1"
        params: list = []

        if unread_only:
            query += " AND is_read = 0"
        if blog_id is not None:
            query += " AND blog_id = ?"
            params.append(blog_id)

        query += " ORDER BY discovered_date DESC"
        rows = conn.execute(query, params).fetchall()
        return [self._row_to_article(row) for row in rows]

    def mark_article_read(self, article_id: int) -> bool:
        """Mark an article as read.

        Args:
            article_id: The article's id

        Returns:
            True if article was updated, False if not found
        """
        conn = self._get_conn()
        cursor = conn.execute(
            "UPDATE articles SET is_read = 1 WHERE id = ?",
            (article_id,),
        )
        conn.commit()
        return cursor.rowcount > 0

    def mark_article_unread(self, article_id: int) -> bool:
        """Mark an article as unread.

        Args:
            article_id: The article's id

        Returns:
            True if article was updated, False if not found
        """
        conn = self._get_conn()
        cursor = conn.execute(
            "UPDATE articles SET is_read = 0 WHERE id = ?",
            (article_id,),
        )
        conn.commit()
        return cursor.rowcount > 0

    def get_articles_for_blog(self, blog_id: int) -> list[Article]:
        """Get all articles for a specific blog.

        Args:
            blog_id: The blog's id

        Returns:
            List of Article objects
        """
        return self.list_articles(blog_id=blog_id)

    def _row_to_article(self, row: sqlite3.Row) -> Article:
        """Convert a database row to an Article object."""
        return Article(
            id=row["id"],
            blog_id=row["blog_id"],
            title=row["title"],
            url=row["url"],
            published_date=self._parse_datetime(row["published_date"]),
            discovered_date=self._parse_datetime(row["discovered_date"]),
            is_read=bool(row["is_read"]),
        )

    @staticmethod
    def _parse_datetime(value: Optional[str]) -> Optional[datetime]:
        """Parse a datetime string from the database."""
        if value is None:
            return None
        try:
            return datetime.fromisoformat(value)
        except (ValueError, TypeError):
            return None
