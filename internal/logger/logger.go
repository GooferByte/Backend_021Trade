package logger

import (
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// New returns a configured logrus.Logger. JSON output is used to keep logs structured.
func New(env string) *logrus.Logger {
	log := logrus.New()
	log.SetOutput(os.Stdout)
	log.SetLevel(parseLevel(env))
	log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
	})
	return log
}

func parseLevel(env string) logrus.Level {
	if strings.ToLower(env) == "local" || strings.ToLower(env) == "dev" {
		return logrus.DebugLevel
	}
	return logrus.InfoLevel
}
