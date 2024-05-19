package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func HttpServer(port, wd string) *http.Server {
	// Создание объект сервера
	httpServer := http.Server{
		Addr: ":" + port, // Установка адреса сервера
	}

	// Определение обработчика для корневого пути
	requestHandler := http.FileServer(http.Dir(wd))

	// Настройка сервера
	httpServer.Handler = requestHandler

	// Запуск сервера на указанном порту
	log.Printf("Сервер запущен на порту %v\n", port)
	go func() {
		err := httpServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatal("Http server error \n", err)
		}
	}()
	return &httpServer
}

func main() {
	// Определение порта
	port := os.Getenv("TODO_PORT")
	defaultPort := 7540 // Порт по умолчанию
	if port == "" {
		port = strconv.Itoa(defaultPort)
	}

	// Определение директории для файлов
	webDir := "../web"

	// Запуск сервера
	server := HttpServer(port, webDir)

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
