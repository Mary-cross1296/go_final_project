package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

type Scheduler struct {
	ID      int
	Date    time.Duration
	Title   string
	Comment string
	Repeat  string
}

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

func ChekingDataBase() error {
	tableName := "scheduler.db"

	appPath, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	dbFile := filepath.Join(filepath.Dir(appPath), tableName)
	_, err = os.Stat(dbFile)

	var install bool
	if err != nil {
		install = true
	}

	if install == true {
		// если install равен true, после открытия БД требуется выполнить
		// sql-запрос с CREATE TABLE и CREATE INDEX
		db, err := sql.Open("sqlite", "scheduler.db")
		if err != nil {
			fmt.Printf("Error opening database: %s", err)
			return fmt.Errorf(err.Error())
		}
		defer db.Close()
		CreateTableWithIndex(db, tableName)
	}
}

func CreateTableWithIndex(db *sql.DB, tableName string) error {
	createTableRequest := `
	CREATE TABLE scheduler (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		date DATE NOT NULL,
		title VARCHAR(128) NOT NULL DEFAULT "",
		comment VARCHAR(256) NOT NULL DEFAULT "",
		repeat VARCHAR(128) NOT NULL DEFAULT ""
	);
	`
	createIndexRequest := "CREATE INDEX index_date ON scheduler(date);"

	_, err := db.Exec(createTableRequest)
	if err != nil {
		fmt.Printf("Error creating table: %s", err)
		return err
	}

	_, err = db.Exec(createIndexRequest, tableName)
	if err != nil {
		fmt.Printf("Error creating index: %s", err)
		return err
	}
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
	ChekingDataBase()

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
