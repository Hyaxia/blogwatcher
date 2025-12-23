"""Tests for blog scanning logic."""

import tempfile
from datetime import datetime
from pathlib import Path
from unittest.mock import Mock, patch

import pytest

from blogwatcher.db import Database
from blogwatcher.models import Article, Blog
from blogwatcher.rss import FeedArticle, FeedParseError
from blogwatcher.scanner import (
    ScanResult,
    scan_all_blogs,
    scan_blog,
    scan_blog_by_name,
)
from blogwatcher.scraper import ScrapedArticle, ScrapeError


@pytest.fixture
def db():
    """Create a temporary database for testing."""
    with tempfile.TemporaryDirectory() as tmpdir:
        db_path = Path(tmpdir) / "test.db"
        database = Database(db_path)
        yield database
        database.close()


@pytest.fixture
def blog(db: Database) -> Blog:
    """Create a test blog in the database."""
    blog = Blog(
        id=None,
        name="Test Blog",
        url="https://example.com",
        feed_url="https://example.com/feed.xml",
    )
    return db.add_blog(blog)


@pytest.fixture
def blog_with_scraper(db: Database) -> Blog:
    """Create a test blog with scraper selector."""
    blog = Blog(
        id=None,
        name="Scrape Blog",
        url="https://example.com/blog",
        scrape_selector="article h2 a",
    )
    return db.add_blog(blog)


class TestScanResultDataclass:
    """Tests for the ScanResult dataclass."""

    def test_create_with_all_fields(self):
        """Test creating ScanResult with all fields."""
        result = ScanResult(
            blog_name="Test Blog",
            new_articles=5,
            total_found=10,
            source="rss",
            error=None,
        )
        assert result.blog_name == "Test Blog"
        assert result.new_articles == 5
        assert result.total_found == 10
        assert result.source == "rss"
        assert result.error is None

    def test_create_with_error(self):
        """Test creating ScanResult with error."""
        result = ScanResult(
            blog_name="Test Blog",
            new_articles=0,
            total_found=0,
            source="none",
            error="Connection failed",
        )
        assert result.error == "Connection failed"


