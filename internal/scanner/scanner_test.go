package scanner

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Hyaxia/blogwatcher/internal/model"
	"github.com/Hyaxia/blogwatcher/internal/storage"
)

const sampleFeed = `<?xml version="1.0" encoding="UTF-8" ?>
<rss version="2.0">
<channel>
<title>Example Feed</title>
<item>
<title>First</title>
<link>https://example.com/1</link>
</item>
<item>
<title>Second</title>
<link>https://example.com/2</link>
</item>
</channel>
</rss>`

const metadataFeed = `<?xml version="1.0" encoding="UTF-8" ?>
<rss version="2.0"><channel><title>Metadata Feed</title><item>
<title>Metadata article</title><link>https://example.com/metadata</link>
<description><![CDATA[<p>Stored description.</p>]]></description>
<keyword>go</keyword><keyword>rss</keyword>
</item></channel></rss>`

func TestScanBlogRSS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleFeed))
	}))
	defer server.Close()

	db := openTestDB(t)
	defer db.Close()

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com", FeedURL: server.URL})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	result := ScanBlog(db, blog)
	if result.NewArticles != 2 {
		t.Fatalf("expected 2 new articles, got %d", result.NewArticles)
	}
	if result.Source != "rss" {
		t.Fatalf("expected rss source, got %s", result.Source)
	}

	articles, err := db.ListArticles(false, nil)
	if err != nil {
		t.Fatalf("list articles: %v", err)
	}
	if len(articles) != 2 {
		t.Fatalf("expected 2 articles")
	}
}

func TestScanBlogStoresMetadataByDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(metadataFeed))
	}))
	defer server.Close()

	db := openTestDB(t)
	defer db.Close()
	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com", FeedURL: server.URL})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	result := ScanBlog(db, blog)
	if result.Error != "" || result.NewArticles != 1 {
		t.Fatalf("unexpected scan result: %+v", result)
	}
	articles, err := db.ListArticles(false, nil)
	if err != nil {
		t.Fatalf("list articles: %v", err)
	}
	if len(articles) != 1 || articles[0].Description != "Stored description." || articles[0].Keywords != "" {
		t.Fatalf("expected default description storage without keywords, got %+v", articles)
	}
}

func TestScanBlogMetadataOptionsAreIndependent(t *testing.T) {
	tests := []struct {
		name        string
		options     Options
		description string
		keywords    string
	}{
		{name: "disabled"},
		{name: "description only", options: Options{StoreDescriptions: true, DescriptionMaxChars: 1000}, description: "Stored description."},
		{name: "keywords only", options: Options{StoreKeywords: true}, keywords: "go,rss"},
		{name: "combined", options: Options{StoreDescriptions: true, StoreKeywords: true, DescriptionMaxChars: 1000}, description: "Stored description.", keywords: "go,rss"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(metadataFeed))
			}))
			defer server.Close()

			db := openTestDB(t)
			defer db.Close()
			blog, err := db.AddBlog(model.Blog{Name: test.name, URL: "https://example.com/" + test.name, FeedURL: server.URL})
			if err != nil {
				t.Fatalf("add blog: %v", err)
			}

			result := ScanBlogWithOptions(db, blog, test.options)
			if result.Error != "" || result.NewArticles != 1 {
				t.Fatalf("unexpected scan result: %+v", result)
			}
			articles, err := db.ListArticles(false, nil)
			if err != nil {
				t.Fatalf("list articles: %v", err)
			}
			if len(articles) != 1 || articles[0].Description != test.description || articles[0].Keywords != test.keywords {
				t.Fatalf("unexpected stored metadata: %+v", articles)
			}
		})
	}
}

func TestScanDoesNotBackfillKnownArticle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(metadataFeed))
	}))
	defer server.Close()

	db := openTestDB(t)
	defer db.Close()
	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com", FeedURL: server.URL})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}
	article, err := db.AddArticle(model.Article{BlogID: blog.ID, Title: "Metadata article", URL: "https://example.com/metadata"})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}

	result := ScanBlogWithOptions(db, blog, Options{StoreDescriptions: true, StoreKeywords: true, DescriptionMaxChars: 1000})
	if result.NewArticles != 0 {
		t.Fatalf("expected known article to be skipped: %+v", result)
	}
	stored, err := db.GetArticle(article.ID)
	if err != nil {
		t.Fatalf("get article: %v", err)
	}
	if stored.Description != "" || stored.Keywords != "" {
		t.Fatalf("known article was backfilled: %+v", stored)
	}
}

func TestScanBlogScraperFallback(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<body>
  <article><h2><a href="/one">First</a></h2></article>
</body>
</html>`

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	})
	mux.HandleFunc("/feed.xml", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	db := openTestDB(t)
	defer db.Close()

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: server.URL, FeedURL: server.URL + "/feed.xml", ScrapeSelector: "article h2 a"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	result := ScanBlog(db, blog)
	if result.Source != "scraper" {
		t.Fatalf("expected scraper source, got %s", result.Source)
	}
	if result.NewArticles != 1 {
		t.Fatalf("expected 1 new article, got %d", result.NewArticles)
	}
	if result.Error != "" {
		t.Fatalf("expected no error, got %s", result.Error)
	}
}

func TestScanAllBlogsConcurrent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleFeed))
	}))
	defer server.Close()

	db := openTestDB(t)
	defer db.Close()

	for i, name := range []string{"TestA", "TestB"} {
		_, err := db.AddBlog(model.Blog{Name: name, URL: "https://example.com/" + name, FeedURL: server.URL})
		if err != nil {
			t.Fatalf("add blog %d: %v", i, err)
		}
	}

	results, err := ScanAllBlogs(db, 2)
	if err != nil {
		t.Fatalf("scan all blogs: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func openTestDB(t *testing.T) *storage.Database {
	t.Helper()
	path := filepath.Join(t.TempDir(), "blogwatcher.db")
	db, err := storage.OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	return db
}

func TestScanBlogRespectsExistingArticles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleFeed))
	}))
	defer server.Close()

	db := openTestDB(t)
	defer db.Close()

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com", FeedURL: server.URL})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	_, err = db.AddArticle(model.Article{BlogID: blog.ID, Title: "First", URL: "https://example.com/1", DiscoveredDate: ptrTime(time.Now())})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}

	result := ScanBlog(db, blog)
	if result.NewArticles != 1 {
		t.Fatalf("expected 1 new article, got %d", result.NewArticles)
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
