package rss

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/mmcdole/gofeed"
)

type FeedArticle struct {
	Title         string
	URL           string
	PublishedDate *time.Time
	Keywords      string
	Description   string
}

type FeedParseError struct {
	Message string
}

func (e FeedParseError) Error() string {
	return e.Message
}

func ParseFeed(feedURL string, timeout time.Duration, userAgent string) ([]FeedArticle, error) {
	client := &http.Client{Timeout: timeout}
	response, err := getWithOptionalUserAgent(client, feedURL, userAgent)
	if err != nil {
		return nil, FeedParseError{Message: fmt.Sprintf("failed to fetch feed: %v", err)}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, FeedParseError{Message: fmt.Sprintf("failed to fetch feed: status %d", response.StatusCode)}
	}

	// Use custom XML parser to capture all <keyword> tags
	articles, err := parseFeedXML(response.Body)
	if err != nil {
		return nil, FeedParseError{Message: fmt.Sprintf("failed to parse feed: %v", err)}
	}

	return articles, nil
}

// parseFeedXML uses xml.Decoder to capture all custom elements including multiple <keyword> tags
func parseFeedXML(reader io.Reader) ([]FeedArticle, error) {
	decoder := xml.NewDecoder(reader)
	var articles []FeedArticle
	var currentItem *xml.StartElement
	var currentTitle, currentLink, currentDescription string
	var currentPubDate string
	var currentKeywords []string

	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		switch elem := token.(type) {
		case xml.StartElement:
			if elem.Name.Local == "item" {
				// Start of new item, reset
				currentItem = &elem
				currentTitle = ""
				currentLink = ""
				currentDescription = ""
				currentPubDate = ""
				currentKeywords = nil
			} else if currentItem != nil {
				// Inside item, look for specific elements
				switch elem.Name.Local {
				case "title":
					if text := readTextContent(decoder, elem); text != "" {
						currentTitle = text
					}
				case "link":
					if text := readTextContent(decoder, elem); text != "" {
						currentLink = text
					}
				case "description":
					if text := readTextContent(decoder, elem); text != "" {
						currentDescription = text
					}
				case "pubDate":
					if text := readTextContent(decoder, elem); text != "" {
						currentPubDate = text
					}
				case "keyword":
					if text := readTextContent(decoder, elem); text != "" {
						currentKeywords = append(currentKeywords, text)
					}
				}
			}
		case xml.EndElement:
			if elem.Name.Local == "item" && currentTitle != "" && currentLink != "" {
				pubDate := parseRSSDate(currentPubDate)
				desc := stripHTML(currentDescription)
				articles = append(articles, FeedArticle{
					Title:         strings.TrimSpace(currentTitle),
					URL:           strings.TrimSpace(currentLink),
					PublishedDate: pubDate,
					Keywords:      strings.Join(currentKeywords, ","),
					Description:   strings.TrimSpace(desc),
				})
				currentItem = nil
			}
		}
	}

	return articles, nil
}

func readTextContent(decoder *xml.Decoder, elem xml.StartElement) string {
	var text string
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		if char, ok := token.(xml.CharData); ok {
			text += string(char)
		}
		if _, ok := token.(xml.EndElement); ok {
			break
		}
	}
	return text
}

func parseRSSDate(dateStr string) *time.Time {
	if dateStr == "" {
		return nil
	}
	// Try multiple date formats
	formats := []string{
		time.RFC1123Z,    // "Mon, 02 Jan 2006 15:04:05 -0700"
		time.RFC1123,     // "Mon, 02 Jan 2006 15:04:05 GMT"
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 GMT",
		time.RFC3339,
	}
	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return &t
		}
	}
	return nil
}

func DiscoverFeedURL(blogURL string, timeout time.Duration, userAgent string) (string, error) {
	client := &http.Client{Timeout: timeout}
	response, err := getWithOptionalUserAgent(client, blogURL, userAgent)
	if err != nil {
		return "", nil
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", nil
	}

	contentType := response.Header.Get("Content-Type")
	// If the URL already returns a feed content-type, validate and return it directly
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil {
		// Only accept explicit feed types, not generic XML (to avoid sitemap false positives)
		if mediaType == "application/rss+xml" || mediaType == "application/atom+xml" || mediaType == "application/feed+json" {
			return blogURL, nil
		}
	}

	base, err := url.Parse(blogURL)
	if err != nil {
		return "", nil
	}

	doc, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		return "", nil
	}

	feedTypes := []string{
		"application/rss+xml",
		"application/atom+xml",
		"application/feed+json",
		"application/xml",
		"text/xml",
	}

	for _, feedType := range feedTypes {
		// Use token matching for rel (rel~='value' matches rel as space-separated token)
		// Use case-insensitive type matching [type~='value']
		selection := doc.Find(fmt.Sprintf("link[rel~='alternate'][type~='%s']", feedType)).First()
		if selection.Length() == 0 {
			// Also check rel="self" for feeds that use self-referencing links (e.g. TechCrunch tag feeds)
			selection = doc.Find(fmt.Sprintf("link[rel~='self'][type~='%s']", feedType)).First()
		}
		if selection.Length() == 0 {
			continue
		}
		href, exists := selection.Attr("href")
		if !exists {
			continue
		}
		resolved := resolveURL(base, href)
		if resolved != "" {
			return resolved, nil
		}
	}

	commonPaths := []string{
		"/feed",
		"/feed/",
		"/rss",
		"/rss/",
		"/feed.xml",
		"/rss.xml",
		"/atom.xml",
		"/index.xml",
	}

	for _, path := range commonPaths {
		resolved := resolveURL(base, path)
		if resolved == "" {
			continue
		}
		ok, err := isValidFeed(resolved, timeout, userAgent)
		if err == nil && ok {
			return resolved, nil
		}
	}

	return "", nil
}

func isValidFeed(feedURL string, timeout time.Duration, userAgent string) (bool, error) {
	client := &http.Client{Timeout: timeout}
	response, err := getWithOptionalUserAgent(client, feedURL, userAgent)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false, nil
	}

	parser := gofeed.NewParser()
	feed, err := parser.Parse(response.Body)
	if err != nil {
		return false, err
	}

	return len(feed.Items) > 0 || strings.TrimSpace(feed.Title) != "", nil
}

func getWithOptionalUserAgent(client *http.Client, targetURL string, userAgent string) (*http.Response, error) {
	request, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(userAgent) != "" {
		request.Header.Set("User-Agent", userAgent)
	}
	return client.Do(request)
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

func stripHTML(html string) string {
	re := strings.NewReplacer(
		"<br>", " ",
		"<br/>", " ",
		"<br />", " ",
		"<p>", " ",
		"</p>", " ",
		"<div>", " ",
		"</div>", " ",
		"&nbsp;", " ",
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
	)
	html = re.Replace(html)
	for strings.Contains(html, "<") && strings.Contains(html, ">") {
		start := strings.Index(html, "<")
		end := strings.Index(html, ">")
		if end > start {
			html = html[:start] + html[end+1:]
		} else {
			break
		}
	}
	return strings.TrimSpace(html)
}

func IsFeedError(err error) bool {
	var parseErr FeedParseError
	return errors.As(err, &parseErr)
}
