FROM golang:1.21.5

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем все файлы проекта в рабочую директорию образа
COPY . .
RUN go mod download

# Компилируем программу
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/backend/main ./backend/main.go

# Устанавливаем рабочую директорию
WORKDIR /app/backend

# Определяем переменные окружения для веб-сервера
ENV TODO_PORT=7540 \
    TODO_DBFILE="/app/backend/scheduler.db" \
    TODO_PASSWORD=finalgo \
    TODO_WEB_DIR="/app/web"

# Убедитесь, что файл базы данных существует и имеет правильные права доступа
RUN touch /app/backend/scheduler.db && chmod 666 /app/backend/scheduler.db

# Запускаем сервер
CMD ["/app/backend/main"]