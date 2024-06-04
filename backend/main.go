package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "modernc.org/sqlite"
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

func OpenDataBase(tableName string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", tableName)
	if err != nil {
		fmt.Printf("Error opening database: %s\n", err)
		return nil, err
	}
	return db, nil
}

func ChekingDataBase() error {
	tableName := "scheduler.db"

	appPath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}

	// Определение пути к файлу БД. Создаем две переменные:
	// в первой dbFile путь из переменной окружения
	// во второй dbFileDefualt путь к базе данных по умолчанию
	dbFile := os.Getenv("TODO_DBFILE")
	dbFileDefualt := filepath.Join(filepath.Dir(appPath), tableName)

	if dbFile == "" {
		dbFile = dbFileDefualt
		_, err = os.Stat(dbFile)
		if err != nil {
			fmt.Printf("Database file information missing: %s \nA new database file will be created... \n", err)
			// Функция, которая открывает БД
			db, err := OpenDataBase(tableName)
			defer db.Close()
			if err != nil {
				fmt.Printf("%s", err)
				return err
			}
			// Функция, которая создает таблицу и индекс
			err = CreateTableWithIndex(db)
			if err != nil {
				fmt.Printf("%s", err)
				return err
			}
		} else {
			fmt.Println("The database file already exists")
		}
	}
	return nil
}

func CreateTableWithIndex(db *sql.DB) error {
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
		fmt.Printf("Error creating table: %s\n", err)
		return err
	}

	_, err = db.Exec(createIndexRequest)
	if err != nil {
		fmt.Printf("Error creating index: %s\n", err)
		return err
	}
	return nil
}

func NextDate(now time.Time, date string, repeat string) (string, error) {
	// Парсин стартового-исходного времени, когда задача была выполнена первый раз
	startDate, err := time.Parse("20060102", date)
	if err != nil {
		fmt.Printf("The start time cannot be converted to a valid date: %s", err)
		return "", err
	}

	switch {
	case strings.HasPrefix(repeat, "d "):

		return СalculatDailyTask(now, startDate, repeat) // Функция расчета даты для ежедневных задач
	case repeat == "y":
		return СalculatYearlyTask(now, startDate) // Функция расчета даты для ежегодных дел
	case strings.HasPrefix(repeat, "w "):
		return СalculatWeeklyTask(now, startDate, repeat) // Функция расчета даты для задач на определенные дни недели
	case strings.HasPrefix(repeat, "m "):
		return "", nil // Функция расчета даты для задач на определенные дни месяца
	case repeat == "":
		return "", errors.New("Repeat is empty")
	default:
		return "", errors.New("Repetition rule is not supported")
	}

}

func СalculatDailyTask(now time.Time, startDate time.Time, repeat string) (string, error) {
	days, err := strconv.Atoi(strings.TrimPrefix(repeat, "d "))
	if err != nil {
		fmt.Printf("Error converting string to number:%s \n", err)
		return "", err
	}

	if days <= 0 || days > 400 {
		err = errors.New("Invalid number of days")
		return "", err
	}

	nextDate := startDate
	for now.After(nextDate) || now == nextDate {
		nextDate = nextDate.AddDate(0, 0, days)
	}
	return nextDate.Format("20060102"), nil
}

func СalculatYearlyTask(now time.Time, startDate time.Time) (string, error) {
	nextDate := startDate.AddDate(1, 0, 0)
	for now.After(nextDate) {
		nextDate = nextDate.AddDate(1, 0, 0)
	}
	return nextDate.Format("20660102"), nil
}

