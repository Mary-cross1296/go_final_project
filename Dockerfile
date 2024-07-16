# Используем базовый образ Ubuntu
FROM ubuntu:latest

# Устанавливаем необходимые пакеты
RUN apt-get update && apt-get install -y \
    golang-go \
    ca-certificates \
    curl \
    sqlite3

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем go.mod и go.sum для установки зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем все файлы проекта в рабочую директорию образа
COPY . .

# Копируем базу данных в контейнер
COPY backend/scheduler.db /app/backend/scheduler.db

# Компилируем программу
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/backend/main ./backend/main.go

# Определяем переменные окружения для веб-сервера
ENV TODO_PORT=7540 \
    TODO_DBFILE="/app/backend/scheduler.db" \
    TODO_PASSWORD=finalgo \
    TODO_WEB_DIR="/app/web"

# Открываем порт веб-сервера
EXPOSE 7540

# Запускаем сервер
CMD ["/bin/sh", "-c", "/app/backend/main"]