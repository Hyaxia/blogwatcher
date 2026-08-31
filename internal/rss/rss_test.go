package rss

import (
	"net/http"
	"net/http/httptest"
	"strings"
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
<description>Default description.</description>
<keyword>default</keyword>
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

	articles, err := ParseFeed(server.URL, 2*time.Second, "")
	if err != nil {
		t.Fatalf("parse feed: %v", err)
	}
	if len(articles) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(articles))
	}
	if articles[0].PublishedDate == nil {
		t.Fatalf("expected published date")
	}
	if articles[0].Description != "" || articles[0].Keywords != "" {
		t.Fatalf("default parser did not apply metadata defaults: %+v", articles[0])
	}
}

func TestParseFeedMetadataOptionsAreIndependent(t *testing.T) {
	feed := `<?xml version="1.0"?><rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/"><channel>
<title>Metadata</title><item><title>First</title><link>https://example.com/1</link>
<description><![CDATA[<p>First &amp; useful paragraph.</p><p>Ignored.</p>]]></description>
<content:encoded><![CDATA[<p>Content fallback.</p>]]></content:encoded>
<keyword>alpha</keyword><keyword> beta </keyword><keyword> </keyword></item></channel></rss>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(feed))
	}))
	defer server.Close()

	tests := []struct {
		name        string
		options     ParseOptions
		description string
		keywords    string
	}{
		{name: "disabled", options: ParseOptions{}},
		{name: "description only", options: ParseOptions{StoreDescriptions: true, DescriptionMaxChars: 1000}, description: "First & useful paragraph."},
		{name: "keywords only", options: ParseOptions{StoreKeywords: true}, keywords: "alpha,beta"},
		{name: "combined", options: ParseOptions{StoreDescriptions: true, StoreKeywords: true, DescriptionMaxChars: 1000}, description: "First & useful paragraph.", keywords: "alpha,beta"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			articles, err := ParseFeedWithOptions(server.URL, 2*time.Second, "", test.options)
			if err != nil {
				t.Fatalf("parse feed: %v", err)
			}
			if len(articles) != 1 {
				t.Fatalf("expected one article, got %d", len(articles))
			}
			if articles[0].Description != test.description || articles[0].Keywords != test.keywords {
				t.Fatalf("unexpected metadata: %+v", articles[0])
			}
		})
	}
}

func TestParseFeedKeywordLimitKeepsArticles(t *testing.T) {
	largeDescription := strings.Repeat("x", int(maxBufferedFeedBytes)+1)
	feed := `<?xml version="1.0"?><rss version="2.0"><channel>` +
		`<title>Large feed</title><item><title>Kept</title>` +
		`<link>https://example.com/kept</link><keyword>omitted</keyword><description>` +
		largeDescription + `</description></item></channel></rss>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(feed))
	}))
	defer server.Close()

	articles, err := ParseFeedWithOptions(server.URL, 2*time.Second, "", ParseOptions{StoreKeywords: true})
	if err != nil {
		t.Fatalf("oversized feed should still parse: %v", err)
	}
	if len(articles) != 1 {
		t.Fatalf("expected one article, got %d", len(articles))
	}
	if articles[0].Title != "Kept" || articles[0].Keywords != "" {
		t.Fatalf("expected article without keywords, got %+v", articles[0])
	}
}

func TestDescriptionSelectionAndNormalization(t *testing.T) {
	tests := []struct {
		name        string
		description string
		content     string
		want        string
	}{
		{name: "description wins", description: `<p>Preferred&nbsp;paragraph.</p><p>Later.</p>`, content: `<p>Fallback.</p>`, want: "Preferred paragraph."},
		{name: "markup only falls back", description: `<p><script>ignore()</script> </p><style>p{}</style>`, content: `<p>Useful <strong>content</strong>.</p>`, want: "Useful content."},
		{name: "plain text block", description: " First line\ncontinues.\n\nSecond block.", want: "First line continues."},
		{name: "empty", description: "  ", content: `<script>ignore()</script>`, want: ""},
		{name: "product hunt shaped atom content", content: `<p>Firebase for Agents</p><p>Discussion | Link</p>`, want: "Firebase for Agents"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := selectDescription(test.description, test.content, 1000); got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
		})
	}
}

func TestDescriptionCharacterLimit(t *testing.T) {
	tests := []struct {
		name  string
		value string
		limit int
		want  string
	}{
		{name: "unlimited", value: "one two three", limit: 0, want: "one two three"},
		{name: "under limit", value: "short", limit: 5, want: "short"},
		{name: "word boundary", value: "one two three", limit: 9, want: "one two…"},
		{name: "single rune", value: "long", limit: 1, want: "…"},
		{name: "unicode safe", value: "café crème brûlée", limit: 11, want: "café crème…"},
		{name: "long word", value: "supercalifragilistic", limit: 6, want: "super…"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := limitAtWordBoundary(test.value, test.limit)
			if got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
			if test.limit > 0 && len([]rune(got)) > test.limit {
				t.Fatalf("result exceeds %d characters: %q", test.limit, got)
			}
		})
	}
}

func TestDescriptionCharacterLimitCJK(t *testing.T) {
	t.Run("all Chinese text falls back to rune boundary", func(t *testing.T) {
		value := strings.Repeat("汉", 1200)
		got := limitAtWordBoundary(value, 1000)
		want := strings.Repeat("汉", 999) + "…"
		if got != want {
			t.Fatalf("expected 999 Chinese runes plus ellipsis, got %d runes", len([]rune(got)))
		}
	})

	t.Run("nearby Chinese punctuation is included", func(t *testing.T) {
		got := limitAtWordBoundary("甲乙丙丁戊己，庚辛壬", 8)
		if got != "甲乙丙丁戊己，…" {
			t.Fatalf("expected punctuation boundary, got %q", got)
		}
	})

	t.Run("distant whitespace does not discard CJK text", func(t *testing.T) {
		value := "开头 " + strings.Repeat("中", 100)
		got := limitAtWordBoundary(value, 50)
		want := string([]rune(value)[:49]) + "…"
		if got != want {
			t.Fatalf("expected exact rune-boundary fallback, got %q", got)
		}
	})
}

func TestParseFeedKeywordAlignment(t *testing.T) {
	feed := `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom">
<title>Alignment</title>
<entry><title>Skipped</title><keyword>first</keyword></entry>
<entry><title>Kept</title><link href="https://example.com/kept"/><keyword>second</keyword></entry>
</feed>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(feed))
	}))
	defer server.Close()

	articles, err := ParseFeedWithOptions(server.URL, 2*time.Second, "", ParseOptions{StoreKeywords: true})
	if err != nil {
		t.Fatalf("parse feed: %v", err)
	}
	if len(articles) != 1 || articles[0].Keywords != "second" {
		t.Fatalf("keywords did not stay aligned: %+v", articles)
	}
}