func СalculatWeeklyTask(now time.Time, startDate time.Time, repeat string) (string, error) {
	// Удаляем префикс, чтобы осталась часть строки с числами
	daysStr := strings.TrimPrefix(repeat, "w ")

	// Пробуем разделить сроку по запятым, чтобы получить массив из строк
	days := strings.Split(daysStr, ",")

	// Создаем новый массив для целых чисел
	var daysWeekNum []int

	// В цикле перебираем ранее полученный строковый массив days
	for _, dayStr := range days {
		// Форматируем строки в целые числа и добавляем в новый числовой массив
		dayNum, err := strconv.Atoi(dayStr)
		if err != nil {
			fmt.Printf("Error converting string to number: %s", err)
			return "", err
		}
		if dayNum < 1 || dayNum > 7 {
			err = errors.New("invalid number of days of weeks")
			return "", err
		}
		daysWeekNum = append(daysWeekNum, dayNum)
	}

	// Находим следующий день после текущего now
	nextDate := startDate
	for now.After(nextDate) || now == nextDate {
		nextDate = nextDate.AddDate(0, 0, 1)
	}

	// Перебираем дни после найденного nextDate и сравниваем с числами из массива
	// Заводим счетчик, чтобы цикл не выполнялся бесконечно
	counter := 0
	for nextDate.After(now) {
		if counter >= 14 {
			return "", errors.New("Next date for task not found")
		}

		for _, dayWeek := range daysWeekNum {
			if dayWeek == int(nextDate.Weekday()) {
				return nextDate.Format("20060102"), nil
			}
		}
		nextDate = nextDate.AddDate(0, 0, 1)
		counter++
	}
}

func СalculatMonthlyTask(now time.Time, startDate time.Time, repeat string) (string, error) {
	daysMonthStr := strings.TrimPrefix(repeat, "m ")
	daysMonth := strings.Split(daysMonthStr, " ")

	// Длина полученного слайса = 1, говорит о том, что мы имеем дело только с днями месяца
	if len(daysMonth) == 1 {
		days := daysMonth[0]
		daysStr := strings.Split(days, ",")

		var daysNum []int
		for _, day := range daysStr {
			dayNum, err := strconv.Atoi(day)
			if err != nil {
				fmt.Printf("Error converting string to number: %s", err)
				return "", err
			}
			daysNum = append(daysNum, dayNum)
		}
		СalculatDayOfMonthTask(now, startDate, daysNum)
	}

	// Пока мы не знаем, какая перед нами комбинация
	// день + месяцы
	// дни + месяцы
	numList1 := strings.Split(daysMonth[0], ",")
	numList2 := strings.Split(daysMonth[1], ",")

	// День + месяцы
	if len(numList1) == 1 {
		// Преобразуем единственный элемент первого массива в число
		// Получаем день задачи
		dayNum, _ := strconv.Atoi(numList1[0])

		// Преобразуем элементы второго массива в числа
		// Создаем новый числовой массив
		var monthsNum []int
		for _, month := range numList2 {
			monthNum, err := strconv.Atoi(month)
			if err != nil {
				fmt.Printf("Error converting string to number: %s", err)
				return "", err
			}
			monthsNum = append(monthsNum, monthNum)
		}
		СalculatDayOfMonthsTask(now, startDate, dayNum, monthsNum)
	}

	return "", nil
}

func СalculatDayOfMonthTask(now time.Time, startDate time.Time, daysNum []int) (string, error) {
	nowTime := now.Format("20060102")
	nowDate, _ := time.Parse("20060102", nowTime)

	// Находим следующий день после текущей даты now
	nextDate := nowDate.AddDate(0, 0, 1)

	// Перебираем дни после текущей даты и сравниваем с числом из массива дней
	for nextDate.After(now) {
		for _, day := range daysNum {
			// Если день из массива равен дню проверяемой даты, то назначаем выполнение задачи
			if day == nextDate.Day() {
				return nextDate.Format("20060102"), nil
			}
			nextDate = nextDate.AddDate(0, 0, 1)
		}
	}
	return "", nil
}

func СalculatDayOfMonthsTask(now time.Time, startDate time.Time, dayNum int, monthsNum []int) (string, error) {
	return "", nil
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
