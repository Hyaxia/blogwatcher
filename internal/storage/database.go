package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/Hyaxia/blogwatcher/internal/model"
)

const sqliteTimeLayout = time.RFC3339Nano

func DefaultDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".blogwatcher", "blogwatcher.db"), nil
}

type Database struct {
	path string
	conn *sql.DB
}

func OpenDatabase(path string) (*Database, error) {
	if path == "" {
		var err error
		path, err = DefaultDBPath()
		if err != nil {
			return nil, err
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	dsn := fmt.Sprintf("file:%s?_busy_timeout=5000&_foreign_keys=1", path)
	conn, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	db := &Database{path: path, conn: conn}
	if err := db.init(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return db, nil
}

func (db *Database) Path() string {
	return db.path
}

func (db *Database) Close() error {
	if db.conn == nil {
		return nil
	}
	return db.conn.Close()
}

func (db *Database) init() error {
	schema := `
		CREATE TABLE IF NOT EXISTS blogs (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			url TEXT NOT NULL UNIQUE,
			feed_url TEXT,
			scrape_selector TEXT,
			last_scanned TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS articles (
			id INTEGER PRIMARY KEY,
			blog_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			url TEXT NOT NULL UNIQUE,
			published_date TIMESTAMP,
			discovered_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			is_read BOOLEAN DEFAULT FALSE,
			FOREIGN KEY (blog_id) REFERENCES blogs(id)
		);
	`
	_, err := db.conn.Exec(schema)
	return err
}

func (db *Database) AddBlog(blog model.Blog) (model.Blog, error) {
	result, err := db.conn.Exec(
		`INSERT INTO blogs (name, url, feed_url, scrape_selector, last_scanned)
		VALUES (?, ?, ?, ?, ?)`,
		blog.Name,
		blog.URL,
		nullIfEmpty(blog.FeedURL),
		nullIfEmpty(blog.ScrapeSelector),
		formatTimePtr(blog.LastScanned),
	)
	if err != nil {
		return blog, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return blog, err
	}
	blog.ID = id
	return blog, nil
}

func (db *Database) GetBlog(id int64) (*model.Blog, error) {
	row := db.conn.QueryRow(`SELECT id, name, url, feed_url, scrape_selector, last_scanned FROM blogs WHERE id = ?`, id)
	return scanBlog(row)
}

func (db *Database) GetBlogByName(name string) (*model.Blog, error) {
	row := db.conn.QueryRow(`SELECT id, name, url, feed_url, scrape_selector, last_scanned FROM blogs WHERE name = ?`, name)
	return scanBlog(row)
}

func (db *Database) GetBlogByURL(url string) (*model.Blog, error) {
	row := db.conn.QueryRow(`SELECT id, name, url, feed_url, scrape_selector, last_scanned FROM blogs WHERE url = ?`, url)
	return scanBlog(row)
}

func (db *Database) ListBlogs() ([]model.Blog, error) {
	rows, err := db.conn.Query(`SELECT id, name, url, feed_url, scrape_selector, last_scanned FROM blogs ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blogs []model.Blog
	for rows.Next() {
		blog, err := scanBlog(rows)
		if err != nil {
			return nil, err
		}
		if blog != nil {
			blogs = append(blogs, *blog)
		}
	}
	return blogs, rows.Err()
}

func (db *Database) UpdateBlog(blog model.Blog) error {
	_, err := db.conn.Exec(
		`UPDATE blogs SET name = ?, url = ?, feed_url = ?, scrape_selector = ?, last_scanned = ? WHERE id = ?`,
		blog.Name,
		blog.URL,
		nullIfEmpty(blog.FeedURL),
		nullIfEmpty(blog.ScrapeSelector),
		formatTimePtr(blog.LastScanned),
		blog.ID,
	)
	return err
}

func (db *Database) UpdateBlogLastScanned(id int64, lastScanned time.Time) error {
	_, err := db.conn.Exec(`UPDATE blogs SET last_scanned = ? WHERE id = ?`, lastScanned.Format(sqliteTimeLayout), id)
	return err
}

func (db *Database) RemoveBlog(id int64) (bool, error) {
	_, err := db.conn.Exec(`DELETE FROM articles WHERE blog_id = ?`, id)
	if err != nil {
		return false, err
	}
	result, err := db.conn.Exec(`DELETE FROM blogs WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (db *Database) AddArticle(article model.Article) (model.Article, error) {
	result, err := db.conn.Exec(
		`INSERT INTO articles (blog_id, title, url, published_date, discovered_date, is_read)
		VALUES (?, ?, ?, ?, ?, ?)`,
		article.BlogID,
		article.Title,
		article.URL,
		formatTimePtr(article.PublishedDate),
		formatTimePtr(article.DiscoveredDate),
		article.IsRead,
	)
	if err != nil {
		return article, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return article, err
	}
	article.ID = id
	return article, nil
}

func (db *Database) AddArticlesBulk(articles []model.Article) (int, error) {
	if len(articles) == 0 {
		return 0, nil
	}
	_tx, err := db.conn.Begin()
	if err != nil {
		return 0, err
	}
	stmt, err := _tx.Prepare(`INSERT INTO articles (blog_id, title, url, published_date, discovered_date, is_read) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = _tx.Rollback()
		return 0, err
	}
	defer stmt.Close()

	for _, article := range articles {
		_, err := stmt.Exec(
			article.BlogID,
			article.Title,
			article.URL,
			formatTimePtr(article.PublishedDate),
			formatTimePtr(article.DiscoveredDate),
			article.IsRead,
		)
		if err != nil {
			_ = _tx.Rollback()
			return 0, err
		}
	}
	if err := _tx.Commit(); err != nil {
		return 0, err
	}
	return len(articles), nil
}

func (db *Database) GetArticle(id int64) (*model.Article, error) {
	row := db.conn.QueryRow(`SELECT id, blog_id, title, url, published_date, discovered_date, is_read FROM articles WHERE id = ?`, id)
	return scanArticle(row)
}

func (db *Database) GetArticleByURL(url string) (*model.Article, error) {
	row := db.conn.QueryRow(`SELECT id, blog_id, title, url, published_date, discovered_date, is_read FROM articles WHERE url = ?`, url)
	return scanArticle(row)
}

func (db *Database) ArticleExists(url string) (bool, error) {
	row := db.conn.QueryRow(`SELECT 1 FROM articles WHERE url = ?`, url)
	var one int
	switch err := row.Scan(&one); {
	case err == nil:
		return true, nil
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	default:
		return false, err
	}
}

func (db *Database) GetExistingArticleURLs(urls []string) (map[string]struct{}, error) {
	result := make(map[string]struct{})
	if len(urls) == 0 {
		return result, nil
	}

	chunkSize := 900
	for start := 0; start < len(urls); start += chunkSize {
		end := start + chunkSize
		if end > len(urls) {
			end = len(urls)
		}
		chunk := urls[start:end]
		placeholders := strings.TrimRight(strings.Repeat("?,", len(chunk)), ",")
		query := fmt.Sprintf("SELECT url FROM articles WHERE url IN (%s)", placeholders)
		rows, err := db.conn.Query(query, interfaceSlice(chunk)...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var url string
			if err := rows.Scan(&url); err != nil {
				rows.Close()
				return nil, err
			}
			result[url] = struct{}{}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return result, nil
}

func (db *Database) ListArticles(unreadOnly bool, blogID *int64) ([]model.Article, error) {
	query := `SELECT id, blog_id, title, url, published_date, discovered_date, is_read FROM articles WHERE 1=1`
	var args []interface{}
	if unreadOnly {
		query += " AND is_read = 0"
	}
	if blogID != nil {
		query += " AND blog_id = ?"
		args = append(args, *blogID)
	}
	query += " ORDER BY discovered_date DESC"

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []model.Article
	for rows.Next() {
		article, err := scanArticle(rows)
		if err != nil {
			return nil, err
		}
		if article != nil {
			articles = append(articles, *article)
		}
	}
	return articles, rows.Err()
}

func (db *Database) MarkArticleRead(id int64) (bool, error) {
	result, err := db.conn.Exec(`UPDATE articles SET is_read = 1 WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (db *Database) MarkArticleUnread(id int64) (bool, error) {
	result, err := db.conn.Exec(`UPDATE articles SET is_read = 0 WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func scanBlog(scanner interface{ Scan(dest ...any) error }) (*model.Blog, error) {
	var (
		id             int64
		name           string
		url            string
		feedURL        sql.NullString
		scrapeSelector sql.NullString
		lastScanned    sql.NullString
	)
	if err := scanner.Scan(&id, &name, &url, &feedURL, &scrapeSelector, &lastScanned); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	blog := &model.Blog{
		ID:             id,
		Name:           name,
		URL:            url,
		FeedURL:        feedURL.String,
		ScrapeSelector: scrapeSelector.String,
	}
	if lastScanned.Valid {
		if parsed, err := parseTime(lastScanned.String); err == nil {
			blog.LastScanned = &parsed
		}
	}
	return blog, nil
}

func scanArticle(scanner interface{ Scan(dest ...any) error }) (*model.Article, error) {
	var (
		id            int64
		blogID        int64
		title         string
		url           string
		publishedDate sql.NullString
		discovered    sql.NullString
		isRead        bool
	)
	if err := scanner.Scan(&id, &blogID, &title, &url, &publishedDate, &discovered, &isRead); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	article := &model.Article{
		ID:     id,
		BlogID: blogID,
		Title:  title,
		URL:    url,
		IsRead: isRead,
	}
	if publishedDate.Valid {
		if parsed, err := parseTime(publishedDate.String); err == nil {
			article.PublishedDate = &parsed
		}
	}
	if discovered.Valid {
		if parsed, err := parseTime(discovered.String); err == nil {
			article.DiscoveredDate = &parsed
		}
	}

	return article, nil
}

func formatTimePtr(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(sqliteTimeLayout)
	return &formatted
}

func parseTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, errors.New("empty time")
	}
	parsed, err := time.Parse(sqliteTimeLayout, value)
	if err == nil {
		return parsed, nil
	}
	return time.Parse("2006-01-02 15:04:05", value)
}

func nullIfEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func interfaceSlice(values []string) []interface{} {
	result := make([]interface{}, len(values))
	for i, value := range values {
		result[i] = value
	}
	return result
}
