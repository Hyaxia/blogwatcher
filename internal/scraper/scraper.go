package scraper

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type ScrapedArticle struct {
	Title         string
	URL           string
	PublishedDate *time.Time
}

type ScrapeError struct {
	Message string
}

func (e ScrapeError) Error() string {
	return e.Message
}

func ScrapeBlog(blogURL string, selector string, timeout time.Duration) ([]ScrapedArticle, error) {
	client := &http.Client{Timeout: timeout}
	response, err := client.Get(blogURL)
	if err != nil {
		return nil, ScrapeError{Message: fmt.Sprintf("failed to fetch page: %v", err)}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, ScrapeError{Message: fmt.Sprintf("failed to fetch page: status %d", response.StatusCode)}
	}

	base, err := url.Parse(blogURL)
	if err != nil {
		return nil, ScrapeError{Message: "invalid blog url"}
	}

	doc, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		return nil, ScrapeError{Message: fmt.Sprintf("failed to parse page: %v", err)}
	}

	seen := make(map[string]struct{})
	var articles []ScrapedArticle

	doc.Find(selector).Each(func(_ int, selection *goquery.Selection) {
		link := selection
		if goquery.NodeName(selection) != "a" {
			link = selection.Find("a").First()
		}
		if link.Length() == 0 {
			return
		}
		href, exists := link.Attr("href")
		if !exists {
			return
		}
		resolved := resolveURL(base, href)
		if resolved == "" {
			return
		}
		if _, ok := seen[resolved]; ok {
			return
		}
		seen[resolved] = struct{}{}

		title := extractTitle(link, selection)
		if title == "" {
			return
		}
		articles = append(articles, ScrapedArticle{
			Title: title,
			URL:   resolved,
		})
	})

	return articles, nil
}

func extractTitle(link *goquery.Selection, parent *goquery.Selection) string {
	text := strings.TrimSpace(link.Text())
	if text != "" {
		return text
	}
	if title, exists := link.Attr("title"); exists {
		title = strings.TrimSpace(title)
		if title != "" {
			return title
		}
	}
	if parent != nil && parent != link {
		text = strings.TrimSpace(parent.Text())
		if text != "" {
			return text
		}
	}
	return ""
}

func resolveURL(base *url.URL, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	parsed, err := url.Parse(href)
	if err != nil {
		return ""
	}
	return base.ResolveReference(parsed).String()
}

func IsScrapeError(err error) bool {
	var scrapeErr ScrapeError
	return errors.As(err, &scrapeErr)
}
