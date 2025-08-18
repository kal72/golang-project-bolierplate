package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	AppName string
	Host    string
	Port    int
	LogPath string
}

func NewViper() *viper.Viper {
	viper := viper.New()

	viper.SetConfigName("config")
	viper.SetConfigType("json")
	viper.AddConfigPath("./../")
	viper.AddConfigPath("./")
	err := viper.ReadInConfig()

	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %w \n", err))
	}

	return viper
}