class TestScanBlog:
    """Tests for the scan_blog function."""

    @patch("blogwatcher.scanner.parse_feed")
    def test_scan_blog_with_rss(self, mock_parse_feed, db: Database, blog: Blog):
        """Test scanning a blog with RSS feed."""
        mock_parse_feed.return_value = [
            FeedArticle(
                title="Post 1",
                url="https://example.com/post1",
                published_date=datetime(2024, 1, 1),
            ),
            FeedArticle(
                title="Post 2",
                url="https://example.com/post2",
                published_date=datetime(2024, 1, 2),
            ),
        ]

        result = scan_blog(db, blog)

        assert result.blog_name == "Test Blog"
        assert result.new_articles == 2
        assert result.total_found == 2
        assert result.source == "rss"
        assert result.error is None

        # Verify articles were inserted
        articles = db.list_articles()
        assert len(articles) == 2
        assert articles[0].title in ["Post 1", "Post 2"]

    @patch("blogwatcher.scanner.parse_feed")
    def test_scan_blog_deduplicates_articles(
        self, mock_parse_feed, db: Database, blog: Blog
    ):
        """Test that duplicate articles within same scan are deduplicated."""
        mock_parse_feed.return_value = [
            FeedArticle(title="Post 1", url="https://example.com/post1"),
            FeedArticle(title="Post 1 Duplicate", url="https://example.com/post1"),
            FeedArticle(title="Post 2", url="https://example.com/post2"),
        ]

        result = scan_blog(db, blog)

        assert result.new_articles == 2
        assert result.total_found == 2

    @patch("blogwatcher.scanner.parse_feed")
    def test_scan_blog_skips_existing_articles(
        self, mock_parse_feed, db: Database, blog: Blog
    ):
        """Test that existing articles are not re-inserted."""
        # Add an existing article
        existing = Article(
            id=None,
            blog_id=blog.id,
            title="Existing Post",
            url="https://example.com/existing",
        )
        db.add_article(existing)

        mock_parse_feed.return_value = [
            FeedArticle(title="Existing Post", url="https://example.com/existing"),
            FeedArticle(title="New Post", url="https://example.com/new"),
        ]

        result = scan_blog(db, blog)

        assert result.new_articles == 1
        assert result.total_found == 2

        articles = db.list_articles()
        assert len(articles) == 2

    @patch("blogwatcher.scanner.parse_feed")
    def test_scan_blog_updates_last_scanned(
        self, mock_parse_feed, db: Database, blog: Blog
    ):
        """Test that last_scanned timestamp is updated after scan."""
        mock_parse_feed.return_value = []
        assert blog.last_scanned is None

        scan_blog(db, blog)

        updated_blog = db.get_blog(blog.id)
        assert updated_blog.last_scanned is not None

    @patch("blogwatcher.scanner.scrape_blog")
    @patch("blogwatcher.scanner.parse_feed")
    def test_scan_blog_falls_back_to_scraper(
        self, mock_parse_feed, mock_scrape_blog, db: Database, blog_with_scraper: Blog
    ):
        """Test fallback to HTML scraping when RSS fails."""
        mock_parse_feed.side_effect = FeedParseError("Feed not found")
        mock_scrape_blog.return_value = [
            ScrapedArticle(title="Scraped Post", url="https://example.com/scraped"),
        ]

        result = scan_blog(db, blog_with_scraper)

        assert result.source == "scraper"
        assert result.new_articles == 1
        assert result.error is None

    @patch("blogwatcher.scanner.discover_feed_url")
    @patch("blogwatcher.scanner.parse_feed")
    def test_scan_blog_discovers_feed_url(
        self, mock_parse_feed, mock_discover_feed_url, db: Database
    ):
        """Test that feed URL is discovered and saved."""
        blog = Blog(id=None, name="No Feed Blog", url="https://example.com")
        db.add_blog(blog)

        mock_discover_feed_url.return_value = "https://example.com/discovered-feed.xml"
        mock_parse_feed.return_value = [
            FeedArticle(title="Post", url="https://example.com/post"),
        ]

        scan_blog(db, blog)

        updated_blog = db.get_blog(blog.id)
        assert updated_blog.feed_url == "https://example.com/discovered-feed.xml"

    @patch("blogwatcher.scanner.discover_feed_url")
    @patch("blogwatcher.scanner.scrape_blog")
    def test_scan_blog_no_feed_uses_scraper(
        self, mock_scrape_blog, mock_discover_feed_url, db: Database
    ):
        """Test that scraper is used when no feed is available."""
        blog = Blog(
            id=None,
            name="No Feed Blog",
            url="https://example.com",
            scrape_selector="article a",
        )
        db.add_blog(blog)

        mock_discover_feed_url.return_value = None
        mock_scrape_blog.return_value = [
            ScrapedArticle(title="Scraped", url="https://example.com/scraped"),
        ]

        result = scan_blog(db, blog)

        assert result.source == "scraper"
        assert result.new_articles == 1

    @patch("blogwatcher.scanner.scrape_blog")
    @patch("blogwatcher.scanner.parse_feed")
    def test_scan_blog_reports_both_errors(
        self, mock_parse_feed, mock_scrape_blog, db: Database, blog_with_scraper: Blog
    ):
        """Test that both RSS and scraper errors are reported."""
        mock_parse_feed.side_effect = FeedParseError("RSS failed")
        mock_scrape_blog.side_effect = ScrapeError("Scraper failed")

        result = scan_blog(db, blog_with_scraper)

        assert result.source == "none"
        assert result.new_articles == 0
        assert "Scraper" in result.error

    @patch("blogwatcher.scanner.parse_feed")
    def test_scan_blog_rss_error_only(self, mock_parse_feed, db: Database, blog: Blog):
        """Test error when only RSS fails and no scraper configured."""
        mock_parse_feed.side_effect = FeedParseError("RSS failed")

        result = scan_blog(db, blog)

        assert result.source == "none"
        assert result.error is not None
        assert "RSS failed" in result.error

    @patch("blogwatcher.scanner.parse_feed")
    def test_scan_blog_sets_article_metadata(
        self, mock_parse_feed, db: Database, blog: Blog
    ):
        """Test that article metadata is correctly set."""
        pub_date = datetime(2024, 1, 15, 10, 0, 0)
        mock_parse_feed.return_value = [
            FeedArticle(
                title="Post with Date",
                url="https://example.com/dated-post",
                published_date=pub_date,
            ),
        ]

        scan_blog(db, blog)

        articles = db.list_articles()
        assert len(articles) == 1
        assert articles[0].title == "Post with Date"
        assert articles[0].blog_id == blog.id
        assert articles[0].published_date == pub_date
        assert articles[0].is_read is False
        assert articles[0].discovered_date is not None


