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
		log.Printf("Error loading .env file from path %s: %v", envPath, err)
	} else {
		log.Printf(".env file loaded successfully from path %s", envPath)
	}
}

// Инициализация переменных окружения
func Init() {
	PassConfig = os.Getenv("TODO_PASSWORD")
	PortConfig = os.Getenv("TODO_PORT")
	DBPathConfig = os.Getenv("TODO_DBFILE")
	WebDirPathConfig = os.Getenv("TODO_WEB_DIR")
}
