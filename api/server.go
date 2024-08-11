package api

import (
	"log"
	"net/http"

	"github.com/Mary-cross1296/go_final_project/auth"
	"github.com/Mary-cross1296/go_final_project/storage"
	"github.com/gorilla/mux"
)

// Структура для обработки запросов
type Handlers struct {
	DB *storage.DataBase
}

// Функция для создания и запуска HTTP сервера
func HttpServer(port, wd string, db *storage.DataBase) *http.Server {
	// Создание роутера
	router := mux.NewRouter()

	// Создание обработчиков
	handlers := &Handlers{DB: db}

	// Обработчики запросов
	router.HandleFunc("/api/nextdate", NextDateHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/task", auth.Auth(handlers.AddTaskHandler)).Methods(http.MethodPost)
	router.HandleFunc("/api/task", auth.Auth(handlers.GetTaskByIDHandler)).Methods(http.MethodGet)
	router.HandleFunc("/api/task", auth.Auth(handlers.SaveTaskHandler)).Methods(http.MethodPut)
	router.HandleFunc("/api/task/done", auth.Auth(handlers.DoneTaskHandler)).Methods(http.MethodPost)
	router.HandleFunc("/api/task", auth.Auth(handlers.DeleteTaskHandler)).Methods(http.MethodDelete)
	router.HandleFunc("/api/tasks", auth.Auth(handlers.GetTasksHandler)).Methods(http.MethodGet)
	router.HandleFunc("/api/signin", UserAuthorizationHandler).Methods(http.MethodPost)

	// Обработчик статических файлов
	StaticFileHandler(wd, router)

	// Создание объект сервера
	httpServer := http.Server{
		Addr:    ":" + port, // Установка адреса сервера
		Handler: router,     // Установка роутера в качестве обработчика
	}

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
