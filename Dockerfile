FROM golang:1.21.5

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем все файлы проекта в рабочую директорию образа
COPY . .
RUN go mod download

# Определяем переменные окружения для компиляции и веб-сервера
ENV CGO_ENABLED=1 \
    GOOS=linux \
    TODO_PORT=7540 \
    TODO_DBFILE="/app/backend/scheduler.db" \
    TODO_PASSWORD=finalgo \
    TODO_WEB_DIR="/app/web"

# Компилируем программу
RUN go build -o /app/backend/main ./backend/main.go

# Устанавливаем рабочую директорию
WORKDIR /app/backend

# Запускаем сервер
CMD ["/app/backend/main"]