class TestScanAllBlogs:
    """Tests for the scan_all_blogs function."""

    @patch("blogwatcher.scanner.parse_feed")
    def test_scan_all_blogs_returns_results_for_each(
        self, mock_parse_feed, db: Database
    ):
        """Test that results are returned for each blog."""
        blog1 = Blog(
            id=None, name="Blog 1", url="https://blog1.com", feed_url="https://blog1.com/feed"
        )
        blog2 = Blog(
            id=None, name="Blog 2", url="https://blog2.com", feed_url="https://blog2.com/feed"
        )
        db.add_blog(blog1)
        db.add_blog(blog2)

        mock_parse_feed.return_value = [
            FeedArticle(title="Post", url="https://example.com/post"),
        ]

        results = scan_all_blogs(db)

        assert len(results) == 2
        assert results[0].blog_name == "Blog 1"
        assert results[1].blog_name == "Blog 2"

    def test_scan_all_blogs_empty_database(self, db: Database):
        """Test scanning when no blogs are tracked."""
        results = scan_all_blogs(db)
        assert results == []

    @patch("blogwatcher.scanner.parse_feed")
    def test_scan_all_blogs_continues_on_error(self, mock_parse_feed, db: Database):
        """Test that scanning continues even if one blog fails."""
        blog1 = Blog(
            id=None, name="Blog 1", url="https://blog1.com", feed_url="https://blog1.com/feed"
        )
        blog2 = Blog(
            id=None, name="Blog 2", url="https://blog2.com", feed_url="https://blog2.com/feed"
        )
        db.add_blog(blog1)
        db.add_blog(blog2)

        def side_effect(url, timeout=30):
            if "blog1" in url:
                raise FeedParseError("Blog 1 failed")
            return [FeedArticle(title="Post", url="https://blog2.com/post")]

        mock_parse_feed.side_effect = side_effect

        results = scan_all_blogs(db)

        assert len(results) == 2
        assert results[0].error is not None
        assert results[1].new_articles == 1


class TestScanBlogByName:
    """Tests for the scan_blog_by_name function."""

    @patch("blogwatcher.scanner.parse_feed")
    def test_scan_blog_by_name_found(self, mock_parse_feed, db: Database, blog: Blog):
        """Test scanning a blog by its name."""
        mock_parse_feed.return_value = [
            FeedArticle(title="Post", url="https://example.com/post"),
        ]

        result = scan_blog_by_name(db, "Test Blog")

        assert result is not None
        assert result.blog_name == "Test Blog"
        assert result.new_articles == 1

    def test_scan_blog_by_name_not_found(self, db: Database):
        """Test scanning a non-existent blog by name."""
        result = scan_blog_by_name(db, "Non-existent Blog")
        assert result is None


class TestArticleInsertion:
    """Tests for article insertion during scanning."""

    @patch("blogwatcher.scanner.parse_feed")
    def test_articles_marked_unread_by_default(
        self, mock_parse_feed, db: Database, blog: Blog
    ):
        """Test that new articles are marked as unread."""
        mock_parse_feed.return_value = [
            FeedArticle(title="New Post", url="https://example.com/new"),
        ]

        scan_blog(db, blog)

        articles = db.list_articles()
        assert len(articles) == 1
        assert articles[0].is_read is False

    @patch("blogwatcher.scanner.parse_feed")
    def test_articles_have_discovered_date(
        self, mock_parse_feed, db: Database, blog: Blog
    ):
        """Test that new articles have a discovered date set."""
        mock_parse_feed.return_value = [
            FeedArticle(title="New Post", url="https://example.com/new"),
        ]

        before_scan = datetime.now()
        scan_blog(db, blog)

        articles = db.list_articles()
        assert len(articles) == 1
        assert articles[0].discovered_date is not None
        assert articles[0].discovered_date >= before_scan

    @patch("blogwatcher.scanner.parse_feed")
    def test_articles_preserve_published_date(
        self, mock_parse_feed, db: Database, blog: Blog
    ):
        """Test that published date from feed is preserved."""
        pub_date = datetime(2024, 6, 15, 12, 0, 0)
        mock_parse_feed.return_value = [
            FeedArticle(
                title="Dated Post",
                url="https://example.com/dated",
                published_date=pub_date,
            ),
        ]

        scan_blog(db, blog)

        articles = db.list_articles()
        assert articles[0].published_date == pub_date

    @patch("blogwatcher.scanner.parse_feed")
    def test_articles_linked_to_correct_blog(
        self, mock_parse_feed, db: Database, blog: Blog
    ):
        """Test that articles are linked to the correct blog."""
        mock_parse_feed.return_value = [
            FeedArticle(title="Post", url="https://example.com/post"),
        ]

        scan_blog(db, blog)

        articles = db.list_articles()
        assert articles[0].blog_id == blog.id
