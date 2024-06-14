package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"

	_ "modernc.org/sqlite"
)

type Scheduler struct {
	ID      int
	Date    time.Duration
	Title   string
	Comment string
	Repeat  string
}

// Task представляет задачу
type Task struct {
	Date    string `json:"date"`
	Title   string `json:"title"`
	Comment string `json:"comment"`
	Repeat  string `json:"repeat"`
}

// ErrorResponse представляет структуру ошибки
type ErrorResponse struct {
	Error string `json:"error"`
}

// SuccessResponse представляет успешный ответ с ID
type SuccessResponse struct {
	ID int64 `json:"id"`
}

// Обработчик запросов на /api/nextdate.
func NextDateHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем Get-параметры из запроса
	nowTime := r.FormValue("now")
	date := r.FormValue("date")
	repeat := r.FormValue("repeat")

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

	// Отправляем следующий ответ клиенту
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(nextDate))
}

func StaticFileHandler(wd string) {
	staticHandler := http.FileServer(http.Dir(wd))
	http.Handle("/", staticHandler)
}

// addTaskHandler обрабатывает POST-запрос на добавление задачи
func AddTaskHandler(w http.ResponseWriter, r *http.Request) {
	// Функция для отправки ошибочного ответа
	errorResponse := func(w http.ResponseWriter, errResp ErrorResponse, statusCode int) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(statusCode)
		response, _ := json.Marshal(errResp)
		w.Write(response)
	}

	// Функция для отправки успешного ответа
	successResponse := func(w http.ResponseWriter, successResp SuccessResponse) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		response, _ := json.Marshal(successResp)
		w.Write(response)
	}

	// Проверяем метод запроса
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	// Декодируем JSON-данные запроса в структуру Task
	var task Task
	err := json.NewDecoder(r.Body).Decode(&task)
	if err != nil {
		errorResponse(w, ErrorResponse{Error: "Ошибка десериализации JSON"}, http.StatusBadRequest)
		return
	}

	// Проверяем обязательное поле title
	if task.Title == "" {
		errorResponse(w, ErrorResponse{Error: "Не указан заголовок задачи"}, http.StatusBadRequest)
		return
	}

	// Если дата не указана
	if task.Date == "" {
		task.Date = time.Now().Format("20060102")
	}

	// Проверяем формат даты
	_, err = time.Parse("20060102", task.Date)
	if err != nil {
		errorResponse(w, ErrorResponse{Error: "Дата представлена в неправильном формате"}, http.StatusBadRequest)
		return
	}

	now := time.Now()

	if (task.Date == now.Format("20060102") && task.Repeat == "") || task.Repeat == "" {
		task.Date = now.Format("20060102")
		fmt.Printf("Отладка 1 task.Date %v \n", task.Date)
	} else {
		nextDate, err := NextDate(now, task.Date, task.Repeat)
		if err != nil {
			errorResponse(w, ErrorResponse{Error: "Неправильное правило повторения"}, http.StatusBadRequest)
			return
		}
		task.Date = nextDate
	}

	// Выполняем запрос INSERT в базу данных
	tableName := "scheduler.db"
	db, _ := OpenDataBase(tableName)
	query := "INSERT INTO scheduler (date, title, comment, repeat) VALUES (?, ?, ?, ?)"
	res, err := db.Exec(query, task.Date, task.Title, task.Comment, task.Repeat)
	if err != nil {
		errorResponse(w, ErrorResponse{Error: "Ошибка при выполнении запроса"}, http.StatusInternalServerError)
		return
	}

	// Получаем ID добавленной задачи
	id, err := res.LastInsertId()
	if err != nil {
		errorResponse(w, ErrorResponse{Error: "Ошибка при получении ID задачи"}, http.StatusInternalServerError)
		return
	}

	// Отправляем успешный ответ с ID добавленной задачи
	successResponse(w, SuccessResponse{ID: id})
}

