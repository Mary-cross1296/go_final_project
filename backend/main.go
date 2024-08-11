package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Mary-cross1296/go_final_project/api"
	"github.com/Mary-cross1296/go_final_project/config"
	"github.com/Mary-cross1296/go_final_project/storage"
	"github.com/Mary-cross1296/go_final_project/utils"
)

// Глобальная переменная для подключения к базе данных
var Db *sql.DB

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
	Db, err := storage.OpenDataBase(TableName)
	if err != nil {
		log.Fatalf("Main(): Error opening database: %s\n", err)
	}
	defer Db.Close()

	// Запуск сервера
	server := api.HttpServer(port, webDir, Db)

	// Получение и обновление токена
	if err := utils.GetAndUpdateToken(); err != nil {
		log.Printf("Error updating token: %v\n", err)
	} else {
		log.Println("Token updated successfully")
	}

	// Создание канала для поступления сигналов
	sigs := make(chan os.Signal, 1)
	var sig os.Signal
	for {
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		sig = <-sigs
		log.Println()
		log.Println("signal:", sig)
		if sig == syscall.SIGINT || sig == syscall.SIGTERM {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := server.Shutdown(ctx); err != nil {
				log.Printf("Error during stop: %v\n", err)
			}
			log.Printf("Server stopped correctly")
			break
		}
	}
}
