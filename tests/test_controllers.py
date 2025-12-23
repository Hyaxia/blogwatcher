"""Tests for controllers."""

import tempfile
from pathlib import Path

import pytest

from blogwatcher.controllers import (
    ArticleNotFoundError,
    BlogAlreadyExistsError,
    BlogNotFoundError,
    add_blog,
    get_articles,
    mark_all_articles_read,
    mark_article_read,
    mark_article_unread,
    remove_blog,
)
from blogwatcher.db import Database
from blogwatcher.models import Article


@pytest.fixture
def db():
    """Create a temporary database for testing."""
    with tempfile.TemporaryDirectory() as tmpdir:
        db_path = Path(tmpdir) / "test.db"
        database = Database(db_path)
        yield database
        database.close()


class TestAddBlog:
    """Tests for add_blog controller."""

    def test_add_blog_success(self, db: Database):
        """Test adding a new blog."""
        blog = add_blog(db, "Test Blog", "https://example.com")

        assert blog.name == "Test Blog"
        assert blog.url == "https://example.com"
        assert blog.id is not None

    def test_add_blog_with_feed_url(self, db: Database):
        """Test adding a blog with feed URL."""
        blog = add_blog(
            db,
            "Test Blog",
            "https://example.com",
            feed_url="https://example.com/feed.xml",
        )

        assert blog.feed_url == "https://example.com/feed.xml"

    def test_add_blog_with_scrape_selector(self, db: Database):
        """Test adding a blog with scrape selector."""
        blog = add_blog(
            db,
            "Test Blog",
            "https://example.com",
            scrape_selector="article h2 a",
        )

        assert blog.scrape_selector == "article h2 a"

    def test_add_blog_duplicate_name_raises(self, db: Database):
        """Test that adding a blog with duplicate name raises error."""
        add_blog(db, "Test Blog", "https://example1.com")

        with pytest.raises(BlogAlreadyExistsError) as exc_info:
            add_blog(db, "Test Blog", "https://example2.com")

        assert exc_info.value.field == "name"
        assert exc_info.value.value == "Test Blog"

    def test_add_blog_duplicate_url_raises(self, db: Database):
        """Test that adding a blog with duplicate URL raises error."""
        add_blog(db, "Blog 1", "https://example.com")

        with pytest.raises(BlogAlreadyExistsError) as exc_info:
            add_blog(db, "Blog 2", "https://example.com")

        assert exc_info.value.field == "URL"
        assert exc_info.value.value == "https://example.com"


class TestRemoveBlog:
    """Tests for remove_blog controller."""

    def test_remove_blog_success(self, db: Database):
        """Test removing an existing blog."""
        add_blog(db, "Test Blog", "https://example.com")

        remove_blog(db, "Test Blog")

        assert db.get_blog_by_name("Test Blog") is None

    def test_remove_blog_not_found_raises(self, db: Database):
        """Test that removing non-existent blog raises error."""
        with pytest.raises(BlogNotFoundError) as exc_info:
            remove_blog(db, "Nonexistent")

        assert exc_info.value.name == "Nonexistent"


class TestGetArticles:
    """Tests for get_articles controller."""

    def test_get_articles_empty(self, db: Database):
        """Test getting articles when none exist."""
        articles, blog_names = get_articles(db)

        assert articles == []
        assert blog_names == {}

    def test_get_articles_unread_only(self, db: Database):
        """Test getting only unread articles."""
        blog = add_blog(db, "Test Blog", "https://example.com")
        article1 = Article(
            id=None, blog_id=blog.id, title="Unread", url="https://example.com/1"
        )
        article2 = Article(
            id=None, blog_id=blog.id, title="Read", url="https://example.com/2"
        )
        db.add_article(article1)
        db.add_article(article2)
        db.mark_article_read(article2.id)

        articles, blog_names = get_articles(db, show_all=False)

        assert len(articles) == 1
        assert articles[0].title == "Unread"
        assert blog_names[blog.id] == "Test Blog"

    def test_get_articles_show_all(self, db: Database):
        """Test getting all articles including read."""
        blog = add_blog(db, "Test Blog", "https://example.com")
        article1 = Article(
            id=None, blog_id=blog.id, title="Unread", url="https://example.com/1"
        )
        article2 = Article(
            id=None, blog_id=blog.id, title="Read", url="https://example.com/2"
        )
        db.add_article(article1)
        db.add_article(article2)
        db.mark_article_read(article2.id)

        articles, _ = get_articles(db, show_all=True)

        assert len(articles) == 2

    def test_get_articles_filter_by_blog(self, db: Database):
        """Test filtering articles by blog name."""
        blog1 = add_blog(db, "Blog 1", "https://example1.com")
        blog2 = add_blog(db, "Blog 2", "https://example2.com")
        db.add_article(
            Article(id=None, blog_id=blog1.id, title="A1", url="https://example1.com/1")
        )
        db.add_article(
            Article(id=None, blog_id=blog2.id, title="A2", url="https://example2.com/1")
        )

        articles, _ = get_articles(db, show_all=True, blog_name="Blog 1")

        assert len(articles) == 1
        assert articles[0].title == "A1"

    def test_get_articles_blog_not_found_raises(self, db: Database):
        """Test that filtering by non-existent blog raises error."""
        with pytest.raises(BlogNotFoundError) as exc_info:
            get_articles(db, blog_name="Nonexistent")

        assert exc_info.value.name == "Nonexistent"