func HttpServer(port, wd string) *http.Server {
	// Создание роутера
	router := mux.NewRouter()

	// Обработчик статических файлов
	StaticFileHandler(wd)

	// Обработчики запросов
	router.HandleFunc("/api/nextdate", NextDateHandler).Methods("GET") // Обработчики запросов следующей даты
	router.HandleFunc("/api/task", AddTaskHandler).Methods("POST")     // Обработчик запросов задачи

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

// Функция поиска предварительной nextDate
func PreliminaryNextDate(now time.Time, startDate time.Time) (time.Time, error) {
	nextDate := startDate
	if now == startDate || now.After(startDate) { // Now в будущем относительно startDate
		nextDate = now.AddDate(0, 0, 1)
		return nextDate, nil
	} else if now.Before(startDate) { // Now в прошлом относительно startDate
		nextDate = startDate.AddDate(0, 0, 1)
		return nextDate, nil
	}
	return time.Time{}, fmt.Errorf("TentativeNextDate() could not predetermine the next date")
}

func NowBeforeNextDate(now time.Time, nextDate time.Time, negativeNum int, minNumDay int, daysNum []int) (string, error) {
	// Расчет nextDate в функции PreliminaryNextDate, позволяет всегда выполняться условию now.Before(nextDate)
	for now.Before(nextDate) {
		for _, day := range daysNum {
			// Проверка даты для положительного числа из массива
			if day > 0 && negativeNum == 0 && day == nextDate.Day() {
				return nextDate.Format("20060102"), nil
			}

			if day > 0 && negativeNum == 1 && day == nextDate.Day() {
				negativeNumMin := slices.Min(daysNum)
				allegedNextDate := CalculatAllegedNextDate(nextDate, negativeNumMin)
				if nextDate.Day() <= minNumDay &&
					nextDate.Day() <= allegedNextDate.Day() {
					nextDate = time.Date(nextDate.Year(), nextDate.Month(), minNumDay, 0, 0, 0, 0, nextDate.Location())
					return nextDate.Format("20060102"), nil
				} else if nextDate.Day() >= minNumDay &&
					nextDate.Day() <= allegedNextDate.Day() {
					return allegedNextDate.Format("20060102"), nil
				}
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
				if nextDate.Day() <= minNumDay &&
					nextDate.Day() <= allegedNextDate.Day() {
					nextDate = time.Date(nextDate.Year(), nextDate.Month(), minNumDay, 0, 0, 0, 0, nextDate.Location())
					return nextDate.Format("20060102"), nil
				} else if nextDate.Day() >= minNumDay &&
					nextDate.Day() <= allegedNextDate.Day() {
					return allegedNextDate.Format("20060102"), nil
				}
			}

			// Если день из массива отрицательное число (при этом в массиве два отрицательных числа),
			// то вычисляем следующую дату задачи следующим образом
			if day < 0 && negativeNum == 2 {
				day1 := daysNum[0]
				day2 := daysNum[1]

				allegedNextDate1 := CalculatAllegedNextDate(nextDate, day1)
				allegedNextDate2 := CalculatAllegedNextDate(nextDate, day2)

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

func CalculatAllegedNextDate(nextDate time.Time, day int) time.Time {
	// Устанавливаем предполагаемую следующую дату задания на 1 число текущего месяца
	allegedNextDate := time.Date(nextDate.Year(), nextDate.Month(), 1, 0, 0, 0, 0, nextDate.Location())
	// Переносим на первое число следующего месяца
	allegedNextDate = allegedNextDate.AddDate(0, 1, 0)
	// Из полученной даты вычетаем указанное кол-во дней
	// Получаем предполагаемую дату следующей задачи
	allegedNextDate = allegedNextDate.AddDate(0, 0, day)
	return allegedNextDate
}

func CreateTableWithIndex(db *sql.DB) error {
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
		err = errors.New("invalid number of days")
		return "", err
	}

	var nextDate time.Time
	if days == 1 && now.Format("20060102") == startDate.Format("20060102") {
		nextDate = now
		return nextDate.Format("20060102"), nil
	}

	nextDate = startDate
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
	} else if startDate.Before(now) {
		nextDate = startDate.AddDate(0, 0, 1)
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
	nextDate, _ := PreliminaryNextDate(now, startDate)

	// Считаем кол-во отрицательных чисел в массиве
	negativeNum := CountNegativeNumbers(daysNum)

	// Ищем мининимально число в массиве daysNum
	minNumDay := FindMinNum(daysNum, negativeNum)

	// Расчет nextDate в функции PreliminaryNextDate, позволяет всегда выполняться условию now.Before(nextDate)

	return NowBeforeNextDate(now, nextDate, negativeNum, minNumDay, daysNum)
	//for now.Before(nextDate) {
	//NowBeforeNextDate(now, nextDate, negativeNum, minNumDay, daysNum)
	//}
	//return "", nil
}

// Условие: перенос задачи на определенное число указанных месяцев
func CalculatMonthsTask(now time.Time, startDate time.Time, daysNum []int, monthsNum []int) (string, error) {
	nextDate, _ := PreliminaryNextDate(now, startDate)

	// Ищем подходящий месяц
	isLoop := true
	counter := 0

searchingMonths:
	for isLoop {
		for _, month := range monthsNum {
			if month == int(nextDate.Month()) && counter < 1 {
				isLoop = false
				break searchingMonths
			} else if month == int(nextDate.Month()) {
				nextDate = time.Date(nextDate.Year(), nextDate.Month(), 1, 0, 0, 0, 0, nextDate.Location())
				break searchingMonths
			}
		}
		nextDate = nextDate.AddDate(0, 1, 0)
		counter++
	}

	// Считаем кол-во отрицательных чисел в массиве
	negativeNum := CountNegativeNumbers(daysNum)

	// Ищем мининимально число в массиве daysNum
	minNumDay := FindMinNum(daysNum, negativeNum)

	// Расчет nextDate в функции PreliminaryNextDate, позволяет всегда выполняться условию now.Before(nextDate)
	//for now.Before(nextDate) {
	//return NowBeforeNextDate(now, nextDate, negativeNum, minNumDay, daysNum)
	//}
	return NowBeforeNextDate(now, nextDate, negativeNum, minNumDay, daysNum)
	//return "", nil
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
