package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math"
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

func HttpServer(port, wd string, handlers map[string]http.HandlerFunc) *http.Server {
	// Создание объект сервера
	httpServer := http.Server{
		Addr: ":" + port, // Установка адреса сервера
	}

	// Определение обработчика для корневого пути
	mux := http.NewServeMux()
	requestHandler := http.FileServer(http.Dir(wd))
	mux.Handle("/", requestHandler)

	// Настройка сервера
	//httpServer.Handler = requestHandler

	// Добавление пользовательских обработчиков
	for path, handler := range handlers {
		mux.Handle(path, handler)
	}

	// Присваивание mux полю Handler сервера
	httpServer.Handler = mux

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

func CountNegativeNumbers(nums []int) int {
	counterNegativeNum := 0
	for _, num := range nums {
		if num < 0 {
			counterNegativeNum++
		}
	}
	return counterNegativeNum
}

func FindMinNum(nums []int, numNegative int) int {
	minNumDay := math.MaxInt64

	if numNegative <= 1 {
		for _, minDay := range nums {
			if minDay > 0 && minDay < minNumDay {
				minNumDay = minDay
			}
		}
	} else if numNegative > 1 {
		minNumDay = 0
		for _, minDay := range nums {
			if minDay < 0 && minDay < minNumDay {
				minNumDay = minDay
			}
		}
	}
	return minNumDay
}

func CalculatAllegedNextDate(nextDate time.Time, day int) time.Time {
	// Устанавливаем предполагаемую следующую дату задания на 1 число текущего месяца
	allegedNextDate := time.Date(nextDate.Year(), nextDate.Month(), 1, 0, 0, 0, 0, nextDate.Location())
	// Переносим на первое число следующего месяца
	allegedNextDate = allegedNextDate.AddDate(0, 1, 0)
	// Из полученной даты вычетаем указанное кол-во дней
	// Получаем предполагаемую дату следующей задачи
	allegedNextDate = allegedNextDate.AddDate(0, 0, day-1)
	return allegedNextDate
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

// Условие: задача повторяется каждый раз через заданное кол-во дней
func CalculatDailyTask(now time.Time, startDate time.Time, repeat string) (string, error) {
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
	// Рассматриваем вариант, когда дата начала(starDate) задачи находится в будущем относительно текущего времени(now)
	if now.Before(nextDate) || now == nextDate {
		nextDate = nextDate.AddDate(0, 0, days)
		return nextDate.Format("20060102"), nil
	}

	// Рассматриваем вариант, когда дата начала задачи(starDate) находится в прошлом относительно текущего времени(now)
	for now.After(nextDate) || now == nextDate {
		nextDate = nextDate.AddDate(0, 0, days)
	}

	return nextDate.Format("20060102"), nil
}

// Условие: перенос задачи на год вперед
func CalculatYearlyTask(now time.Time, startDate time.Time) (string, error) {
	nextDate := startDate.AddDate(1, 0, 0)
	for now.After(nextDate) {
		nextDate = nextDate.AddDate(1, 0, 0)
	}
	return nextDate.Format("20060102"), nil
}

// Условие: перенос задачи на один из указанных дней недели
func CalculatWeeklyTask(now time.Time, startDate time.Time, repeat string) (string, error) {
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
	fmt.Printf("Отладка daysWeekNum %v \n", daysWeekNum)

	nextDate := startDate                         // Если now находится в прошлом относительно startDate
	if now == startDate || now.After(startDate) { // Если now равно startDate или если now в будущем относительно starDate
		nextDate = now.AddDate(0, 0, 1)
	}

	// Перебираем дни после найденного nextDate и сравниваем с числами из массива
	// Заводим счетчик, чтобы цикл не выполнялся бесконечно
	counter := 0
	for nextDate.After(now) {
		if counter > 14 {
			return "", errors.New("next date for task not found")
		}

		for _, dayWeek := range daysWeekNum {
			if dayWeek == int(nextDate.Weekday()) || (dayWeek == 7 && nextDate.Weekday() == 0) {
				return nextDate.Format("20060102"), nil
			}
		}
		nextDate = nextDate.AddDate(0, 0, 1)
		counter++
	}
	return "", nil
}

// Определение нужной функции для определения ежемесячных задач
func CalculatMonthlyTask(now time.Time, startDate time.Time, repeat string) (string, error) {
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

			if dayNum > -3 && dayNum < 32 {
				daysNum = append(daysNum, dayNum)
			} else {
				return "", fmt.Errorf("number greater than 32 or less than -3")
			}
		}

		return CalculatDayOfMonthTask(now, startDate, daysNum)
	}

	// Пока мы не знаем, какая перед нами комбинация
	// день + месяцы
	// дни + месяцы
	numList1 := strings.Split(daysMonth[0], ",")
	numList2 := strings.Split(daysMonth[1], ",")

	var daysNum []int
	for _, day := range numList1 {
		dayNum, err := strconv.Atoi(day)
		if err != nil {
			fmt.Printf("Error converting string to number: %s", err)
			return "", err
		}
		daysNum = append(daysNum, dayNum)
	}
	// Преобразуем единственный элемент первого массива в число
	// Получаем день задачи
	//dayNum, _ := strconv.Atoi(numList1[0])

	// Преобразуем элементы второго массива в числа
	// Создаем новый числовой массив
	var monthsNum []int
	for _, month := range numList2 {
		monthNum, err := strconv.Atoi(month)
		if err != nil {
			fmt.Printf("Error converting string to number: %s", err)
			return "", err
		}

		if monthNum < 1 && monthNum > 12 {
			return "", fmt.Errorf("month cannot be greater than 13 or a negative number")

		}
		monthsNum = append(monthsNum, monthNum)
	}

	return CalculatMonthsTask(now, startDate, daysNum, monthsNum)
}

