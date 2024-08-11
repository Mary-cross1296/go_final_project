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

func ChekingDataBase(tableName string) error {
	//appPath, err := os.Executable()
	appPath, err := os.Getwd()
	//log.Printf("ChekingDataBase() appPath %v", appPath)
	//log.Printf("ChekingDataBase() appPath1 %v", appPath1)
	if err != nil {
		log.Fatal(err)
	}

	// Определение пути к файлу БД. Создаем две переменные:
	// в первой dbFile путь из переменной окружения
	// во второй dbFileDefualt путь к базе данных по умолчанию
	//dbFile := os.Getenv("TODO_DBFILE")
	dbFile := config.DBPathConfig
	//log.Printf("Отладка 1 TODO_DBFILE %v", os.Getenv("TODO_DBFILE"))
	dbFileDefualt := filepath.Join(filepath.Dir(appPath), tableName)
	//log.Printf("Отладка 2 dbFileDefualt %v", dbFileDefualt)

	if dbFile == "" {
		dbFile = dbFileDefualt
		//log.Printf("Отладка 3 dbFile %v", dbFile)
		_, err = os.Stat(dbFile)
		if err != nil {
			log.Printf("Database file information missing: %s \nA new database file will be created... \n", err)
			// Функция, которая открывает БД
			db, err := OpenDataBase(tableName)
			defer db.Close()
			if err != nil {
				log.Printf("%s", err)
				return err
			}
			// Функция, которая создает таблицу и индекс
			err = CreateTableWithIndex(db)
			if err != nil {
				log.Printf("%s", err)
				return err
			}
		} else {
			log.Println("The database file already exists")
		}
	} else {
		_, err = os.Stat(dbFile)
		if err != nil {
			log.Printf("Database file information missing: %s \nA new database file will be created... \n", err)
			// Функция, которая открывает БД
			db, err := OpenDataBase(tableName)
			defer db.Close()
			if err != nil {
				log.Printf("%s", err)
				return err
			}
			// Функция, которая создает таблицу и индекс
			err = CreateTableWithIndex(db)
			if err != nil {
				log.Printf("%s", err)
				return err
			}
		}
		log.Println("The database file already exists")
	}
	return nil
}
