"""Tests for RSS/Atom feed parsing."""

from datetime import datetime
from unittest.mock import Mock, patch

import pytest

from blogwatcher.rss import (
    FeedArticle,
    FeedParseError,
    _is_valid_feed,
    _parse_entry_date,
    discover_feed_url,
    parse_feed,
)


# Sample RSS feed content for testing
SAMPLE_RSS_FEED = """<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Blog</title>
    <link>https://example.com</link>
    <description>A test blog</description>
    <item>
      <title>First Post</title>
      <link>https://example.com/post1</link>
      <pubDate>Mon, 01 Jan 2024 12:00:00 GMT</pubDate>
    </item>
    <item>
      <title>Second Post</title>
      <link>https://example.com/post2</link>
      <pubDate>Tue, 02 Jan 2024 12:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>
"""

SAMPLE_ATOM_FEED = """<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Test Blog</title>
  <link href="https://example.com"/>
  <entry>
    <title>Atom Post</title>
    <link href="https://example.com/atom-post"/>
    <updated>2024-01-15T10:00:00Z</updated>
  </entry>
</feed>
"""

SAMPLE_HTML_WITH_RSS_LINK = """<!DOCTYPE html>
<html>
<head>
  <title>Test Blog</title>
  <link rel="alternate" type="application/rss+xml" href="/feed.xml" title="RSS Feed">
</head>
<body>
  <h1>Welcome to Test Blog</h1>
</body>
</html>
"""

SAMPLE_HTML_WITH_ATOM_LINK = """<!DOCTYPE html>
<html>
<head>
  <title>Test Blog</title>
  <link rel="alternate" type="application/atom+xml" href="https://example.com/atom.xml" title="Atom Feed">
</head>
<body>
  <h1>Welcome to Test Blog</h1>
</body>
</html>
"""

SAMPLE_HTML_NO_FEED = """<!DOCTYPE html>
<html>
<head>
  <title>Test Blog</title>
</head>
<body>
  <h1>Welcome to Test Blog</h1>
</body>
</html>
"""


class TestFeedArticleDataclass:
    """Tests for the FeedArticle dataclass."""

    def test_create_with_all_fields(self):
        """Test creating FeedArticle with all fields."""
        article = FeedArticle(
            title="Test Title",
            url="https://example.com/post",
            published_date=datetime(2024, 1, 1, 12, 0, 0),
        )
        assert article.title == "Test Title"
        assert article.url == "https://example.com/post"
        assert article.published_date == datetime(2024, 1, 1, 12, 0, 0)

    def test_create_without_date(self):
        """Test creating FeedArticle without published_date."""
        article = FeedArticle(title="Test Title", url="https://example.com/post")
        assert article.title == "Test Title"
        assert article.url == "https://example.com/post"
        assert article.published_date is None