// Условие: перенос задачи на заданный день месяца
func CalculatDayOfMonthTask(now time.Time, startDate time.Time, daysNum []int) (string, error) {
	// Считаем кол-во отрицательных чисел в массиве
	negativeNum := CountNegativeNumbers(daysNum)

	// Ищем мининимально число в массиве daysNum
	minNumDay := FindMinNum(daysNum, negativeNum)

	nextDate := startDate                         // Если now находится в прошлом относительно startDate
	if now == startDate || now.After(startDate) { // Если now равно startDate или если now в будущем относительно starDate
		nextDate = now.AddDate(0, 0, 1)
	}

	// Текущая(now) дата в прошлом относитительно даты старта(startDate)
	for now.Before(nextDate) {
		for _, day := range daysNum {
			// Если день из массива положительное число и равен дню проверяемой даты, то назначаем выполнение задачи
			if day > 0 && day == nextDate.Day() {
				return nextDate.Format("20060102"), nil
			}

			if len(daysNum) == 1 && negativeNum == 1 {
				allegedNextDate := CalculatAllegedNextDate(nextDate, day)

				if nextDate.Day() <= allegedNextDate.Day() {
					return allegedNextDate.Format("20060102"), nil
				}
			}

			// Если день из массива отрицательное число при этом в массиве одно отрицательное число,
			// то вычисляем следующую дату задачи
			if day < 0 && negativeNum == 1 {
				allegedNextDate := CalculatAllegedNextDate(nextDate, day)
				fmt.Printf("отладка allegedNextDate %v", allegedNextDate)

				if nextDate.Day() >= minNumDay &&
					nextDate.Day() <= allegedNextDate.Day() &&
					allegedNextDate.Day() <= minNumDay {
					return allegedNextDate.Format("20060102"), nil
				}
			}

			// Если день из массива отрицательное число (при этом в массиве два отрицательных числа),
			// то вычисляем следующую дату задачи следующим образом
			if day < 0 && negativeNum == 2 {
				day1 := daysNum[0]
				day2 := daysNum[1]

				allegedNextDate1 := CalculatAllegedNextDate(nextDate, day1)
				fmt.Printf("отладка allegedNextDate1 %v", allegedNextDate1)

				allegedNextDate2 := CalculatAllegedNextDate(nextDate, day2)
				fmt.Printf("отладка allegedNextDate2 %v", allegedNextDate2)

				// Вычисляем дату, которая происходит раньше
				if allegedNextDate1.Day() >= nextDate.Day() &&
					allegedNextDate2.Day() > nextDate.Day() {
					return allegedNextDate2.Format("20060102"), nil
				} else {
					return allegedNextDate1.Format("20060102"), nil
				}
			}
		}
		nextDate = nextDate.AddDate(0, 0, 1)
	}

	// Текущая дата(now) в будущем относительно старта(startDate)
	// Перебираем дни после текущей даты и сравниваем с числом из массива дней
	for nextDate.After(now) {
		for _, day := range daysNum {
			// Если день из массива положительное число и равен дню проверяемой даты, то назначаем выполнение задачи
			if day > 0 && day == nextDate.Day() {
				return nextDate.Format("20060102"), nil
			}

			// Если день из массива отрицательное число при этом в массиве одно отрицательное число,
			// то вычисляем следующую дату задачи
			if day < 0 && negativeNum == 1 {
				allegedNextDate := CalculatAllegedNextDate(nextDate, day)
				fmt.Printf("отладка allegedNextDate %v", allegedNextDate)

				if nextDate.Day() >= minNumDay &&
					nextDate.Day() <= allegedNextDate.Day() &&
					allegedNextDate.Day() <= minNumDay {
					return allegedNextDate.Format("20060102"), nil
				}
			}

			// Если день из массива отрицательное число (при этом в массиве два отрицательных числа),
			// то вычисляем следующую дату задачи следующим образом
			if day < 0 && negativeNum == 2 {
				day1 := daysNum[0]
				day2 := daysNum[1]

				allegedNextDate1 := CalculatAllegedNextDate(nextDate, day1)
				fmt.Printf("отладка allegedNextDate1 %v", allegedNextDate1)

				allegedNextDate2 := CalculatAllegedNextDate(nextDate, day2)
				fmt.Printf("отладка allegedNextDate2 %v", allegedNextDate2)

				// Вычисляем дату, которая происходит раньше
				if allegedNextDate1.Day() >= nextDate.Day() &&
					allegedNextDate2.Day() > nextDate.Day() {
					return allegedNextDate2.Format("20060102"), nil
				} else {
					return allegedNextDate1.Format("20060102"), nil
				}
			}
		}
		nextDate = nextDate.AddDate(0, 0, 1)
	}
	return "", nil
}

