package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Mary-cross1296/go_final_project/api"
	"github.com/Mary-cross1296/go_final_project/storage"
	"github.com/Mary-cross1296/go_final_project/utils"
	"github.com/joho/godotenv"
)

func main() {
	//Загрузка переменных окружения
	err := godotenv.Load()
	if err != nil {
		log.Print("Error loading .env file")
	}

	passwordCorrect := os.Getenv("TODO_PASSWORD")
	fmt.Print(passwordCorrect)

	// Определение порта
	port := os.Getenv("TODO_PORT")
	defaultPort := 7540 // Порт по умолчанию
	if port == "" {
		port = strconv.Itoa(defaultPort)
	}

	// Определение директории для файлов
	webDir := os.Getenv("TODO_WEB_DIR")
	if webDir == "" {
		webDir = "../web" // Путь по умолчанию для локального запуска
	}

	log.Printf("Путь к базе данных: %s", os.Getenv("TODO_DBFILE"))

	// Запуск сервера
	server := api.HttpServer(port, webDir)
	storage.ChekingDataBase()

	// Получение и обновление токена
	if err := utils.GetAndUpdateToken(); err != nil {
		fmt.Printf("Error updating token: %v\n", err)
	} else {
		fmt.Println("Token updated successfully")
	}

	// Создание канала для поступления сигналов
	sigs := make(chan os.Signal, 1)
	var sig os.Signal
	for {
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		sig = <-sigs
		fmt.Println()
		fmt.Println("signal:", sig)
		if sig == syscall.SIGINT || sig == syscall.SIGTERM {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := server.Shutdown(ctx); err != nil {
				fmt.Printf("Error during stop: %v\n", err)
			}
			fmt.Printf("Server stopped correctly")
			break
		}
	}
}
