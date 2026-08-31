package rss

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

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

type ParseOptions struct {
	StoreDescriptions   bool
	StoreKeywords       bool
	DescriptionMaxChars int
}

const (
	defaultDescriptionMaxChars int   = 1000
	maxBufferedFeedBytes       int64 = 10 << 20
)

type FeedParseError struct {
	Message string
}

func (e FeedParseError) Error() string {
	return e.Message
}

func ParseFeed(feedURL string, timeout time.Duration, userAgent string) ([]FeedArticle, error) {
	return ParseFeedWithOptions(feedURL, timeout, userAgent, ParseOptions{
		StoreDescriptions:   true,
		StoreKeywords:       false,
		DescriptionMaxChars: defaultDescriptionMaxChars,
	})
}

func ParseFeedWithOptions(feedURL string, timeout time.Duration, userAgent string, options ParseOptions) ([]FeedArticle, error) {
	if options.StoreDescriptions && options.DescriptionMaxChars < 0 {
		return nil, FeedParseError{Message: "description maximum must be zero or greater"}
	}

	client := &http.Client{Timeout: timeout}
	response, err := getWithOptionalUserAgent(client, feedURL, userAgent)
	if err != nil {
		return nil, FeedParseError{Message: fmt.Sprintf("failed to fetch feed: %v", err)}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, FeedParseError{Message: fmt.Sprintf("failed to fetch feed: status %d", response.StatusCode)}
	}

	var (
		feed        *gofeed.Feed
		keywordSets [][]string
		keywordBody boundedKeywordFeedBuffer
	)
	parser := gofeed.NewParser()
	if options.StoreKeywords {
		feed, err = parser.Parse(io.TeeReader(response.Body, &keywordBody))
		if err == nil && !keywordBody.exceeded {
			// Keyword extraction is supplementary. A failure here must not regress
			// a feed that gofeed parsed successfully.
			keywordSets, _ = parseKeywordSets(keywordBody.Bytes())
		}
	} else {
		feed, err = parser.Parse(response.Body)
	}
	if err != nil {
		return nil, FeedParseError{Message: fmt.Sprintf("failed to parse feed: %v", err)}
	}

	articles := make([]FeedArticle, 0, len(feed.Items))
	for index, item := range feed.Items {
		title := strings.TrimSpace(item.Title)
		link := strings.TrimSpace(item.Link)
		if title == "" || link == "" {
			continue
		}

		article := FeedArticle{
			Title:         title,
			URL:           link,
			PublishedDate: pickPublishedDate(item),
		}
		if options.StoreKeywords && index < len(keywordSets) {
			article.Keywords = strings.Join(keywordSets[index], ",")
		}
		if options.StoreDescriptions {
			article.Description = selectDescription(item.Description, item.Content, options.DescriptionMaxChars)
		}
		articles = append(articles, article)
	}

	return articles, nil
}

type boundedKeywordFeedBuffer struct {
	body     bytes.Buffer
	exceeded bool
}

func (buffer *boundedKeywordFeedBuffer) Write(data []byte) (int, error) {
	if buffer.exceeded {
		return len(data), nil
	}
	remaining := maxBufferedFeedBytes - int64(buffer.body.Len())
	if int64(len(data)) > remaining {
		if remaining > 0 {
			_, _ = buffer.body.Write(data[:int(remaining)])
		}
		buffer.exceeded = true
		return len(data), nil
	}
	_, err := buffer.body.Write(data)
	return len(data), err
}

func (buffer *boundedKeywordFeedBuffer) Bytes() []byte {
	return buffer.body.Bytes()
}

func parseKeywordSets(body []byte) ([][]string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(body))
	var (
		sets           [][]string
		current        []string
		keywordText    strings.Builder
		depth          int
		containerDepth int
		keywordDepth   int
	)

	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return sets, nil
			}
			return nil, err
		}

		switch element := token.(type) {
		case xml.StartElement:
			depth++
			if containerDepth == 0 && (element.Name.Local == "item" || element.Name.Local == "entry") {
				containerDepth = depth
				current = nil
				continue
			}
			if containerDepth > 0 && depth == containerDepth+1 && element.Name.Local == "keyword" {
				keywordDepth = depth
				keywordText.Reset()
			}
		case xml.CharData:
			if keywordDepth > 0 {
				keywordText.Write([]byte(element))
			}
		case xml.EndElement:
			if keywordDepth == depth && element.Name.Local == "keyword" {
				if keyword := strings.TrimSpace(keywordText.String()); keyword != "" {
					current = append(current, keyword)
				}
				keywordDepth = 0
			}
			if containerDepth == depth && (element.Name.Local == "item" || element.Name.Local == "entry") {
				sets = append(sets, current)
				containerDepth = 0
				current = nil
			}
			depth--
		}
	}
}

var blankLinePattern = regexp.MustCompile(`\r?\n[\t ]*\r?\n+`)

func selectDescription(description string, content string, maxChars int) string {
	selected := firstMeaningfulParagraph(description)
	if selected == "" {
		selected = firstMeaningfulParagraph(content)
	}
	return limitAtWordBoundary(selected, maxChars)
}

func firstMeaningfulParagraph(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if !strings.Contains(raw, "<") {
		blocks := blankLinePattern.Split(html.UnescapeString(raw), -1)
		for _, block := range blocks {
			if text := normalizeWhitespace(block); text != "" {
				return text
			}
		}
		return ""
	}

	document, err := goquery.NewDocumentFromReader(strings.NewReader(raw))
	if err != nil {
		return normalizeWhitespace(html.UnescapeString(raw))
	}
	document.Find("script, style").Remove()

	var paragraph string
	document.Find("p").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
		paragraph = normalizeWhitespace(selection.Text())
		return paragraph == ""
	})
	if paragraph != "" {
		return paragraph
	}
	return normalizeWhitespace(document.Text())
}

func normalizeWhitespace(value string) string {
	return strings.Join(strings.Fields(html.UnescapeString(value)), " ")
}

func limitAtWordBoundary(value string, maxChars int) string {
	if value == "" || maxChars == 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxChars {
		return value
	}
	if maxChars == 1 {
		return "…"
	}

	cutoff := maxChars - 1
	boundary := cutoff
	foundBoundary := unicode.IsSpace(runes[cutoff])
	lookbackStart := max(cutoff-32, 0)
	for index := cutoff - 1; !foundBoundary && index >= lookbackStart; index-- {
		switch {
		case unicode.IsPunct(runes[index]):
			boundary = index + 1
			foundBoundary = true
		case unicode.IsSpace(runes[index]):
			boundary = index
			foundBoundary = true
		}
	}
	prefix := strings.TrimSpace(string(runes[:boundary]))
	if prefix == "" {
		prefix = string(runes[:cutoff])
	}
	return prefix + "…"
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
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil {
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
		selection := doc.Find(fmt.Sprintf("link[rel~='alternate'][type~='%s']", feedType)).First()
		if selection.Length() == 0 {
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

func pickPublishedDate(item *gofeed.Item) *time.Time {
	if item == nil {
		return nil
	}
	if item.PublishedParsed != nil {
		return item.PublishedParsed
	}
	if item.UpdatedParsed != nil {
		return item.UpdatedParsed
	}
	return nil
}

func IsFeedError(err error) bool {
	var parseErr FeedParseError
	return errors.As(err, &parseErr)
}
