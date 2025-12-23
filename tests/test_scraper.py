"""Tests for HTML scraping."""

from datetime import datetime
from unittest.mock import Mock, patch

import pytest

from blogwatcher.scraper import (
    ScrapedArticle,
    ScrapeError,
    _extract_title,
    scrape_blog,
)


# Sample HTML content for testing
SAMPLE_BLOG_PAGE = """<!DOCTYPE html>
<html>
<head>
  <title>Test Blog</title>
</head>
<body>
  <div class="posts">
    <article>
      <h2><a href="/post1">First Post Title</a></h2>
    </article>
    <article>
      <h2><a href="/post2">Second Post Title</a></h2>
    </article>
    <article>
      <h2><a href="https://example.com/post3">Third Post Title</a></h2>
    </article>
  </div>
</body>
</html>
"""

SAMPLE_BLOG_PAGE_LIST_FORMAT = """<!DOCTYPE html>
<html>
<body>
  <ul class="article-list">
    <li><a href="/articles/one">Article One</a></li>
    <li><a href="/articles/two">Article Two</a></li>
    <li><a href="/articles/three">Article Three</a></li>
  </ul>
</body>
</html>
"""

SAMPLE_BLOG_PAGE_CARD_FORMAT = """<!DOCTYPE html>
<html>
<body>
  <div class="cards">
    <div class="card">
      <a href="/card1" title="Card Title One">
        <img src="image.jpg" alt="thumbnail">
      </a>
    </div>
    <div class="card">
      <a href="/card2">Card Title Two</a>
    </div>
  </div>
</body>
</html>
"""

SAMPLE_BLOG_PAGE_WITH_DUPLICATES = """<!DOCTYPE html>
<html>
<body>
  <div class="posts">
    <a href="/post1">Post One</a>
    <a href="/post1">Post One Again</a>
    <a href="/post2">Post Two</a>
  </div>
</body>
</html>
"""

SAMPLE_BLOG_PAGE_NO_LINKS = """<!DOCTYPE html>
<html>
<body>
  <div class="posts">
    <article>
      <h2>Title Without Link</h2>
    </article>
  </div>
</body>
</html>
"""

SAMPLE_BLOG_PAGE_EMPTY_TITLES = """<!DOCTYPE html>
<html>
<body>
  <div class="posts">
    <a href="/post1"></a>
    <a href="/post2">   </a>
    <a href="/post3">Valid Title</a>
  </div>
</body>
</html>
"""


class TestScrapedArticleDataclass:
    """Tests for the ScrapedArticle dataclass."""

    def test_create_with_all_fields(self):
        """Test creating ScrapedArticle with all fields."""
        article = ScrapedArticle(
            title="Test Title",
            url="https://example.com/post",
            published_date=datetime(2024, 1, 1, 12, 0, 0),
        )
        assert article.title == "Test Title"
        assert article.url == "https://example.com/post"
        assert article.published_date == datetime(2024, 1, 1, 12, 0, 0)

    def test_create_without_date(self):
        """Test creating ScrapedArticle without published_date."""
        article = ScrapedArticle(title="Test Title", url="https://example.com/post")
        assert article.title == "Test Title"
        assert article.url == "https://example.com/post"
        assert article.published_date is None