class TestMarkArticleRead:
    """Tests for mark_article_read controller."""

    def test_mark_article_read_success(self, db: Database):
        """Test marking an article as read."""
        blog = add_blog(db, "Test Blog", "https://example.com")
        article = Article(
            id=None, blog_id=blog.id, title="Test", url="https://example.com/1"
        )
        db.add_article(article)

        result = mark_article_read(db, article.id)

        assert result.id == article.id
        updated = db.get_article(article.id)
        assert updated.is_read is True

    def test_mark_article_read_already_read(self, db: Database):
        """Test marking an already read article."""
        blog = add_blog(db, "Test Blog", "https://example.com")
        article = Article(
            id=None, blog_id=blog.id, title="Test", url="https://example.com/1"
        )
        db.add_article(article)
        db.mark_article_read(article.id)

        result = mark_article_read(db, article.id)

        assert result.is_read is True

    def test_mark_article_read_not_found_raises(self, db: Database):
        """Test that marking non-existent article raises error."""
        with pytest.raises(ArticleNotFoundError) as exc_info:
            mark_article_read(db, 999)

        assert exc_info.value.article_id == 999


class TestMarkAllArticlesRead:
    """Tests for mark_all_articles_read controller."""

    def test_mark_all_articles_read_success(self, db: Database):
        """Test marking all unread articles as read."""
        blog = add_blog(db, "Test Blog", "https://example.com")
        db.add_article(
            Article(id=None, blog_id=blog.id, title="A1", url="https://example.com/1")
        )
        db.add_article(
            Article(id=None, blog_id=blog.id, title="A2", url="https://example.com/2")
        )

        marked = mark_all_articles_read(db)

        assert len(marked) == 2
        articles, _ = get_articles(db, show_all=False)
        assert len(articles) == 0

    def test_mark_all_articles_read_by_blog(self, db: Database):
        """Test marking all unread articles for a specific blog."""
        blog1 = add_blog(db, "Blog 1", "https://example1.com")
        blog2 = add_blog(db, "Blog 2", "https://example2.com")
        db.add_article(
            Article(id=None, blog_id=blog1.id, title="A1", url="https://example1.com/1")
        )
        db.add_article(
            Article(id=None, blog_id=blog2.id, title="A2", url="https://example2.com/1")
        )

        marked = mark_all_articles_read(db, blog_name="Blog 1")

        assert len(marked) == 1
        articles, _ = get_articles(db, show_all=False)
        assert len(articles) == 1
        assert articles[0].blog_id == blog2.id

    def test_mark_all_articles_read_empty(self, db: Database):
        """Test marking all when no unread articles exist."""
        marked = mark_all_articles_read(db)

        assert marked == []

    def test_mark_all_articles_read_blog_not_found_raises(self, db: Database):
        """Test that filtering by non-existent blog raises error."""
        with pytest.raises(BlogNotFoundError) as exc_info:
            mark_all_articles_read(db, blog_name="Nonexistent")

        assert exc_info.value.name == "Nonexistent"


class TestMarkArticleUnread:
    """Tests for mark_article_unread controller."""

    def test_mark_article_unread_success(self, db: Database):
        """Test marking an article as unread."""
        blog = add_blog(db, "Test Blog", "https://example.com")
        article = Article(
            id=None, blog_id=blog.id, title="Test", url="https://example.com/1"
        )
        db.add_article(article)
        db.mark_article_read(article.id)

        result = mark_article_unread(db, article.id)

        assert result.id == article.id
        updated = db.get_article(article.id)
        assert updated.is_read is False

    def test_mark_article_unread_already_unread(self, db: Database):
        """Test marking an already unread article."""
        blog = add_blog(db, "Test Blog", "https://example.com")
        article = Article(
            id=None, blog_id=blog.id, title="Test", url="https://example.com/1"
        )
        db.add_article(article)

        result = mark_article_unread(db, article.id)

        assert result.is_read is False

    def test_mark_article_unread_not_found_raises(self, db: Database):
        """Test that marking non-existent article raises error."""
        with pytest.raises(ArticleNotFoundError) as exc_info:
            mark_article_unread(db, 999)

        assert exc_info.value.article_id == 999
