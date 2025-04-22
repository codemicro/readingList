package database

import (
	"context"
	"fmt"
	"git.tdpain.net/codemicro/readingList/cmd/readinglistd/internal/config"
	"git.tdpain.net/codemicro/readingList/models"
	_ "github.com/mattn/go-sqlite3"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"time"
)

type DB struct {
	client   *mongo.Client
	database *mongo.Database
}

func NewDB(conf *config.Config) (*DB, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(conf.MongoDSN))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	return &DB{client: client, database: client.Database(conf.MongoDatabase)}, nil
}

func (db *DB) InsertArticle(article *models.Article) error {
	res, err := db.database.Collection("articles").InsertOne(context.Background(), article)
	if err != nil {
		return fmt.Errorf("insert new article: %w", err)
	}
	fmt.Printf("ID: %v\n", res.InsertedID)
	article.ID = res.InsertedID
	return nil
}

func (db *DB) GetAllArticles() ([]*models.Article, error) {
	cursor, err := db.database.Collection("articles").Aggregate(context.Background(), mongo.Pipeline{
		bson.D{{"$sort", bson.D{{"date", 1}}}},
	})
	if err != nil {
		return nil, fmt.Errorf("find all articles: %w", err)
	}
	defer cursor.Close(context.Background())

	var results []*models.Article
	if err := cursor.All(context.Background(), &results); err != nil {
		return nil, fmt.Errorf("read all articles: %w", err)
	}

	return results, nil
}

func (db *DB) GetArticlesForMonth(year int, month int) ([]*models.Article, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	var end time.Time
	if month+1 > 12 {
		end = time.Date(year+1, time.Month(1), 1, 0, 0, 0, 0, time.UTC)
	} else {
		end = time.Date(year, time.Month(month+1), 1, 0, 0, 0, 0, time.UTC)
	}

	cursor, err := db.database.Collection("articles").Aggregate(context.Background(), mongo.Pipeline{
		bson.D{{"$match", bson.D{{"date", bson.D{{"$gt", start}}}}}},
		bson.D{{"$match", bson.D{{"date", bson.D{{"$lt", end}}}}}},
		bson.D{{"$sort", bson.D{{"date", 1}}}},
	})
	if err != nil {
		return nil, fmt.Errorf("find all articles: %w", err)
	}
	defer cursor.Close(context.Background())

	var results []*models.Article
	if err := cursor.All(context.Background(), &results); err != nil {
		return nil, fmt.Errorf("read all articles: %w", err)
	}

	return results, nil
}
