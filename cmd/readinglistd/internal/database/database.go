package database

import (
	"database/sql"
	"errors"
	"fmt"
	"git.tdpain.net/codemicro/readingList/models"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

const programSchemaVersion = 2

func NewDB(fname string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("sqlite3", fname)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS schema_version(
		"n" integer not null
	)`)
	if err != nil {
		return nil, fmt.Errorf("create schema_version table: %w", err)
	}

	var currentSchemaVersion int
	if err := db.QueryRowx("SELECT n FROM schema_version").Scan(&currentSchemaVersion); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("unable to read schema version from database: %w", err)
		}
	}

	fmt.Println("current schema version", currentSchemaVersion)

	switch currentSchemaVersion {
	case 0:
		// Note that version 0 did not originally include a schema_version mechanism so a v0 db so this statement must be if not exists
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
			return nil, fmt.Errorf("create articles table: %w", err)
		}
		currentSchemaVersion = 1
		fallthrough
	case 1:
		_, err = db.Exec(`ALTER TABLE articles ADD COLUMN is_favourite INTEGER NOT NULL DEFAULT FALSE`)
		if err != nil {
			return nil, fmt.Errorf("add is_favourite to articles table: %w", err)
		}
		currentSchemaVersion = 2
		fallthrough
	case programSchemaVersion:
		// noop
	}

	_, err = db.Exec(`DELETE FROM schema_version`)
	if err != nil {
		return nil, fmt.Errorf("delete old schema version number: %w", err)
	}

	_, err = db.Exec(`INSERT INTO schema_version(n) VALUES (?)`, programSchemaVersion)
	if err != nil {
		return nil, fmt.Errorf("insert schema version number: %w", err)
	}

	return db, nil
}

func InsertArticle(db *sqlx.DB, article *models.Article) error {
	_, err := db.NamedExec(
		`INSERT INTO articles("id", "date", "url", "title", "description", "image_url", "hacker_news_url") VALUES (:id, :date, :url, :title, :description, :image_url, :hacker_news_url, :is_favourite)`,
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
