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
