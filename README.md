# API Health Monitor

Backend-сервис на Go для управления HTTP-эндпоинтами, доступность которых планируется периодически проверять. Данные об отслеживаемых эндпоинтах и результаты проверок хранятся в PostgreSQL.

> **Текущий статус:** реализованы CRUD HTTP API для отслеживаемых эндпоинтов, PostgreSQL-репозиторий и начальная схема базы данных. Планировщик проверок и запись результатов проверок пока находятся в разработке.

## Технологии

- **Go 1.27.3** — основной язык разработки.
- **net/http** — HTTP-сервер и маршрутизация.
- **PostgreSQL 18** — хранение данных.
- **pgx/v5** — PostgreSQL-драйвер и пул соединений.
- **Docker Compose** — локальное окружение.
- **golang-migrate** — применение SQL-миграций через Docker Compose.
- **log/slog** — структурированное логирование.

## Структура проекта

```text
.
├── cmd/
│   └── api/
│       └── main.go                 # Точка входа, инициализация и запуск приложения
├── internal/
│   ├── config/
│   │   └── config.go               # Загрузка конфигурации из переменных окружения
│   ├── features/
│   │   └── endpoints/
│   │       ├── repository/
│   │       │   ├── model.go        # Модель Endpoint
│   │       │   ├── repository.go   # Запросы к PostgreSQL
│   │       │   └── repository_test.go
│   │       └── transport/
│   │           ├── dto.go          # DTO, валидация и JSON-хелперы
│   │           └── handlers.go     # HTTP-обработчики эндпоинтов
│   └── server/
│       └── router.go               # Регистрация HTTP-маршрутов
├── migrations/
│   ├── 000001_create_initial_schema.up.sql
│   └── 000001_create_initial_schema.down.sql
├── Dockerfile
├── docker-compose.yaml
├── .env.example
├── .gitignore
├── go.mod
└── README.md
```

## Функциональность API

Базовый путь: `/endpoints`.

| Метод | Путь | Описание | Успешный ответ |
|---|---|---|---|
| `POST` | `/endpoints` | Создать эндпоинт | `201 Created` |
| `GET` | `/endpoints` | Получить список эндпоинтов | `200 OK` |
| `GET` | `/endpoints/{id}` | Получить эндпоинт по ID | `200 OK` |
| `PATCH` | `/endpoints/{id}` | Частично обновить эндпоинт | `200 OK` |
| `DELETE` | `/endpoints/{id}` | Удалить эндпоинт | `204 No Content` |

### Создание эндпоинта

Пример запроса:

```http
POST /endpoints
Content-Type: application/json
```

```json
{
  "url": "https://example.com",
  "check_interval_seconds": 60
}
```

`url` обязателен и должен использовать схему `http` или `https`. `check_interval_seconds` необязателен: если поле не передано, PostgreSQL применит значение по умолчанию `60`. Если поле передано, значение должно быть положительным.

### Получение списка

```http
GET /endpoints
```

Можно отфильтровать результаты по активности:

```http
GET /endpoints?is_active=true
GET /endpoints?is_active=false
```

Без query-параметра возвращаются все эндпоинты.

### Частичное обновление

Пример запроса:

```http
PATCH /endpoints/1
Content-Type: application/json
```

```json
{
  "check_interval_seconds": 120,
  "is_active": false
}
```

Можно передать одно или оба поддерживаемых поля: `check_interval_seconds` и `is_active`. URL через этот метод не изменяется.

### Формат ошибок

Ошибки API возвращаются в JSON:

```json
{
  "error": "endpoint not found",
  "time": "2026-11-01T12:00:00Z"
}
```

Основные статусы: `400 Bad Request` — некорректный запрос, `404 Not Found` — эндпоинт не найден, `409 Conflict` — URL уже существует, `500 Internal Server Error` — внутренняя ошибка.

## Локальный запуск

### Требования

- Go версии `1.27.3`.
- Docker и Docker Compose.
- PostgreSQL (если запускаешь приложение без Docker Compose).

### Настройка окружения

Создай `.env` на основе `.env.example` и задай пароль PostgreSQL и строку подключения. Не добавляй `.env` в Git: файл уже исключён через `.gitignore`.

Для запуска приложения непосредственно на хосте `DATABASE_URL` должен указывать на `localhost`, например:

```env
DATABASE_URL=postgres://health_monitor:YOUR_PASSWORD@localhost:5432/health_monitor?sslmode=disable
```

Для сервисов внутри Docker Compose хост базы данных — `db`:

```env
POSTGRES_PASSWORD=YOUR_PASSWORD
DATABASE_URL=postgres://health_monitor:YOUR_PASSWORD@db:5432/health_monitor?sslmode=disable
```

Используй соответствующую строку подключения для выбранного способа запуска.

### Запуск PostgreSQL и миграций через Docker Compose

```bash
docker compose up -d db
docker compose run --rm migrate
```

После успешного применения миграций запусти API на хосте:

```bash
DATABASE_URL='postgres://health_monitor:YOUR_PASSWORD@localhost:5432/health_monitor?sslmode=disable' go run ./cmd/api
```

По умолчанию HTTP-сервер слушает `:8080`.


## Тестирование

Интеграционные тесты репозитория используют отдельную тестовую базу данных. Перед запуском создай отдельную БД, примени к ней миграции и укажи её URL в `TEST_DATABASE_URL`.

Пример:

```bash
TEST_DATABASE_URL='postgres://health_monitor:YOUR_PASSWORD@localhost:5432/health_monitor_test?sslmode=disable' go test ./...
```

Тесты пропускаются, если `TEST_DATABASE_URL` не задан. Тестовая база должна быть отдельной от основной.

Полезные команды:

```bash
gofmt -w ./cmd ./internal
go vet ./...
go test ./...
```

## Схема базы данных

- `monitored_endpoints` — отслеживаемые URL, интервал проверки, признак активности и временные метки.
- `check_results` — история результатов проверок: HTTP-статус, время ответа, время начала и завершения, сообщение об ошибке.

Для `check_results` поле `is_success` вычисляется PostgreSQL на основе HTTP-статуса `2xx`. Удаление эндпоинта каскадно удаляет связанные результаты проверок.

## Дальнейшие задачи

- Реализовать периодический запуск проверок активных эндпоинтов.
- Сохранять результаты проверок в `check_results`.
- Продумать обработку таймаутов и сетевых ошибок.
- Добавить тесты HTTP-обработчиков и graceful shutdown.
