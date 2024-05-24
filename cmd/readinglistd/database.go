package main

import (
	"fmt"

	"git.tdpain.netcodemicro/readingList/models"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func NewDB(fname string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("sqlite3", fname)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS articles(
		"id" varchar not null primary key,
		"date" datetime not null,
		"url" varchar not null,
		"title" varchar not null,
		"description" varchar,
		"image_url" varchar,
		"hacker_news_url" varchar
	)`)
	if err != nil {
		return nil, fmt.Errorf("create table: %w", err)
	}

	return db, nil
}

func InsertArticle(db *sqlx.DB, article *models.Article) error {
	_, err := db.NamedExec(
		`INSERT INTO articles("id", "date", "url", "title", "description", "image_url", "hacker_news_url") VALUES (:id, :date, :url, :title, :description, :image_url, :hacker_news_url)`,
		article,
	)
	if err != nil {
		return fmt.Errorf("insert article: %w", err)
	}
	return nil
}

func GetAllArticles(db *sqlx.DB) ([]*models.Article, error) {
	articles := []*models.Article{}
	err := db.Select(&articles, `SELECT * FROM articles`)
	if err != nil {
		return nil, fmt.Errorf("select all articles: %w", err)
	}
	// if err := res.StructScan(&articles); err != nil {
	// 	return nil, fmt.Errorf("scan article results: %w", err)
	// }
	return articles, nil
}
