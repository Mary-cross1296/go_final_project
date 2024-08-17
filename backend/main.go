package main

import (
	"log"
	"net/http"
	"strconv"

	"github.com/Mary-cross1296/go_final_project/api"
	"github.com/Mary-cross1296/go_final_project/config"
	"github.com/Mary-cross1296/go_final_project/storage"
)

const TableName = "scheduler.db"

func main() {
	envPath := "../config/.env"

	// Загрузка переменных окружения
	config.LoadEnvVar(envPath)

	// Инициализация глобальных переменных со значениями перменных окружения
	config.Init()

	// Определение порта
	port := config.PortConfig
	defaultPort := 7540 // Порт по умолчанию
	if port == "" {
		port = strconv.Itoa(defaultPort)
	}

	// Определение директории для файлов
	webDir := config.WebDirPathConfig
	if webDir == "" {
		webDir = "../web" // Путь по умолчанию для локального запуска
	}

	// Проверка существования файла базы данных
	storage.ChekingDataBase(TableName)

	// Инициализация базы данных
	db, err := storage.OpenDataBase(TableName)
	if err != nil {
		log.Fatalf("Main(): Error opening database: %s\n", err)
	}
	defer db.Close()

	// Создание сервера
	server, err := api.HttpServer(port, webDir, db)
	if err != nil {
		log.Fatalf("Main(): Error starting server: %s\n", err)
	}

	// Запуск сервера и ожидание его завершения
	log.Printf("Server is running on port %v\n", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Main(): Error occurred while running server: %s\n", err)
	}
}
