package config

import (
	"golang-project-boilerplate/internal/utils/logger"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gopkg.in/natefinch/lumberjack.v2"
)

func NewLogger(viper *viper.Viper) *logger.Logger {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})

	appName := viper.GetString("app.name")
	stdout := viper.GetBool("log.stdout")
	path := viper.GetString("log.path")
	if path == "" {
		panic("config 'log.path' cannot be empty!")
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

	return logger.New(log, appName)
}
