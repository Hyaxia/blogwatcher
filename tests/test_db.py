"""Tests for database initialization and schema."""

import dataclasses
import tempfile
from pathlib import Path
from typing import get_type_hints

import pytest

from blogwatcher.db import Database
from blogwatcher.models import Article, Blog


@pytest.fixture
def db():
    """Create a temporary database for testing."""
    with tempfile.TemporaryDirectory() as tmpdir:
        db_path = Path(tmpdir) / "test.db"
        database = Database(db_path)
        yield database
        database.close()


class TestDatabaseInitialization:
    """Tests for database initialization."""

    def test_database_file_created(self, db: Database):
        """Test that database file is created on initialization."""
        assert db.db_path.exists()

    def test_tables_exist(self, db: Database):
        """Test that both blogs and articles tables are created."""
        conn = db._get_conn()
        cursor = conn.execute(
            "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name"
        )
        tables = [row[0] for row in cursor.fetchall()]
        assert "blogs" in tables
        assert "articles" in tables

    def test_blogs_table_columns(self, db: Database):
        """Test that blogs table has correct columns."""
        conn = db._get_conn()
        cursor = conn.execute("PRAGMA table_info(blogs)")
        columns = {row[1]: row[2] for row in cursor.fetchall()}

        expected_columns = {
            "id": "INTEGER",
            "name": "TEXT",
            "url": "TEXT",
            "feed_url": "TEXT",
            "scrape_selector": "TEXT",
            "last_scanned": "TIMESTAMP",
        }

        assert columns == expected_columns

    def test_articles_table_columns(self, db: Database):
        """Test that articles table has correct columns."""
        conn = db._get_conn()
        cursor = conn.execute("PRAGMA table_info(articles)")
        columns = {row[1]: row[2] for row in cursor.fetchall()}

        expected_columns = {
            "id": "INTEGER",
            "blog_id": "INTEGER",
            "title": "TEXT",
            "url": "TEXT",
            "published_date": "TIMESTAMP",
            "discovered_date": "TIMESTAMP",
            "is_read": "BOOLEAN",
        }

        assert columns == expected_columns


class TestSchemaMatchesModels:
    """Tests that database schema matches the model dataclasses."""

    def test_blogs_table_matches_blog_model(self, db: Database):
        """Test that blogs table columns match Blog model fields."""
        conn = db._get_conn()
        cursor = conn.execute("PRAGMA table_info(blogs)")
        db_columns = {row[1] for row in cursor.fetchall()}

        model_fields = {field.name for field in dataclasses.fields(Blog)}

        assert db_columns == model_fields, (
            f"Mismatch between blogs table and Blog model.\n"
            f"Table columns: {db_columns}\n"
            f"Model fields: {model_fields}"
        )

    def test_articles_table_matches_article_model(self, db: Database):
        """Test that articles table columns match Article model fields."""
        conn = db._get_conn()
        cursor = conn.execute("PRAGMA table_info(articles)")
        db_columns = {row[1] for row in cursor.fetchall()}

        model_fields = {field.name for field in dataclasses.fields(Article)}

        assert db_columns == model_fields, (
            f"Mismatch between articles table and Article model.\n"
            f"Table columns: {db_columns}\n"
            f"Model fields: {model_fields}"
        )


