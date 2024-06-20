package config

import (
	"go.akpain.net/cfger"
)

type Config struct {
	Token                  string
	HTTPAddress            string
	DatabaseFilename       string
	PalmatumAuthentication string
	PalmatumSiteName       string
}

func Get() (*Config, error) {
	cl := cfger.New()
	var conf = &Config{
		Token:                  cl.GetEnv("READINGLISTD_INGEST_TOKEN").Required().AsString(),
		HTTPAddress:            cl.GetEnv("READINGLISTD_HTTP_ADDR").WithDefault(":9231").AsString(),
		DatabaseFilename:       cl.GetEnv("READINGLISTD_DATABASE_FILENAME").WithDefault("readinglist.sqlite3.db").AsString(),
		PalmatumAuthentication: cl.GetEnv("READINGLISTD_PALMATUM_AUTH").Required().AsString(),
		PalmatumSiteName:       cl.GetEnv("READINGLISTD_SITE_NAME").Required().AsString(),
	}
	return conf, nil
}