// Условие: перенос задачи на определенное число указанных месяцев
func CalculatMonthsTask(now time.Time, startDate time.Time, daysNum []int, monthsNum []int) (string, error) {
	nextDate := startDate                         // Если now находится в прошлом относительно startDate
	if now == startDate || now.After(startDate) { // Если now равно startDate или если now в будущем относительно starDate
		nextDate = now.AddDate(0, 0, 1)
	}

	// Ищем подходящий месяц
	monthBool := true
	counter := 0

outerLoop:
	for monthBool {
		for _, month := range monthsNum {
			if month == int(nextDate.Month()) && counter < 1 {
				break outerLoop
			} else if month == int(nextDate.Month()) {
				nextDate = time.Date(nextDate.Year(), nextDate.Month(), 1, 0, 0, 0, 0, nextDate.Location())
				break outerLoop
			}
		}
		nextDate = nextDate.AddDate(0, 1, 0)
		counter++
	}

	// Считаем кол-во отрицательных чисел в массиве
	negativeNum := CountNegativeNumbers(daysNum)

	// Ищем мининимально число в массиве daysNum
	minNumDay := FindMinNum(daysNum, negativeNum)

	// Текущая(now) дата в прошлом относитительно даты старта(startDate)
	for now.Before(nextDate) {
		for _, day := range daysNum {
			// Если день из массива положительное число и равен дню проверяемой даты, то назначаем выполнение задачи
			if day > 0 && day == nextDate.Day() {
				return nextDate.Format("20060102"), nil
			}

			if len(daysNum) == 1 && negativeNum == 1 {
				allegedNextDate := CalculatAllegedNextDate(nextDate, day)

				if nextDate.Day() <= allegedNextDate.Day() {
					return allegedNextDate.Format("20060102"), nil
				}
			}

			// Если день из массива отрицательное число при этом в массиве одно отрицательное число,
			// то вычисляем следующую дату задачи
			if day < 0 && negativeNum == 1 {
				allegedNextDate := CalculatAllegedNextDate(nextDate, day)
				fmt.Printf("отладка allegedNextDate %v", allegedNextDate)

				if nextDate.Day() >= minNumDay &&
					nextDate.Day() <= allegedNextDate.Day() &&
					allegedNextDate.Day() <= minNumDay {
					return allegedNextDate.Format("20060102"), nil
				}
			}

			// Если день из массива отрицательное число (при этом в массиве два отрицательных числа),
			// то вычисляем следующую дату задачи следующим образом
			if day < 0 && negativeNum == 2 {
				day1 := daysNum[0]
				day2 := daysNum[1]

				allegedNextDate1 := CalculatAllegedNextDate(nextDate, day1)
				fmt.Printf("отладка allegedNextDate1 %v", allegedNextDate1)

				allegedNextDate2 := CalculatAllegedNextDate(nextDate, day2)
				fmt.Printf("отладка allegedNextDate2 %v", allegedNextDate2)

				// Вычисляем дату, которая происходит раньше
				if allegedNextDate1.Day() >= nextDate.Day() &&
					allegedNextDate2.Day() > nextDate.Day() {
					return allegedNextDate2.Format("20060102"), nil
				} else {
					return allegedNextDate1.Format("20060102"), nil
				}
			}
		}
		nextDate = nextDate.AddDate(0, 0, 1)
	}

	// Текущая дата(now) в будущем относительно старта(startDate)
	// Перебираем дни после текущей даты и сравниваем с числом из массива дней
	for nextDate.After(now) {
		for _, day := range daysNum {
			// Если день из массива положительное число и равен дню проверяемой даты, то назначаем выполнение задачи
			if day > 0 && day == nextDate.Day() {
				return nextDate.Format("20060102"), nil
			}

			// Если день из массива отрицательное число при этом в массиве одно отрицательное число,
			// то вычисляем следующую дату задачи
			if day < 0 && negativeNum == 1 {
				allegedNextDate := CalculatAllegedNextDate(nextDate, day)
				fmt.Printf("отладка allegedNextDate %v", allegedNextDate)

				if nextDate.Day() >= minNumDay &&
					nextDate.Day() <= allegedNextDate.Day() &&
					allegedNextDate.Day() <= minNumDay {
					return allegedNextDate.Format("20060102"), nil
				}
			}

			// Если день из массива отрицательное число (при этом в массиве два отрицательных числа),
			// то вычисляем следующую дату задачи следующим образом
			if day < 0 && negativeNum == 2 {
				day1 := daysNum[0]
				day2 := daysNum[1]

				allegedNextDate1 := CalculatAllegedNextDate(nextDate, day1)
				fmt.Printf("отладка allegedNextDate1 %v", allegedNextDate1)

				allegedNextDate2 := CalculatAllegedNextDate(nextDate, day2)
				fmt.Printf("отладка allegedNextDate2 %v", allegedNextDate2)

				// Вычисляем дату, которая происходит раньше
				if allegedNextDate1.Day() >= nextDate.Day() &&
					allegedNextDate2.Day() > nextDate.Day() {
					return allegedNextDate2.Format("20060102"), nil
				} else {
					return allegedNextDate1.Format("20060102"), nil
				}
			}
		}
		nextDate = nextDate.AddDate(0, 0, 1)
	}
	return "", nil
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
		return CalculatDailyTask(now, startDate, repeat) // Функция расчета даты для ежедневных задач
	case repeat == "y":
		return CalculatYearlyTask(now, startDate) // Функция расчета даты для ежегодных дел
	case strings.HasPrefix(repeat, "w "):
		return CalculatWeeklyTask(now, startDate, repeat) // Функция расчета даты для задач на определенные дни недели
	case strings.HasPrefix(repeat, "m "):
		return CalculatMonthlyTask(now, startDate, repeat) // Функция расчета даты для задач на определенные дни месяца
	case repeat == "":
		return "", errors.New("Repeat is empty")
	default:
		return "", errors.New("Repetition rule is not supported")
	}
}

