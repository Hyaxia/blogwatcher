package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Hyaxia/blogwatcher/internal/model"
)

func TestDatabaseCreatesFileAndCRUD(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected db file to exist: %v", err)
	}

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}
	if blog.ID == 0 {
		t.Fatal("expected blog ID")
	}

	fetched, err := db.GetBlog(blog.ID)
	if err != nil {
		t.Fatalf("get blog: %v", err)
	}
	if fetched == nil || fetched.Name != "Test" {
		t.Fatalf("unexpected blog: %+v", fetched)
	}

	articles := []model.Article{
		{BlogID: blog.ID, Title: "One", URL: "https://example.com/1"},
		{BlogID: blog.ID, Title: "Two", URL: "https://example.com/2"},
	}
	count, err := db.AddArticlesBulk(articles)
	if err != nil {
		t.Fatalf("add articles bulk: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 articles, got %d", count)
	}

	list, err := db.ListArticles(false, nil)
	if err != nil {
		t.Fatalf("list articles: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(list))
	}

	ok, err := db.MarkArticleRead(list[0].ID)
	if err != nil || !ok {
		t.Fatalf("mark read: %v", err)
	}

	updated, err := db.GetArticle(list[0].ID)
	if err != nil {
		t.Fatalf("get article: %v", err)
	}
	if updated == nil || !updated.IsRead {
		t.Fatalf("expected article read: %+v", updated)
	}

	now := time.Now()
	if err := db.UpdateBlogLastScanned(blog.ID, now); err != nil {
		t.Fatalf("update last scanned: %v", err)
	}

	deleted, err := db.RemoveBlog(blog.ID)
	if err != nil {
		t.Fatalf("remove blog: %v", err)
	}
	if !deleted {
		t.Fatalf("expected blog removal")
	}
}

func TestGetExistingArticleURLs(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	_, err = db.AddArticle(model.Article{BlogID: blog.ID, Title: "One", URL: "https://example.com/1"})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}

	existing, err := db.GetExistingArticleURLs([]string{"https://example.com/1", "https://example.com/2"})
	if err != nil {
		t.Fatalf("get existing: %v", err)
	}
	if _, ok := existing["https://example.com/1"]; !ok {
		t.Fatalf("expected existing url")
	}
	if _, ok := existing["https://example.com/2"]; ok {
		t.Fatalf("did not expect url")
	}
}
