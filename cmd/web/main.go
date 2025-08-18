package main

import (
	"fmt"
	"golang-project-boilerplate/internal/config"
	"log"
)

func main() {
	viper := config.NewViper()
	logger := config.NewLogger(viper)
	fiberApp := config.NewFiber(viper)
	db := config.NewDatabase(viper, logger)
	validate := config.NewValidator()

	appConfig := new(config.AppConfig)
	appConfig.Log = logger
	appConfig.FiberApp = fiberApp
	appConfig.DB = db
	appConfig.Validate = validate
	appConfig.Config = viper
	config.Container(appConfig)

	host := viper.GetString("app.host")
	port := viper.GetInt("app.port")

	fmt.Printf("Starting server %s:%d", host, port)
	err := fiberApp.Listen(fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
