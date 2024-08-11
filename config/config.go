package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

var (
	PassConfig       string
	PortConfig       string
	DBPathConfig     string
	WebDirPathConfig string
)

// Загрузка переменных окружения
func LoadEnvVar(envPath string) {
	err := godotenv.Load(envPath)
	if err != nil {
		log.Print("Error loading .env file")
	}
}

// Инициализация переменных окружения
func Init() {
	PassConfig = os.Getenv("TODO_PASSWORD")
	PortConfig = os.Getenv("TODO_PORT")
	DBPathConfig = os.Getenv("TODO_DBFILE")
	WebDirPathConfig = os.Getenv("TODO_WEB_DIR")
}