class TestParseFeed:
    """Tests for the parse_feed function."""

    @patch("blogwatcher.rss.requests.get")
    def test_parse_rss_feed(self, mock_get):
        """Test parsing a valid RSS feed."""
        mock_response = Mock()
        mock_response.content = SAMPLE_RSS_FEED.encode()
        mock_response.raise_for_status = Mock()
        mock_get.return_value = mock_response

        articles = parse_feed("https://example.com/feed.xml")

        assert len(articles) == 2
        assert articles[0].title == "First Post"
        assert articles[0].url == "https://example.com/post1"
        assert articles[1].title == "Second Post"
        assert articles[1].url == "https://example.com/post2"

    @patch("blogwatcher.rss.requests.get")
    def test_parse_atom_feed(self, mock_get):
        """Test parsing a valid Atom feed."""
        mock_response = Mock()
        mock_response.content = SAMPLE_ATOM_FEED.encode()
        mock_response.raise_for_status = Mock()
        mock_get.return_value = mock_response

        articles = parse_feed("https://example.com/atom.xml")

        assert len(articles) == 1
        assert articles[0].title == "Atom Post"
        assert articles[0].url == "https://example.com/atom-post"

    @patch("blogwatcher.rss.requests.get")
    def test_parse_feed_with_dates(self, mock_get):
        """Test that dates are correctly parsed from feed entries."""
        mock_response = Mock()
        mock_response.content = SAMPLE_RSS_FEED.encode()
        mock_response.raise_for_status = Mock()
        mock_get.return_value = mock_response

        articles = parse_feed("https://example.com/feed.xml")

        assert articles[0].published_date is not None
        assert articles[0].published_date.year == 2024
        assert articles[0].published_date.month == 1
        assert articles[0].published_date.day == 1

    @patch("blogwatcher.rss.requests.get")
    def test_parse_feed_network_error(self, mock_get):
        """Test that network errors raise FeedParseError."""
        import requests

        mock_get.side_effect = requests.RequestException("Connection failed")

        with pytest.raises(FeedParseError) as exc_info:
            parse_feed("https://example.com/feed.xml")

        assert "Failed to fetch feed" in str(exc_info.value)

    @patch("blogwatcher.rss.requests.get")
    def test_parse_feed_invalid_content(self, mock_get):
        """Test that invalid feed content raises FeedParseError."""
        mock_response = Mock()
        mock_response.content = b"not a valid feed"
        mock_response.raise_for_status = Mock()
        mock_get.return_value = mock_response

        with pytest.raises(FeedParseError) as exc_info:
            parse_feed("https://example.com/feed.xml")

        assert "Failed to parse feed" in str(exc_info.value)

    @patch("blogwatcher.rss.requests.get")
    def test_parse_feed_skips_entries_without_title(self, mock_get):
        """Test that entries without title are skipped."""
        feed_content = """<?xml version="1.0"?>
        <rss version="2.0">
          <channel>
            <item>
              <link>https://example.com/no-title</link>
            </item>
            <item>
              <title>Has Title</title>
              <link>https://example.com/has-title</link>
            </item>
          </channel>
        </rss>
        """
        mock_response = Mock()
        mock_response.content = feed_content.encode()
        mock_response.raise_for_status = Mock()
        mock_get.return_value = mock_response

        articles = parse_feed("https://example.com/feed.xml")

        assert len(articles) == 1
        assert articles[0].title == "Has Title"

    @patch("blogwatcher.rss.requests.get")
    def test_parse_feed_skips_entries_without_link(self, mock_get):
        """Test that entries without link are skipped."""
        feed_content = """<?xml version="1.0"?>
        <rss version="2.0">
          <channel>
            <item>
              <title>No Link</title>
            </item>
            <item>
              <title>Has Link</title>
              <link>https://example.com/has-link</link>
            </item>
          </channel>
        </rss>
        """
        mock_response = Mock()
        mock_response.content = feed_content.encode()
        mock_response.raise_for_status = Mock()
        mock_get.return_value = mock_response

        articles = parse_feed("https://example.com/feed.xml")

        assert len(articles) == 1
        assert articles[0].title == "Has Link"


class TestDiscoverFeedUrl:
    """Tests for the discover_feed_url function."""

    @patch("blogwatcher.rss.requests.get")
    def test_discover_rss_link(self, mock_get):
        """Test discovering RSS feed from HTML page."""
        mock_response = Mock()
        mock_response.content = SAMPLE_HTML_WITH_RSS_LINK.encode()
        mock_response.status_code = 200
        mock_response.raise_for_status = Mock()
        mock_get.return_value = mock_response

        feed_url = discover_feed_url("https://example.com")

        assert feed_url == "https://example.com/feed.xml"

    @patch("blogwatcher.rss.requests.get")
    def test_discover_atom_link(self, mock_get):
        """Test discovering Atom feed from HTML page."""
        mock_response = Mock()
        mock_response.content = SAMPLE_HTML_WITH_ATOM_LINK.encode()
        mock_response.status_code = 200
        mock_response.raise_for_status = Mock()
        mock_get.return_value = mock_response

        feed_url = discover_feed_url("https://example.com")

        assert feed_url == "https://example.com/atom.xml"

    @patch("blogwatcher.rss._is_valid_feed")
    @patch("blogwatcher.rss.requests.get")
    def test_discover_fallback_to_common_paths(self, mock_get, mock_is_valid_feed):
        """Test fallback to common feed paths when no link tag found."""
        mock_response = Mock()
        mock_response.content = SAMPLE_HTML_NO_FEED.encode()
        mock_response.status_code = 200
        mock_response.raise_for_status = Mock()
        mock_get.return_value = mock_response

        # Make /feed return True as valid feed
        def is_valid_side_effect(url, timeout=10):
            return url == "https://example.com/feed"

        mock_is_valid_feed.side_effect = is_valid_side_effect

        feed_url = discover_feed_url("https://example.com")

        assert feed_url == "https://example.com/feed"

    @patch("blogwatcher.rss._is_valid_feed")
    @patch("blogwatcher.rss.requests.get")
    def test_discover_returns_none_when_no_feed(self, mock_get, mock_is_valid_feed):
        """Test that None is returned when no feed is found."""
        mock_response = Mock()
        mock_response.content = SAMPLE_HTML_NO_FEED.encode()
        mock_response.status_code = 200
        mock_response.raise_for_status = Mock()
        mock_get.return_value = mock_response

        mock_is_valid_feed.return_value = False

        feed_url = discover_feed_url("https://example.com")

        assert feed_url is None

    @patch("blogwatcher.rss.requests.get")
    def test_discover_handles_network_error(self, mock_get):
        """Test that network errors return None."""
        import requests

        mock_get.side_effect = requests.RequestException("Connection failed")

        feed_url = discover_feed_url("https://example.com")

        assert feed_url is None

    @patch("blogwatcher.rss.requests.get")
    def test_discover_handles_relative_urls(self, mock_get):
        """Test that relative feed URLs are converted to absolute."""
        html_with_relative = """<!DOCTYPE html>
        <html>
        <head>
          <link rel="alternate" type="application/rss+xml" href="/blog/feed.xml">
        </head>
        </html>
        """
        mock_response = Mock()
        mock_response.content = html_with_relative.encode()
        mock_response.status_code = 200
        mock_response.raise_for_status = Mock()
        mock_get.return_value = mock_response

        feed_url = discover_feed_url("https://example.com/blog/")

        assert feed_url == "https://example.com/blog/feed.xml"


