package main

import (
	"golang-project-boilerplate/internal/app"
	"log"
)

func main() {
	application, err := app.InitializeApp()
	if err != nil {
		log.Fatalf("failed to initialize app: %v", err)
	}

	application.RunWithGracefulShutdown()
}
