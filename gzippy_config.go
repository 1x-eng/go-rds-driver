package rds

import (
	"database/sql"
	"os"
)

func isErrorFree(e ...bool) bool {
	for _, an_err := range e {
		if !an_err {
			return false
		}
	}
	return true
}

// lookup env vars for gzippy config
func ZippyRDSConfig() (conf *Config, err error) {
	zippyRegion, regionPresent := os.LookupEnv("ZIPPY_REGION")
	zippyDatabase, dbPresent := os.LookupEnv("ZIPPY_DATABASE")
	zippyDBResourceARN, dbARNPresent := os.LookupEnv("ZIPPY_DB_RESOURCE_ARN")
	zippyDBSecretARN, dbSecretARNPresent := os.LookupEnv("ZIPPY_DB_SECRET_ARN")

	if !isErrorFree(regionPresent, dbPresent, dbARNPresent, dbSecretARNPresent) {
		panic("One or more necessary env vars for Zippy stack unavailable.")
	}

	conf = &Config{
		ResourceArn: zippyDBResourceARN,
		SecretArn:   zippyDBSecretARN,
		Database:    zippyDatabase,
		AWSRegion:   zippyRegion,
		ParseTime:   true,
	}

	return conf, nil
}

func ZippyRDSClient() *sql.DB {
	conf, zippyConfigErr := ZippyRDSConfig()
	if zippyConfigErr != nil {
		panic(zippyConfigErr)
	}

	dsn, confToDSNErr := conf.ToDSN()
	if confToDSNErr != nil {
		panic(confToDSNErr)
	}

	client, err := sql.Open("rds", dsn)
	if err != nil {
		panic(err)
	}

	return client
}
