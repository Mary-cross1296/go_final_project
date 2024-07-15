# Шаг 1: Используем официальный образ Golang для компиляции и тестирования приложения
FROM golang:latest AS builder

# Устанавливаем рабочую директорию для сборки
WORKDIR /app

# Копируем go.mod и go.sum для установки зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем все файлы проекта в рабочую директорию образа
COPY . .

# Собираем программу для Linux
RUN CGO_ENABLED=0 GOOS=linux go build -o backend/main ./backend/main.go

# Шаг 2: Используем образ Golang для конечного образа (вместо Alpine)
FROM golang:latest

# Устанавливаем необходимые пакеты
RUN apt-get update && apt-get install -y ca-certificates curl

# Создаем рабочую директорию в конечном образе
WORKDIR /app

# Копируем скомпилированный бинарный файл и поддиректорию web из builder-образа
COPY --from=builder /app /app

# Определяем переменные окружения для веб-сервера
ENV TODO_PORT=7540 \
    TODO_DBFILE="/app/backend/scheduler.db" \
    TODO_PASSWORD=finalgo \
    TODO_WEB_DIR="/app/web"

# Открываем порт веб-сервера
EXPOSE 7540

# Копируем базу данных в контейнер
COPY backend/scheduler.db /app/backend/scheduler.db

# Запускаем сервер в фоновом режиме
CMD ["/bin/sh", "-c", "/app/backend/main & sleep 5 && go test ./tests -v"]
