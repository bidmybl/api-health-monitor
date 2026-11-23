# API Health Monitor

Backend-сервис на Go для мониторинга доступности HTTP-эндпоинтов. Позволяет управлять списком отслеживаемых URL, настраивать интервалы проверок и хранить историю результатов в PostgreSQL.

> **Текущий статус:** реализованы HTTP CRUD API, PostgreSQL-репозиторий, миграции, конфигурация, структурированное логирование и graceful shutdown. Периодический запуск проверок и сохранение их результатов находятся в разработке.

## Технологии

- **Go** — основной язык разработки.
- **net/http** — HTTP-сервер, маршрутизация и обработчики.
- **PostgreSQL 18** — хранение данных.
- **pgx/v5** — PostgreSQL-драйвер и пул соединений.
- **Docker Compose** — локальное окружение.
- **golang-migrate** — управление SQL-миграциями.
- **log/slog** — структурированное логирование.
- **testing, httptest** — тестирование.

## Архитектура проекта

```text
.
├── cmd/
│   └── api/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── features/
│   │   └── endpoints/
│   │       ├── repository/
│   │       │   ├── model.go
│   │       │   ├── repository.go
│   │       │   └── repository_test.go
│   │       └── transport/
│   │           ├── dto.go
│   │           ├── handlers.go
│   │           └── handlers_test.go
│   └── server/
│       └── router.go
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

### Основные компоненты

- **Transport** — HTTP-обработчики, DTO, валидация входных данных и формирование JSON-ответов.
- **Repository** — взаимодействие с PostgreSQL через `pgxpool`.
- **Server** — регистрация HTTP-маршрутов.
- **Config** — загрузка конфигурации из переменных окружения.
- **Main** — инициализация зависимостей, запуск HTTP-сервера и graceful shutdown.

## HTTP API

Базовый путь: `/endpoints`.

| Метод | Путь | Описание | Успешный статус |
|---|---|---|---|
| POST | `/endpoints` | Создать эндпоинт | 201 Created |
| GET | `/endpoints` | Получить список | 200 OK |
| GET | `/endpoints/{id}` | Получить по ID | 200 OK |
| PATCH | `/endpoints/{id}` | Частично обновить | 200 OK |
| DELETE | `/endpoints/{id}` | Удалить | 204 No Content |

### Создание эндпоинта

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

`url` обязателен и должен использовать схему `http` или `https`.

`check_interval_seconds` необязателен. Если значение не передано, используется значение по умолчанию — 60 секунд. Переданное значение должно быть положительным.

### Получение списка

```http
GET /endpoints
```

Фильтрация по активности:

```http
GET /endpoints?is_active=true
GET /endpoints?is_active=false
```

Если параметр не передан, возвращаются все эндпоинты.

### Частичное обновление

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

Можно передать одно или оба поддерживаемых поля:

- `check_interval_seconds`
- `is_active`

URL через PATCH не изменяется.

### Формат ошибок

Ошибки возвращаются в JSON:

```json
{
  "error": "endpoint not found",
  "time": "2026-11-01T12:00:00Z"
}
```

Основные HTTP-статусы:

- `400 Bad Request` — некорректный запрос или данные.
- `404 Not Found` — эндпоинт не найден.
- `409 Conflict` — URL уже существует.
- `413 Content Too Large` — тело запроса превышает допустимый размер.
- `500 Internal Server Error` — внутренняя ошибка сервера.

## База данных

Используются две таблицы:

### monitored_endpoints

Хранит информацию об отслеживаемых эндпоинтах:

- уникальный URL;
- интервал проверки в секундах;
- признак активности;
- время создания и последнего обновления.

### check_results

Предназначена для хранения истории проверок:

- ссылка на отслеживаемый эндпоинт;
- HTTP-статус;
- время ответа;
- время начала и завершения проверки;
- сообщение об ошибке;
- вычисляемый признак успешности.

Поле `is_success` вычисляется PostgreSQL на основе HTTP-статуса `2xx`. При удалении эндпоинта связанные результаты удаляются каскадно.

## Локальный запуск

### Требования

- Go (версия указана в `go.mod`);
- Docker;
- Docker Compose.

### Настройка окружения

Создай `.env` на основе `.env.example`:

```bash
cp .env.example .env
```

Укажи пароль PostgreSQL и строку подключения:

```env
POSTGRES_PASSWORD=your_password
DATABASE_URL=postgres://health_monitor:your_password@db:5432/health_monitor?sslmode=disable
TEST_DATABASE_URL=
```

Не добавляй `.env` в Git.

### Запуск через Docker Compose

```bash
docker compose up -d --build
```

Compose запустит PostgreSQL, применит миграции и после их успешного завершения запустит API.

HTTP-сервер доступен на порту `8090`.

Остановка:

```bash
docker compose down
```

Для остановки с удалением данных PostgreSQL:

```bash
docker compose down -v
```

### Запуск API непосредственно на хосте

Можно запустить базу и миграции через Compose:

```bash
docker compose up -d db
docker compose run --rm migrate
```

Затем запустить приложение локально, используя адрес PostgreSQL с `localhost`:

```bash
DATABASE_URL='postgres://health_monitor:your_password@localhost:5432/health_monitor?sslmode=disable' go run ./cmd/api
```

## Тестирование

В проекте используются два вида тестов:

- **HTTP handler tests** — проверяют обработку запросов, валидацию, HTTP-статусы и ответы с помощью `httptest` и тестовой реализации репозитория.
- **Repository integration tests** — проверяют взаимодействие с настоящей PostgreSQL.

Для запуска тестов обработчиков достаточно:

```bash
go test ./internal/features/endpoints/transport -v
```

Интеграционные тесты репозитория требуют отдельной тестовой базы данных. Укажи её адрес в `TEST_DATABASE_URL`. Тесты пропускаются, если переменная не задана.

Тестовая база должна быть отдельной от основной. Перед запуском интеграционных тестов на ней необходимо применить SQL-миграции.

```bash
TEST_DATABASE_URL='postgres://health_monitor:your_password@localhost:5432/health_monitor_test?sslmode=disable' go test ./...
```

Полезные команды:

```bash
gofmt -w ./cmd ./internal
go vet ./...
go test ./...
go test -race ./...
```

## Дальнейшее развитие

- Реализовать HTTP Checker для проверки доступности эндпоинтов.
- Добавить обработку таймаутов и сетевых ошибок.
- Сохранять результаты проверок в `check_results`.
- Реализовать периодический запуск проверок активных эндпоинтов.
- Добавить worker pool для параллельного выполнения проверок.
- Реализовать API для получения истории проверок.
- Добавить CI для автоматического запуска тестов.
