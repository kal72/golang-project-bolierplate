package main

import (
	"golang-project-boilerplate/internal/app"
	"golang-project-boilerplate/internal/config"
)

func main() {
	cfg := config.NewConfig()
	fiberApp := config.NewFiber(cfg)
	lifecycle := app.Container(fiberApp, cfg)
	app.RunWithGracefulShutdown(fiberApp, cfg, lifecycle)
}
