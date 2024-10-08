package models

import (
	"github.com/google/uuid"
	"time"
)

type NewArticle struct {
	URL         string    `validate:"required,url"`
	Title       string    `validate:"required"`
	Description string    `db:"description"`
	ImageURL    string    `db:"image_url"`
	Date        time.Time `db:"date" validate:"required"`
	IsFavourite bool      `db:"is_favourite"`
}

type Article struct {
	NewArticle
	ID            uuid.UUID
	HackerNewsURL string `db:"hacker_news_url"`
}
