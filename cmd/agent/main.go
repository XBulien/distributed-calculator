package main

import (
	"distributed-calculator/internal/agent"
	"log"
	"os" // Import для доступа к переменным окружения
)

func main() {
	// Получаем адрес оркестратора из переменной окружения (или используем значение по умолчанию)
	orchestratorAddress := os.Getenv("ORCHESTRATOR_ADDRESS")
	if orchestratorAddress == "" {
		orchestratorAddress = "http://localhost:8080" // Значение по умолчанию
		log.Printf("ORCHESTRATOR_ADDRESS не установлен, используем значение по умолчанию: %s", orchestratorAddress)
	}

	// Количество рабочих горутин
	numWorkers := 5 // Можно вынести в переменную окружения, если нужно

	// Инициализация агента с количеством горутин и адресом оркестратора
	agent := agent.NewAgent(numWorkers, orchestratorAddress)

	// Запуск агентов (горутин)
	err := agent.Start()
	if err != nil {
		log.Fatalf("Failed to start agent: %v", err)
	}
}
