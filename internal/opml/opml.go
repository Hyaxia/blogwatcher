package opml

import (
	"encoding/xml"
	"io"

	"golang.org/x/net/html/charset"
)

type document struct {
	Body body `xml:"body"`
}

type body struct {
	Outlines []outline `xml:"outline"`
}

type outline struct {
	Type     string    `xml:"type,attr"`
	Text     string    `xml:"text,attr"`
	Title    string    `xml:"title,attr"`
	XMLURL   string    `xml:"xmlUrl,attr"`
	HTMLURL  string    `xml:"htmlUrl,attr"`
	Children []outline `xml:"outline"`
}

type Feed struct {
	Title   string
	SiteURL string
	FeedURL string
}

func Parse(r io.Reader) ([]Feed, error) {
	var doc document
	dec := xml.NewDecoder(r)
	dec.CharsetReader = charset.NewReaderLabel
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}

	var feeds []Feed
	for _, o := range doc.Body.Outlines {
		feeds = collectFeeds(feeds, o)
	}
	return feeds, nil
}

func collectFeeds(feeds []Feed, o outline) []Feed {
	if o.XMLURL != "" {
		title := o.Title
		if title == "" {
			title = o.Text
		}
		feeds = append(feeds, Feed{
			Title:   title,
			SiteURL: o.HTMLURL,
			FeedURL: o.XMLURL,
		})
	}
	for _, child := range o.Children {
		feeds = collectFeeds(feeds, child)
	}
	return feeds
}