func TestParseJSONFeedWithKeywordOption(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/feed+json")
		_, _ = w.Write([]byte(`{"version":"https://jsonfeed.org/version/1.1","title":"JSON","items":[{"id":"1","url":"https://example.com/1","title":"One"}]}`))
	}))
	defer server.Close()

	articles, err := ParseFeedWithOptions(server.URL, 2*time.Second, "", ParseOptions{StoreKeywords: true})
	if err != nil {
		t.Fatalf("parse JSON feed: %v", err)
	}
	if len(articles) != 1 || articles[0].Keywords != "" {
		t.Fatalf("unexpected JSON feed result: %+v", articles)
	}
}

func TestSupplementaryKeywordParserFailureIsIsolated(t *testing.T) {
	if _, err := parseKeywordSets([]byte(`<rss><channel><item><keyword>broken`)); err == nil {
		t.Fatal("expected malformed XML to fail supplementary parsing")
	}
}

func TestParseFeedRejectsNegativeDescriptionLimit(t *testing.T) {
	_, err := ParseFeedWithOptions("not a URL", time.Second, "", ParseOptions{StoreDescriptions: true, DescriptionMaxChars: -1})
	if err == nil || !strings.Contains(err.Error(), "zero or greater") {
		t.Fatalf("expected negative-limit error, got %v", err)
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

	feedURL, err := DiscoverFeedURL(server.URL, 2*time.Second, "")
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

	feedURL, err := DiscoverFeedURL(server.URL+"/tag/AI/feed/", 2*time.Second, "")
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

	feedURL, err := DiscoverFeedURL(server.URL, 2*time.Second, "")
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
