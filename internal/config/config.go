package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	App struct {
		Name string
		Host string
		Port int
	}
	Log struct {
		Path   string
		Stdout bool
	}
	Database struct {
		Username string
		Password string
		Host     string
		Port     int
		Name     string
		Pool     struct {
			Idle     int
			Max      int
			Lifetime int
		}
	}
	Redis struct {
		Username string
		Password string
		Host     string
		Port     int
		DB       int
		Pool     struct {
			Idle    int
			Max     int
			Timeout int
		}
	}
	Jwt struct {
		Secret  string
		Expired int //minutes
	}
}

func NewConfig() *Config {
	viper := viper.New()

	viper.SetConfigName("config")
	viper.SetConfigType("json")
	viper.AddConfigPath("./../")
	viper.AddConfigPath("./")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Fatal error config file: %w \n", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		log.Fatalf("Fatal error parse config: %w \n", err)
	}

	return &config
}
