package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/Mary-cross1296/go_final_project/tests"
)

func main() {
	// Определение порта
	port := os.Getenv("TODO_PORT")
	if port == "" {
		port = strconv.Itoa(tests.Port) // Порт по умолчанию
	}

	// Определение директории для файлов
	webDir := "../web"

	// Создание HTTP сервера
	http.Handle("/", http.FileServer(http.Dir(webDir)))

	// Запуск сервера на указанном порту
	log.Printf("Сервер запущен на порту %v\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
