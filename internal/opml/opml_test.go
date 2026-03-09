package opml

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="1.0">
    <head><title>Test</title></head>
    <body>
        <outline text="Category1" title="Category1">
            <outline type="rss" text="Blog One" title="Blog One" xmlUrl="http://blog1.com/feed" htmlUrl="http://blog1.com"/>
            <outline type="rss" text="Blog Two" title="Blog Two" xmlUrl="http://blog2.com/rss" htmlUrl="http://blog2.com"/>
        </outline>
        <outline text="Category2" title="Category2">
            <outline type="rss" text="Blog Three" title="Blog Three" xmlUrl="http://blog3.com/atom.xml" htmlUrl="http://blog3.com"/>
        </outline>
    </body>
</opml>`

	feeds, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(feeds) != 3 {
		t.Fatalf("expected 3 feeds, got %d", len(feeds))
	}

	expected := []Feed{
		{Title: "Blog One", SiteURL: "http://blog1.com", FeedURL: "http://blog1.com/feed"},
		{Title: "Blog Two", SiteURL: "http://blog2.com", FeedURL: "http://blog2.com/rss"},
		{Title: "Blog Three", SiteURL: "http://blog3.com", FeedURL: "http://blog3.com/atom.xml"},
	}
	for i, f := range feeds {
		if f != expected[i] {
			t.Errorf("feed %d: got %+v, want %+v", i, f, expected[i])
		}
	}
}

func TestParseEmptyCategories(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="1.0">
    <head><title>Test</title></head>
    <body>
        <outline text="Empty" title="Empty"/>
        <outline text="HasOne" title="HasOne">
            <outline type="rss" text="Blog" title="Blog" xmlUrl="http://blog.com/feed" htmlUrl="http://blog.com"/>
        </outline>
        <outline text="AlsoEmpty" title="AlsoEmpty"/>
    </body>
</opml>`

	feeds, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(feeds) != 1 {
		t.Fatalf("expected 1 feed, got %d", len(feeds))
	}
	if feeds[0].Title != "Blog" {
		t.Errorf("expected title 'Blog', got '%s'", feeds[0].Title)
	}
}

func TestParseUsesTextWhenTitleEmpty(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="1.0">
    <head><title>Test</title></head>
    <body>
        <outline text="Cat" title="Cat">
            <outline type="rss" text="Fallback Name" xmlUrl="http://example.com/feed" htmlUrl="http://example.com"/>
        </outline>
    </body>
</opml>`

	feeds, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(feeds) != 1 {
		t.Fatalf("expected 1 feed, got %d", len(feeds))
	}
	if feeds[0].Title != "Fallback Name" {
		t.Errorf("expected title 'Fallback Name', got '%s'", feeds[0].Title)
	}
}

func TestParseSkipsOutlineWithoutXmlUrl(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="1.0">
    <head><title>Test</title></head>
    <body>
        <outline text="Cat" title="Cat">
            <outline type="rss" text="No Feed" title="No Feed" htmlUrl="http://example.com"/>
            <outline type="rss" text="Has Feed" title="Has Feed" xmlUrl="http://example.com/feed" htmlUrl="http://example.com"/>
        </outline>
    </body>
</opml>`

	feeds, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(feeds) != 1 {
		t.Fatalf("expected 1 feed, got %d", len(feeds))
	}
	if feeds[0].Title != "Has Feed" {
		t.Errorf("expected title 'Has Feed', got '%s'", feeds[0].Title)
	}
}

func TestParseSubscriptionList(t *testing.T) {
	// Based on opml.org/examples/subscriptionList.opml
	// Flat list of RSS feeds without categories (OPML 2.0)
	input := `<?xml version="1.0" encoding="ISO-8859-1"?>
<opml version="2.0">
	<head><title>mySubscriptions.opml</title></head>
	<body>
		<outline text="CNET News.com" description="Tech news" htmlUrl="http://news.com.com/" language="unknown" title="CNET News.com" type="rss" version="RSS2" xmlUrl="http://news.com.com/2547-1_3-0-5.xml"/>
		<outline text="washingtonpost.com - Politics" description="Politics" htmlUrl="http://www.washingtonpost.com/wp-dyn/politics" language="unknown" title="washingtonpost.com - Politics" type="rss" version="RSS2" xmlUrl="http://www.washingtonpost.com/wp-srv/politics/rssheadlines.xml"/>
		<outline text="Scripting News" description="It's even worse than it appears." htmlUrl="http://www.scripting.com/" language="unknown" title="Scripting News" type="rss" version="RSS2" xmlUrl="http://www.scripting.com/rss.xml"/>
	</body>
</opml>`

	feeds, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(feeds) != 3 {
		t.Fatalf("expected 3 feeds, got %d", len(feeds))
	}
	if feeds[0].Title != "CNET News.com" {
		t.Errorf("expected 'CNET News.com', got '%s'", feeds[0].Title)
	}
	if feeds[0].FeedURL != "http://news.com.com/2547-1_3-0-5.xml" {
		t.Errorf("unexpected feed URL: %s", feeds[0].FeedURL)
	}
	if feeds[0].SiteURL != "http://news.com.com/" {
		t.Errorf("unexpected site URL: %s", feeds[0].SiteURL)
	}
}

func TestParseNoFeeds(t *testing.T) {
	// Based on opml.org/examples/states.opml — hierarchical outline with no RSS feeds
	input := `<?xml version="1.0"?>
<opml version="2.0">
	<head><title>states.opml</title></head>
	<body>
		<outline text="United States">
			<outline text="Far West">
				<outline text="Alaska"/>
				<outline text="California"/>
			</outline>
			<outline text="New England">
				<outline text="Connecticut"/>
				<outline text="Maine"/>
			</outline>
		</outline>
	</body>
</opml>`

	feeds, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(feeds) != 0 {
		t.Fatalf("expected 0 feeds from states outline, got %d", len(feeds))
	}
}

func TestParseDirectoryLinks(t *testing.T) {
	// Based on opml.org/examples/directory.opml — type="link" outlines, no xmlUrl
	input := `<?xml version="1.0" encoding="ISO-8859-1"?>
<opml version="2.0">
	<head><title>scriptingNewsDirectory.opml</title></head>
	<body>
		<outline text="Scripting News sites" type="link" url="http://hosting.opml.org/dave/mySites.opml"/>
		<outline text="News.Com top 100 OPML" type="link" url="http://news.com.com/html/ne/blogs/CNETNewsBlog100.opml"/>
		<outline text="TechCrunch reviews" type="link" url="http://hosting.opml.org/techcrunch.opml.org/TechCrunch.opml"/>
	</body>
</opml>`

	feeds, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(feeds) != 0 {
		t.Fatalf("expected 0 feeds from directory links, got %d", len(feeds))
	}
}

func TestParseCategoryAttribute(t *testing.T) {
	// Based on opml.org/examples/category.opml
	input := `<?xml version="1.0" encoding="ISO-8859-1"?>
<opml version="2.0">
	<head><title>Illustrating the category attribute</title></head>
	<body>
		<outline text="The Mets are the best team in baseball." category="/Philosophy/Baseball/Mets,/Tourism/New York"/>
	</body>
</opml>`

	feeds, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(feeds) != 0 {
		t.Fatalf("expected 0 feeds from category example, got %d", len(feeds))
	}
}
