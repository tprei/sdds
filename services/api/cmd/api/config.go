package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/tprei/sdds/services/api/internal/httpapi"
)

const defaultHTTPAddr = ":8080"
const defaultDatabasePath = "sdds.db"

type config struct {
	authLimits   httpapi.AuthLimits
	databasePath string
	httpAddr     string
}

func loadConfig() (config, error) {
	authLimits, err := loadAuthLimits()
	if err != nil {
		return config{}, err
	}

	return config{
		authLimits:   authLimits,
		databasePath: envString("SDDS_DATABASE_PATH", defaultDatabasePath),
		httpAddr:     envString("SDDS_HTTP_ADDR", defaultHTTPAddr),
	}, nil
}

func envString(name string, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}

func loadAuthLimits() (httpapi.AuthLimits, error) {
	defaults := httpapi.DefaultAuthLimits()
	signupRequestsPerMinute, err := envPositiveInt("SDDS_AUTH_SIGNUP_REQUESTS_PER_MINUTE", defaults.SignupRequestsPerMinute)
	if err != nil {
		return httpapi.AuthLimits{}, err
	}
	loginRequestsPerMinute, err := envPositiveInt("SDDS_AUTH_LOGIN_REQUESTS_PER_MINUTE", defaults.LoginRequestsPerMinute)
	if err != nil {
		return httpapi.AuthLimits{}, err
	}
	signupGlobalRequestsPerMinute, err := envPositiveInt("SDDS_AUTH_GLOBAL_SIGNUP_REQUESTS_PER_MINUTE", defaults.SignupGlobalRequestsPerMinute)
	if err != nil {
		return httpapi.AuthLimits{}, err
	}
	loginGlobalRequestsPerMinute, err := envPositiveInt("SDDS_AUTH_GLOBAL_LOGIN_REQUESTS_PER_MINUTE", defaults.LoginGlobalRequestsPerMinute)
	if err != nil {
		return httpapi.AuthLimits{}, err
	}
	passwordHashConcurrency, err := envPositiveInt("SDDS_AUTH_HASH_CONCURRENCY", defaults.PasswordHashConcurrency)
	if err != nil {
		return httpapi.AuthLimits{}, err
	}

	return httpapi.AuthLimits{
		SignupRequestsPerMinute:       signupRequestsPerMinute,
		LoginRequestsPerMinute:        loginRequestsPerMinute,
		SignupGlobalRequestsPerMinute: signupGlobalRequestsPerMinute,
		LoginGlobalRequestsPerMinute:  loginGlobalRequestsPerMinute,
		PasswordHashConcurrency:       passwordHashConcurrency,
	}, nil
}

func envPositiveInt(name string, fallback int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}