class TestScrapeBlog:
    """Tests for the scrape_blog function."""

    @patch("blogwatcher.scraper.requests.get")
    def test_scrape_article_links(self, mock_get):
        """Test scraping article links from a blog page."""
        mock_response = Mock()
        mock_response.content = SAMPLE_BLOG_PAGE.encode()
        mock_response.raise_for_status = Mock()
        mock_get.return_value = mock_response

        articles = scrape_blog("https://example.com/blog", "article h2 a")

        assert len(articles) == 3
        assert articles[0].title == "First Post Title"
        assert articles[0].url == "https://example.com/post1"
        assert articles[1].title == "Second Post Title"
        assert articles[1].url == "https://example.com/post2"
        assert articles[2].title == "Third Post Title"
        assert articles[2].url == "https://example.com/post3"

    @patch("blogwatcher.scraper.requests.get")
    def test_scrape_list_format(self, mock_get):
        """Test scraping articles from a list format."""
        mock_response = Mock()
        mock_response.content = SAMPLE_BLOG_PAGE_LIST_FORMAT.encode()
        mock_response.raise_for_status = Mock()
        mock_get.return_value = mock_response

        articles = scrape_blog("https://example.com", ".article-list a")

        assert len(articles) == 3
        assert articles[0].title == "Article One"
        assert articles[0].url == "https://example.com/articles/one"

    @patch("blogwatcher.scraper.requests.get")
    def test_scrape_with_parent_selector(self, mock_get):
        """Test scraping when selector targets parent element containing link."""
        mock_response = Mock()
        mock_response.content = SAMPLE_BLOG_PAGE.encode()
        mock_response.raise_for_status = Mock()
        mock_get.return_value = mock_response

        articles = scrape_blog("https://example.com", "article h2")

        assert len(articles) == 3
        assert articles[0].title == "First Post Title"

    @patch("blogwatcher.scraper.requests.get")
    def test_scrape_deduplicates_urls(self, mock_get):
        """Test that duplicate URLs are filtered out."""
        mock_response = Mock()
        mock_response.content = SAMPLE_BLOG_PAGE_WITH_DUPLICATES.encode()
        mock_response.raise_for_status = Mock()
        mock_get.return_value = mock_response

        articles = scrape_blog("https://example.com", ".posts a")

        assert len(articles) == 2
        urls = [a.url for a in articles]
        assert "https://example.com/post1" in urls
        assert "https://example.com/post2" in urls

    @patch("blogwatcher.scraper.requests.get")
    def test_scrape_converts_relative_urls(self, mock_get):
        """Test that relative URLs are converted to absolute."""
        mock_response = Mock()
        mock_response.content = SAMPLE_BLOG_PAGE.encode()
        mock_response.raise_for_status = Mock()
        mock_get.return_value = mock_response

        articles = scrape_blog("https://example.com/blog/", "article h2 a")

        assert articles[0].url == "https://example.com/post1"
        assert articles[1].url == "https://example.com/post2"

    @patch("blogwatcher.scraper.requests.get")
    def test_scrape_extracts_title_from_attribute(self, mock_get):
        """Test extracting title from title attribute when text is empty."""
        mock_response = Mock()
        mock_response.content = SAMPLE_BLOG_PAGE_CARD_FORMAT.encode()
        mock_response.raise_for_status = Mock()
        mock_get.return_value = mock_response

        articles = scrape_blog("https://example.com", ".card a")

        assert len(articles) == 2
        assert articles[0].title == "Card Title One"
        assert articles[1].title == "Card Title Two"

    @patch("blogwatcher.scraper.requests.get")
    def test_scrape_skips_empty_titles(self, mock_get):
        """Test that articles with empty titles are skipped."""
        mock_response = Mock()
        mock_response.content = SAMPLE_BLOG_PAGE_EMPTY_TITLES.encode()
        mock_response.raise_for_status = Mock()
        mock_get.return_value = mock_response

        articles = scrape_blog("https://example.com", ".posts a")

        assert len(articles) == 1
        assert articles[0].title == "Valid Title"

    @patch("blogwatcher.scraper.requests.get")
    def test_scrape_returns_empty_when_no_links(self, mock_get):
        """Test that empty list is returned when no links found."""
        mock_response = Mock()
        mock_response.content = SAMPLE_BLOG_PAGE_NO_LINKS.encode()
        mock_response.raise_for_status = Mock()
        mock_get.return_value = mock_response

        articles = scrape_blog("https://example.com", "article h2 a")

        assert articles == []

    @patch("blogwatcher.scraper.requests.get")
    def test_scrape_returns_empty_for_no_matching_selector(self, mock_get):
        """Test that empty list is returned when selector matches nothing."""
        mock_response = Mock()
        mock_response.content = SAMPLE_BLOG_PAGE.encode()
        mock_response.raise_for_status = Mock()
        mock_get.return_value = mock_response

        articles = scrape_blog("https://example.com", ".nonexistent-class")

        assert articles == []

    @patch("blogwatcher.scraper.requests.get")
    def test_scrape_network_error(self, mock_get):
        """Test that network errors raise ScrapeError."""
        import requests

        mock_get.side_effect = requests.RequestException("Connection failed")

        with pytest.raises(ScrapeError) as exc_info:
            scrape_blog("https://example.com", "article a")

        assert "Failed to fetch page" in str(exc_info.value)

    @patch("blogwatcher.scraper.requests.get")
    def test_scrape_http_error(self, mock_get):
        """Test that HTTP errors raise ScrapeError."""
        import requests

        mock_response = Mock()
        mock_response.raise_for_status.side_effect = requests.HTTPError("404 Not Found")
        mock_get.return_value = mock_response

        with pytest.raises(ScrapeError) as exc_info:
            scrape_blog("https://example.com", "article a")

        assert "Failed to fetch page" in str(exc_info.value)

    @patch("blogwatcher.scraper.requests.get")
    def test_scrape_with_custom_timeout(self, mock_get):
        """Test that custom timeout is passed to requests."""
        mock_response = Mock()
        mock_response.content = SAMPLE_BLOG_PAGE.encode()
        mock_response.raise_for_status = Mock()
        mock_get.return_value = mock_response

        scrape_blog("https://example.com", "article a", timeout=60)

        mock_get.assert_called_once_with("https://example.com", timeout=60)


class TestExtractTitle:
    """Tests for the _extract_title helper function."""

    def test_extract_from_link_text(self):
        """Test extracting title from link text content."""
        from bs4 import BeautifulSoup

        html = '<a href="/post">Link Title</a>'
        soup = BeautifulSoup(html, "html.parser")
        link = soup.find("a")

        title = _extract_title(link, link)

        assert title == "Link Title"

    def test_extract_from_title_attribute(self):
        """Test extracting title from title attribute when text is empty."""
        from bs4 import BeautifulSoup

        html = '<a href="/post" title="Attribute Title"><img src="img.jpg"></a>'
        soup = BeautifulSoup(html, "html.parser")
        link = soup.find("a")

        title = _extract_title(link, link)

        assert title == "Attribute Title"

    def test_extract_from_parent_text(self):
        """Test extracting title from parent element text."""
        from bs4 import BeautifulSoup

        html = '<h2><a href="/post"><img src="img.jpg"></a> Parent Title</h2>'
        soup = BeautifulSoup(html, "html.parser")
        parent = soup.find("h2")
        link = soup.find("a")

        title = _extract_title(link, parent)

        assert title == "Parent Title"

    def test_returns_none_for_no_title(self):
        """Test that None is returned when no title can be found."""
        from bs4 import BeautifulSoup

        html = '<a href="/post"><img src="img.jpg"></a>'
        soup = BeautifulSoup(html, "html.parser")
        link = soup.find("a")

        title = _extract_title(link, link)

        assert title is None


class TestScrapeError:
    """Tests for the ScrapeError exception."""

    def test_exception_message(self):
        """Test that exception stores message correctly."""
        error = ScrapeError("Test error message")
        assert str(error) == "Test error message"

    def test_exception_inheritance(self):
        """Test that ScrapeError inherits from Exception."""
        error = ScrapeError("Test")
        assert isinstance(error, Exception)