class TestTableConstraints:
    """Tests for table constraints."""

    def test_blogs_url_unique(self, db: Database):
        """Test that blogs.url has a unique constraint."""
        blog1 = Blog(id=None, name="Blog 1", url="https://example.com")
        blog2 = Blog(id=None, name="Blog 2", url="https://example.com")

        db.add_blog(blog1)
        with pytest.raises(Exception):  # sqlite3.IntegrityError
            db.add_blog(blog2)

    def test_articles_url_unique(self, db: Database):
        """Test that articles.url has a unique constraint."""
        blog = Blog(id=None, name="Test Blog", url="https://example.com")
        db.add_blog(blog)

        article1 = Article(
            id=None,
            blog_id=blog.id,
            title="Article 1",
            url="https://example.com/post1",
        )
        article2 = Article(
            id=None,
            blog_id=blog.id,
            title="Article 2",
            url="https://example.com/post1",
        )

        db.add_article(article1)
        with pytest.raises(Exception):  # sqlite3.IntegrityError
            db.add_article(article2)

    def test_blogs_name_not_null(self, db: Database):
        """Test that blogs.name cannot be null."""
        conn = db._get_conn()
        cursor = conn.execute("PRAGMA table_info(blogs)")
        columns = {row[1]: row[3] for row in cursor.fetchall()}  # notnull flag
        assert columns["name"] == 1  # 1 means NOT NULL

    def test_blogs_url_not_null(self, db: Database):
        """Test that blogs.url cannot be null."""
        conn = db._get_conn()
        cursor = conn.execute("PRAGMA table_info(blogs)")
        columns = {row[1]: row[3] for row in cursor.fetchall()}
        assert columns["url"] == 1

    def test_articles_blog_id_not_null(self, db: Database):
        """Test that articles.blog_id cannot be null."""
        conn = db._get_conn()
        cursor = conn.execute("PRAGMA table_info(articles)")
        columns = {row[1]: row[3] for row in cursor.fetchall()}
        assert columns["blog_id"] == 1

    def test_articles_title_not_null(self, db: Database):
        """Test that articles.title cannot be null."""
        conn = db._get_conn()
        cursor = conn.execute("PRAGMA table_info(articles)")
        columns = {row[1]: row[3] for row in cursor.fetchall()}
        assert columns["title"] == 1

    def test_articles_url_not_null(self, db: Database):
        """Test that articles.url cannot be null."""
        conn = db._get_conn()
        cursor = conn.execute("PRAGMA table_info(articles)")
        columns = {row[1]: row[3] for row in cursor.fetchall()}
        assert columns["url"] == 1

    def test_articles_foreign_key_exists(self, db: Database):
        """Test that articles.blog_id has a foreign key to blogs."""
        conn = db._get_conn()
        cursor = conn.execute("PRAGMA foreign_key_list(articles)")
        foreign_keys = cursor.fetchall()

        assert len(foreign_keys) == 1
        fk = foreign_keys[0]
        assert fk[2] == "blogs"  # referenced table
        assert fk[3] == "blog_id"  # from column
        assert fk[4] == "id"  # to column


class TestDefaultValues:
    """Tests for default column values."""

    def test_articles_is_read_defaults_to_false(self, db: Database):
        """Test that articles.is_read defaults to FALSE."""
        conn = db._get_conn()
        cursor = conn.execute("PRAGMA table_info(articles)")
        columns = {row[1]: row[4] for row in cursor.fetchall()}  # default value
        assert columns["is_read"] == "FALSE"

    def test_articles_discovered_date_has_default(self, db: Database):
        """Test that articles.discovered_date has a default value."""
        conn = db._get_conn()
        cursor = conn.execute("PRAGMA table_info(articles)")
        columns = {row[1]: row[4] for row in cursor.fetchall()}
        assert columns["discovered_date"] == "CURRENT_TIMESTAMP"


class TestPrimaryKeys:
    """Tests for primary keys."""

    def test_blogs_id_is_primary_key(self, db: Database):
        """Test that blogs.id is the primary key."""
        conn = db._get_conn()
        cursor = conn.execute("PRAGMA table_info(blogs)")
        columns = {row[1]: row[5] for row in cursor.fetchall()}  # pk flag
        assert columns["id"] == 1  # 1 means it's the primary key

    def test_articles_id_is_primary_key(self, db: Database):
        """Test that articles.id is the primary key."""
        conn = db._get_conn()
        cursor = conn.execute("PRAGMA table_info(articles)")
        columns = {row[1]: row[5] for row in cursor.fetchall()}
        assert columns["id"] == 1
