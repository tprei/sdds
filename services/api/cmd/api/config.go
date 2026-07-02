package main

import "os"

const defaultHTTPAddr = ":8080"
const defaultDatabasePath = "sdds.db"

type config struct {
	databasePath string
	httpAddr     string
}

func loadConfig() config {
	return config{
		databasePath: envString("SDDS_DATABASE_PATH", defaultDatabasePath),
		httpAddr:     envString("SDDS_HTTP_ADDR", defaultHTTPAddr),
	}
}

func envString(name string, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}
