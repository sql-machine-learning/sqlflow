package sql

import (
	"github.com/sirupsen/logrus"
	"os"
)

var log *logrus.Entry

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}

func init() {
	logDir := getEnv("SQLFLOW_log_dir", "logs")
	logLevel := getEnv("SQLFLOW_log_level", "info")

	ll, e := logrus.ParseLevel(logLevel)
	if e != nil {
		ll = logrus.InfoLevel
	}

	e = os.MkdirAll(logDir, 0744)
	if e != nil {
		log.Panicf("create log directory failed: %v", e)
	}

	lg := logrus.New()
	lg.SetOutput(os.Stdout)
	lg.SetLevel(ll)
	log = lg.WithFields(logrus.Fields{"package": "sql"})
}
