# Распределенный Калькулятор

Этот проект реализует систему распределенного калькулятора. Она состоит из оркестратора и нескольких агентов, которые выполняют вычисления.

▌Архитектура

Система работает следующим образом:

1. Клиент отправляет запрос на вычисление в Оркестратор через REST API.
2. Оркестратор сохраняет задачу и присваивает ей уникальный ID.
3. Агенты опрашивают Оркестратор на предмет новых задач через внутренний API.
4. Оркестратор предоставляет ожидающие задачи доступным Агентам.
5. Агент выполняет вычисление и отправляет результат обратно в Оркестратор через внутренний API.
6. Оркестратор обновляет статус задачи и результат, делая его доступным для получения клиентом.
7. Клиент получает результат через REST API, используя ID задачи.

▌Компоненты

•  Оркестратор: Управляет задачами, назначает их агентам и хранит результаты.
•  Агент: Выполняет вычисления, полученные от оркестратора.

▌Необходимые условия

•  Go (версия 1.20 или новее)
•  Docker (опционально, для контейнерного развертывания)
•  Docker Compose (опционально, для контейнерного развертывания)
•  Make (опционально, для упрощения сборки и развертывания)

▌Инструкции по Сборке и Запуску

1. Клонируйте репозиторий:

  
```
bash
    git clone https://github.com/XBulien/distributed-calculator.git
    cd distributed-calculator
```

2. Соберите компоненты:

```
bash
  make build
```
 Эта команда соберет бинарные файлы оркестратора и агента.

3. Запустите оркестратор:

```
bash
   go run cmd/orchestrator/main.go
```

Это запустит оркестратор по адресу http://localhost:8080.

4. Запустите агентов:

```
bash
    go run .cmd/agent
```

 Это запустит агента, подключенного к оркестратору по адресу http://localhost:8080.
  Вы можете настроить адрес оркестратора, используя переменную окружения ORCHESTRATOR_ADDRESS:

▌Примеры Использования API

Оркестратор предоставляет REST API для взаимодействия с системой. Вот несколько примеров команд curl:

▌1. Отправьте запрос на вычисление


```
bash
curl -X POST -H "Content-Type: application/json" -d '{"expression": "2+2*2"}' http://localhost:8080/api/v1/calculate
```
Ответ:
```
json
{"id": "ваш-id-задачи"}
```

Замените ваш-id-задачи на фактический ID, возвращенный API.

▌2. Получитe cписок всех выражений

```
bash
curl http://localhost:8080/api/v1/expressions
```
Ответ:
```
json
{
 "expressions": [
  {
   "id": "expression-1",
   "expression": "2+2*2",
   "status": "Completed",
   "result": 6
  },
  {
   "id": "expression-2",
   "expression": "3+3",
   "status": "New",
   "result": 0
  }
 ]
}
```

▌3. Получите выражение по ID

```
bash
curl http://localhost:8080/api/v1/expressions/ваш-id-задачи
```

Ответ:
```
json
{
 "expression": [
  {
   "id": "ваш-id-задачи",
   "expression": "2+2*2",
   "status": "Completed",
   "result": 6
  }
 ]
}
```