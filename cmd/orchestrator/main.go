package main

import (
	"distributed-calculator/internal/orchestrator"
	"log"
)

func main() {
	// Инициализация и запуск оркестратора
	server := orchestrator.NewServer()
	log.Println("Orchestrator is starting...")
	server.Start()
}
