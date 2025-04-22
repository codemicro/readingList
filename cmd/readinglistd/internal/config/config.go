package config

import (
	"go.akpain.net/cfger"
)

type Config struct {
	Token         string
	HTTPAddress   string
	MongoDSN      string
	MongoDatabase string
}

func Get() (*Config, error) {
	cl := cfger.New()
	var conf = &Config{
		Token:         cl.GetEnv("READINGLISTD_INGEST_TOKEN").Required().AsString(),
		HTTPAddress:   cl.GetEnv("READINGLISTD_HTTP_ADDR").WithDefault(":9231").AsString(),
		MongoDSN:      cl.GetEnv("READINGLISTD_MONGO_DSN").WithDefault("mongodb://localhost/readinglist").AsString(),
		MongoDatabase: cl.GetEnv("READINGLISTD_MONGO_DATABASE").WithDefault("readinglist").AsString(),
	}
	return conf, nil
}
