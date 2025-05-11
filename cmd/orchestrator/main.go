package main

import (
	"distributed-calculator/internal/orchestrator"
	"log"
)

func main() {
	server, _ := orchestrator.NewServer()
	log.Println("Orchestrator is starting...")
	server.Start()
}
