package config

import (
	"golang-project-boilerplate/internal/shared/logger"
	"os"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

func NewLogger(cfg *Config) *logger.Logger {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})

	appName := cfg.App.Name
	stdout := cfg.Log.Stdout
	path := cfg.Log.Path
	if path == "" {
		log.Fatalf("config 'log.path' cannot be empty!")
	}

	if stdout {
		log.SetOutput(os.Stdout)
	} else {

		lumberjackLogger := &lumberjack.Logger{
			Filename:   path,
			MaxSize:    10,
			MaxBackups: 3,
			MaxAge:     30,
			LocalTime:  true,
		}

		log.SetOutput(lumberjackLogger)
	}

	return logger.New(log, appName, 4000)
}
