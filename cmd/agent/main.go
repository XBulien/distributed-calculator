package main

import (
	"distributed-calculator/internal/agent"
	"log"
	"os"
)

func main() {
	orchestratorAddress := os.Getenv("ORCHESTRATOR_ADDRESS")
	if orchestratorAddress == "" {
		orchestratorAddress = "http://localhost:8080"
		log.Printf("ORCHESTRATOR_ADDRESS не установлен, используем значение по умолчанию: %s", orchestratorAddress)
	}

	numWorkers := 5

	agent := agent.NewAgent(numWorkers, orchestratorAddress)

	agent.Start()

	select {}
}
