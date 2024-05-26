package models

import (
	"time"
	"github.com/google/uuid"
)

type NewArticle struct {
	URL           string `validate:"required,url"`
	Title         string `validate:"required"`
	Description   string `db:"nullzero"`
	ImageURL      string `db:"image_url,nullzero"`
	Date          time.Time `validate:"required"`
}

type Article struct {
	NewArticle
	ID            uuid.UUID
	HackerNewsURL string `db:"hacker_news_url,nullzero"`
}
