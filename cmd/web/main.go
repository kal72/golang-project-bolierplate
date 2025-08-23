package main

import (
	"golang-project-boilerplate/internal/app"
	"golang-project-boilerplate/internal/config"
)

func main() {
	newConfig := config.NewConfig()
	app := app.NewContainer(newConfig)
	app.SetupRoutes()
	app.GracefullyShutdown(app.Run)
}