// Обработчик запросов на /api/nextdate.
func NextDateHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем Get-параметры из запроса
	nowTime := r.FormValue("now")
	date := r.FormValue("date")
	repeat := r.FormValue("repeat")
	//nowTime := r.URL.Query().Get("now")
	//date := r.URL.Query().Get("date")
	//repeat := r.URL.Query().Get("repeat")

	// Преобразуем параметр "now" в формат time.Time
	now, err := time.Parse("20060102", nowTime)
	if err != nil {
		http.Error(w, "Invalid 'now' parameter format. Use YYYYMMDD", http.StatusBadRequest)
		return
	}

	// Вызываем функцию NextDate для получения следующей даты
	nextDate, err := NextDate(now, date, repeat)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Возвращаем следующую дату задачи
	fmt.Printf("Next date: %s \n", nextDate)

	// Отправляем следующий ответ клиенту
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(nextDate))

	// Отладочное сообщение
	log.Printf("Received request for next date. Now: %s, Date: %s, Repeat: %s. Next date: %s", nowTime, date, repeat, nextDate)
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

	// Мапинг обработчиков
	handlers := map[string]http.HandlerFunc{
		"/api/nextdate": NextDateHandler,
	}

	// Запуск сервера
	server := HttpServer(port, webDir, handlers)
	ChekingDataBase()

	// Устанавливаем обработчик для api/nextdate
	http.HandleFunc("/api/nextdate", NextDateHandler)

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
