package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"syscall"
)

func HttpServer(port, wd string, server *http.Server, sigs chan os.Signal) {
	//var httpServer *http.Server
	// Определение обработчика для корневого пути
	requestHandler := http.FileServer(http.Dir(wd))

	// Настройка сервера
	server.Handler = requestHandler

	// Запуск сервера на указанном порту
	log.Printf("Сервер запущен на порту %v\n", port)
	log.Fatal(server.ListenAndServe())
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

	// Создание экземпляра http.Server
	httpServer := &http.Server{
		Addr: ":" + port, // Установка адреса сервера
	}

	// Канал для сигналов
	sigs := make(chan os.Signal, 1)
	var sig os.Signal

	// Запуск сервера
	go HttpServer(port, webDir, httpServer, sigs)

	for {
		sig = <-sigs
		fmt.Println("signal:", sig)
		if sig == syscall.SIGINT || sig == syscall.SIGTERM {
			break
		}
	}
	httpServer.Close()
}
