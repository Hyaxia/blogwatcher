package scraper

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestScrapeBlog(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<body>
  <article><h2><a href="/one">First</a></h2></article>
  <article><h2><a href="/one">First Duplicate</a></h2></article>
  <div class="post"><h3><span><a href="/two" title="Second">Ignore Text</a></span></h3></div>
</body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	articles, err := ScrapeBlog(server.URL, "article h2 a, .post", 2*time.Second)
	if err != nil {
		t.Fatalf("scrape blog: %v", err)
	}
	if len(articles) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(articles))
	}
	if articles[0].URL == "" || articles[1].URL == "" {
		t.Fatalf("expected URLs")
	}
}
