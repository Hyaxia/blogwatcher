package rss

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const sampleFeed = `<?xml version="1.0" encoding="UTF-8" ?>
<rss version="2.0">
<channel>
<title>Example Feed</title>
<item>
<title>First</title>
<link>https://example.com/1</link>
<pubDate>Mon, 02 Jan 2006 15:04:05 GMT</pubDate>
</item>
<item>
<title>Second</title>
<link>https://example.com/2</link>
</item>
</channel>
</rss>`

func TestParseFeed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleFeed))
	}))
	defer server.Close()

	articles, err := ParseFeed(server.URL, 2*time.Second)
	if err != nil {
		t.Fatalf("parse feed: %v", err)
	}
	if len(articles) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(articles))
	}
	if articles[0].PublishedDate == nil {
		t.Fatalf("expected published date")
	}
}

func TestDiscoverFeedURL(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html><head><link rel="alternate" type="application/rss+xml" href="/feed.xml" /></head></html>`))
	})
	mux.HandleFunc("/feed.xml", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleFeed))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	feedURL, err := DiscoverFeedURL(server.URL, 2*time.Second)
	if err != nil {
		t.Fatalf("discover feed: %v", err)
	}
	if feedURL == "" {
		t.Fatalf("expected feed url")
	}
}

func TestDiscoverFeedURL_XMLContentType(t *testing.T) {
	// Test that DiscoverFeedURL returns the URL directly when it returns XML content-type
	// (e.g. TechCrunch tag feeds)
	mux := http.NewServeMux()
	mux.HandleFunc("/tag/AI/feed/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml; charset=UTF-8")
		_, _ = w.Write([]byte(sampleFeed))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	feedURL, err := DiscoverFeedURL(server.URL+"/tag/AI/feed/", 2*time.Second)
	if err != nil {
		t.Fatalf("discover feed: %v", err)
	}
	if feedURL != server.URL+"/tag/AI/feed/" {
		t.Fatalf("expected url to be returned directly for XML content-type, got %s", feedURL)
	}
}

func TestDiscoverFeedURL_RelSelf(t *testing.T) {
	// Test that DiscoverFeedURL also checks rel="self" links
	// (some feeds like TechCrunch tag feeds use rel="self" instead of rel="alternate")
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html><head><link rel="self" type="application/rss+xml" href="/my-feed.xml" /></head></html>`))
	})
	mux.HandleFunc("/my-feed.xml", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleFeed))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	feedURL, err := DiscoverFeedURL(server.URL, 2*time.Second)
	if err != nil {
		t.Fatalf("discover feed: %v", err)
	}
	if feedURL == "" {
		t.Fatalf("expected feed url from rel=self link")
	}
	if feedURL != server.URL+"/my-feed.xml" {
		t.Fatalf("expected feed url to be %s, got %s", server.URL+"/my-feed.xml", feedURL)
	}
}
