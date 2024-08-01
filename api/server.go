package api

import (
	"log"
	"net/http"

	"github.com/Mary-cross1296/go_final_project/auth"
	"github.com/gorilla/mux"
)

// Функция для создания и запуска HTTP сервера
func HttpServer(port, wd string) *http.Server {
	// Создание роутера
	router := mux.NewRouter()

	// Обработчики запросов
	router.HandleFunc("/api/nextdate", NextDateHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/task", auth.Auth(AddTaskHandler)).Methods(http.MethodPost)
	router.HandleFunc("/api/task", auth.Auth(GetTaskForEdit)).Methods(http.MethodGet)
	router.HandleFunc("/api/task", auth.Auth(SaveEditTaskHandler)).Methods(http.MethodPut)
	router.HandleFunc("/api/task/done", auth.Auth(DoneTaskHandler)).Methods(http.MethodPost)
	router.HandleFunc("/api/task", auth.Auth(DeleteTaskHandler)).Methods(http.MethodDelete)
	router.HandleFunc("/api/tasks", auth.Auth(GetListUpcomingTasksHandler)).Methods(http.MethodGet)
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
