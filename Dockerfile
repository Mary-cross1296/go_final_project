# Используем базовый образ Ubuntu
FROM golang:1.21.5

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем go.mod и go.sum для установки зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем все файлы проекта в рабочую директорию образа
COPY . .

# Компилируем программу
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/backend/main ./backend/main.go

# Устанавливаем рабочую директорию
WORKDIR /app/backend

# Определяем переменные окружения для веб-сервера
ENV TODO_PORT=7540 \
    TODO_DBFILE="/app/backend/scheduler.db" \
    TODO_PASSWORD=finalgo \
    TODO_WEB_DIR="/app/web"

# Запускаем сервер
CMD ["/app/backend/main"]