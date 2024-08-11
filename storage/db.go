package storage

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"

	"github.com/Mary-cross1296/go_final_project/config"
	_ "github.com/mattn/go-sqlite3"
)

func OpenDataBase(tableName string) (*DataBase, error) {
	db, err := sql.Open("sqlite3", tableName)
	if err != nil {
		log.Printf("Error opening database: %s\n", err)
		return nil, err
	}
	return &DataBase{db}, nil
}

func CreateTableWithIndex(db *DataBase) error {
	createTableRequest := `
	CREATE TABLE scheduler (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		date VARCHAR(128) NOT NULL,
		title VARCHAR(128) NOT NULL DEFAULT "",
		comment VARCHAR(256) NOT NULL DEFAULT "",
		repeat VARCHAR(128) NOT NULL DEFAULT ""
	);
	`
	createIndexRequest := "CREATE INDEX index_date ON scheduler(date);"

	_, err := db.Exec(createTableRequest)
	if err != nil {
		log.Printf("Error creating table: %s\n", err)
		return err
	}

	_, err = db.Exec(createIndexRequest)
	if err != nil {
		log.Printf("Error creating index: %s\n", err)
		return err
	}
	return nil
}

// fileDoesNotExist проверяет, существует ли файл
func fileDoesNotExist(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

// ChekingDataBase проверяет существование файла базы данных.
// Если файла нет, он создает его и инициализирует базу данных.
func ChekingDataBase(tableName string) error {
	// Получаем текущую рабочую директорию
	appPath, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	// Определяем путь к базе данных
	dbFile := config.DBPathConfig
	dbFileDefault := filepath.Join(appPath, tableName)

	switch {
	// Если путь к базе данных не задан, используем путь по умолчанию
	case dbFile == "":
		dbFile = dbFileDefault
		fallthrough // Переход к следующему случаю для проверки существования файла

	// Если файл базы данных не существует, создаем его
	case fileDoesNotExist(dbFile):
		log.Printf("Database file not found: %s\nA new database file will be created...\n", dbFile)

		// Функция, которая открывает БД
		db, err := OpenDataBase(dbFile)
		if err != nil {
			log.Printf("Error opening database: %s", err)
			return err
		}
		defer db.Close()

		// Функция, которая создает таблицу и индекс
		if err := CreateTableWithIndex(db); err != nil {
			log.Printf("Error creating table and index: %s", err)
			return err
		}

		log.Println("New database file created successfully")

	// Если файл базы данных существует
	default:
		log.Println("The database file already exists")
	}
	return nil
}
