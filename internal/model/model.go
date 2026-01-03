package model

import "time"

type Blog struct {
	ID             int64
	Name           string
	URL            string
	FeedURL        string
	ScrapeSelector string
	LastScanned    *time.Time
}

type Article struct {
	ID             int64
	BlogID         int64
	Title          string
	URL            string
	PublishedDate  *time.Time
	DiscoveredDate *time.Time
	IsRead         bool
}
