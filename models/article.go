package models

import (
	"time"
)

type NewArticle struct {
	URL         string `validate:"required,url"`
	Title       string `validate:"required"`
	Description string
	ImageURL    string    `bson:"image_url"`
	Date        time.Time `validate:"required"`
	IsFavourite bool      `bson:"is_favourite"`
}

type Article struct {
	NewArticle    `bson:",inline"`
	ID            any    `bson:"_id,omitempty"`
	HackerNewsURL string `bson:"hacker_news_url"`
}
