# Test Assignment: Load Balancer

## Описание

Сервис для распределения нагрузки на пул бэкенд-серверов. Использующий алгоритмы round-robin и tokens bucket. 

## Запуск проекта

1. Клонировать репозиторий:
   ```bash
   git clone https://github.com/YellowKek/cloud-test-task
   cd cloud-test-task
2. Начать сборку
    ```bash
   docker compose up --build

## API endpoints
1. POST /rate-limits
    ```json
    {
        "client_id": "back1:8080",
        "capacity": 20,
        "refill_interval": "6s"
    }
2. PUT /rate-limits
    ```json
    {
        "client_id": "back1:8080",
        "capacity": 20,
        "refill_interval": "6s"
    }
3. DELETE /rate-limits
    ```json
    {
        "client_id": "back1:8080"
    }
4. GET /rate-limits вернет конфиги для всех бакетов
5. GET /rate-limits/{clientId} выводит конфиг бакета по id

## Конфигурация (balancer/config/config.yaml)
   ```yaml
   port: порт на котором будет работать приложение
   backends:
   - массив доступных серверов
     db: uri для подключения к бд
     rate_limiting:
        capacity: стандартная емкость бакетов
        refill_interval: стандартный интервал обновления бакетов