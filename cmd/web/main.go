package main

import (
	"golang-project-boilerplate/di"
	"log"
)

func main() {
	application, err := di.InitializeApp()
	if err != nil {
		log.Fatalf("failed to initialize app: %v", err)
	}

	application.RunWithGracefulShutdown()
}