class TestIsValidFeed:
    """Tests for the _is_valid_feed helper function."""

    @patch("blogwatcher.rss.requests.get")
    def test_valid_rss_feed(self, mock_get):
        """Test that valid RSS feed returns True."""
        mock_response = Mock()
        mock_response.content = SAMPLE_RSS_FEED.encode()
        mock_response.status_code = 200
        mock_get.return_value = mock_response

        assert _is_valid_feed("https://example.com/feed.xml") is True

    @patch("blogwatcher.rss.requests.get")
    def test_invalid_feed_returns_false(self, mock_get):
        """Test that invalid feed content returns False."""
        mock_response = Mock()
        mock_response.content = b"<html>Not a feed</html>"
        mock_response.status_code = 200
        mock_get.return_value = mock_response

        assert _is_valid_feed("https://example.com/not-feed") is False

    @patch("blogwatcher.rss.requests.get")
    def test_404_returns_false(self, mock_get):
        """Test that 404 response returns False."""
        mock_response = Mock()
        mock_response.status_code = 404
        mock_get.return_value = mock_response

        assert _is_valid_feed("https://example.com/missing") is False

    @patch("blogwatcher.rss.requests.get")
    def test_network_error_returns_false(self, mock_get):
        """Test that network errors return False."""
        import requests

        mock_get.side_effect = requests.RequestException("Connection failed")

        assert _is_valid_feed("https://example.com/feed") is False


class TestParseEntryDate:
    """Tests for the _parse_entry_date helper function."""

    def test_parse_published_parsed(self):
        """Test parsing date from published_parsed field."""
        entry = {"published_parsed": (2024, 1, 15, 10, 30, 0, 0, 0, 0)}
        result = _parse_entry_date(entry)

        assert result is not None
        assert result.year == 2024
        assert result.month == 1
        assert result.day == 15

    def test_parse_updated_parsed(self):
        """Test parsing date from updated_parsed field."""
        entry = {"updated_parsed": (2024, 2, 20, 14, 0, 0, 0, 0, 0)}
        result = _parse_entry_date(entry)

        assert result is not None
        assert result.year == 2024
        assert result.month == 2
        assert result.day == 20

    def test_prefers_published_over_updated(self):
        """Test that published_parsed is preferred over updated_parsed."""
        entry = {
            "published_parsed": (2024, 1, 1, 0, 0, 0, 0, 0, 0),
            "updated_parsed": (2024, 12, 31, 0, 0, 0, 0, 0, 0),
        }
        result = _parse_entry_date(entry)

        assert result is not None
        assert result.month == 1  # January from published, not December from updated

    def test_returns_none_for_no_date(self):
        """Test that None is returned when no date field exists."""
        entry = {"title": "No date entry"}
        result = _parse_entry_date(entry)

        assert result is None

    def test_handles_invalid_date(self):
        """Test that invalid date tuples are handled gracefully."""
        entry = {"published_parsed": None}
        result = _parse_entry_date(entry)

        assert result is None


class TestFeedParseError:
    """Tests for the FeedParseError exception."""

    def test_exception_message(self):
        """Test that exception stores message correctly."""
        error = FeedParseError("Test error message")
        assert str(error) == "Test error message"

    def test_exception_inheritance(self):
        """Test that FeedParseError inherits from Exception."""
        error = FeedParseError("Test")
        assert isinstance(error, Exception)
