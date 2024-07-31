package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

const jwtKey = "final_progect_go" // секретный ключ для подписи JWT

// Task представляет задачу
type Task struct {
	ID      string `json:"id"`
	Date    string `json:"date"`
	Title   string `json:"title"`
	Comment string `json:"comment"`
	Repeat  string `json:"repeat"`
}

// ErrorResponse представляет структуру ошибки
type ErrorResponse struct {
	Error string `json:"error"`
}

type Password struct {
	Password string `json:"password"`
}

type Token struct {
	Token string `json:"token"`
}

// Claims - структура для хранения данных токена
type Claims struct {
	jwt.StandardClaims
	PasswordHash string `json:"password_hash"`
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
		SendErrorResponse(w, ErrorResponse{Error: "NextDateHandler(): Invalid 'now' parameter format. Use YYYYMMDD"}, http.StatusBadRequest)
		return
	}

	// Вызываем функцию NextDate для получения следующей даты
	nextDate, err := NextDate(now, date, repeat)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: err.Error()}, http.StatusBadRequest)
		return
	}

	// Отправляем следующий ответ клиенту
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(nextDate))
}

// Обработчик статичных файлов
func StaticFileHandler(wd string, router *mux.Router) {
	fs := http.FileServer(http.Dir(wd))
	router.PathPrefix("/").Handler(http.StripPrefix("/", fs))
}

// Функция для отправки ошибочного ответа
func SendErrorResponse(w http.ResponseWriter, errResp ErrorResponse, statusCode int) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(statusCode)
	response, _ := json.Marshal(errResp)
	w.Write(response)
}

// Обрабатчик POST-запросов на добавление задачи
func AddTaskHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем метод запроса
	if r.Method != http.MethodPost {
		SendErrorResponse(w, ErrorResponse{Error: "AddTaskHandler(): Method not supported"}, http.StatusMethodNotAllowed)
		return
	}

	// Декодируем JSON-данные запроса в структуру Task
	var task Task
	err := json.NewDecoder(r.Body).Decode(&task)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "AddTaskHandler(): JSON deserialization error"}, http.StatusBadRequest)
		return
	}

	// Проверяем обязательное поле title
	if task.Title == "" {
		SendErrorResponse(w, ErrorResponse{Error: "AddTaskHandler(): Task title not specified"}, http.StatusBadRequest)
		return
	}

	// Если дата не указана
	if task.Date == "" {
		task.Date = time.Now().Format("20060102")
	}

	// Проверяем формат даты
	date, err := time.Parse("20060102", task.Date)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "AddTaskHandler(): Date is not in the correct format"}, http.StatusBadRequest)
		return
	}

	// Проверка формата поля Repeat
	fmt.Printf("Отладка task.Repeat %v \n", task.Repeat)
	if task.Repeat != "" {
		dateCheck, err := NextDate(time.Now(), task.Date, task.Repeat)
		fmt.Printf("Отладка dateCheck %v \n", dateCheck)
		fmt.Printf("Отладка err %v \n", err)
		if dateCheck == "" && err != nil {
			fmt.Printf("Отладка 66 err %v \n", err)
			SendErrorResponse(w, ErrorResponse{Error: "AddTaskHandler() Invalid repetition condition"}, http.StatusBadRequest)
			return
		}
	}

	now := time.Now()
	if date.Before(now) {

		if task.Repeat == "" || date.Truncate(24*time.Hour) == date.Truncate(24*time.Hour) {
			task.Date = time.Now().Format("20060102")
		} else {
			dateStr := date.Format("20060102")
			nextDate, err := NextDate(now, dateStr, task.Repeat)
			if err != nil {
				SendErrorResponse(w, ErrorResponse{Error: "AddTaskHandler(): Wrong repetition rule"}, http.StatusBadRequest)
				return
			}
			task.Date = nextDate
		}
	}

	// Выполняем запрос INSERT в базу данных
	tableName := "scheduler.db"
	db, _ := OpenDataBase(tableName)
	query := "INSERT INTO scheduler (date, title, comment, repeat) VALUES (?, ?, ?, ?)"

	res, err := db.Exec(query, task.Date, task.Title, task.Comment, task.Repeat)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "AddTaskHandler(): Error executing request"}, http.StatusInternalServerError)
		return
	}

	// Получаем ID добавленной задачи
	id, err := res.LastInsertId()
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "AddTaskHandler(): Error getting task ID"}, http.StatusInternalServerError)
		return
	}

	// Устанавливаем полученный id в качестве строки
	task.ID = fmt.Sprint(id)
	fmt.Printf("Отладка 666 ефыл %v \n", task)

	response := map[string]interface{}{"id": id}
	responseId, err := json.Marshal(response)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "AddTaskHandler(): JSON encoding error"}, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(responseId)
}

// Обрабатчик Get-запросов на получение списка ближайших задач
func GetListUpcomingTasksHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем метод запроса
	if r.Method != http.MethodGet {
		SendErrorResponse(w, ErrorResponse{Error: "GetListUpcomingTasksHandler(): Method not supported"}, http.StatusMethodNotAllowed)
		return
	}

	var tasks []Task
	var task Task
	var rows *sql.Rows
	var err error

	tableName := "scheduler.db"
	db, _ := OpenDataBase(tableName)
	defer db.Close()

	search := r.FormValue("search")
	// Проверяем, что параметр поиска не пустой
	if search != "" {
		var searchDate time.Time
		// Попробуем распознать дату в формате "ггггммдд"
		searchDate, err = time.Parse("20060102", search)
		if err != nil || searchDate.Year() == 1 {
			// Если не получилось, попробуем распознать дату в формате "дд.мм.гггг"
			searchDate, err = time.Parse("02.01.2006", search)
		}

		if err == nil {
			// Если удалось распознать дату, делаем запрос по дате
			searchDateStr := searchDate.Format("20060102")
			rows, err = db.Query("SELECT id, date, title, comment, repeat FROM scheduler WHERE date = ? ORDER BY date DESC LIMIT 50", searchDateStr)
			if err != nil {
				SendErrorResponse(w, ErrorResponse{Error: "GetListUpcomingTasksHandler(): Error executing database query"}, http.StatusInternalServerError)
				return
			}
			defer rows.Close()
		} else {
			// Если дата не распознана, делаем запрос по подстроке в title или comment
			rows, err = db.Query("SELECT id, date, title, comment, repeat FROM scheduler WHERE title LIKE ? OR comment LIKE ? ORDER BY date DESC LIMIT 50", "%"+search+"%", "%"+search+"%")
			if err != nil {
				SendErrorResponse(w, ErrorResponse{Error: "GetListUpcomingTasksHandler(): Error executing database query"}, http.StatusInternalServerError)
				return
			}
			defer rows.Close()
		}
	} else {
		// Делаем запрос по поиску всех задач с сортировкой по дате
		rows, err = db.Query("SELECT id, date, title, comment, repeat FROM scheduler ORDER BY date DESC LIMIT 50")
		if err != nil {
			SendErrorResponse(w, ErrorResponse{Error: "GetListUpcomingTasksHandler(): Error executing database query"}, http.StatusInternalServerError)
			return
		}
		defer rows.Close()
	}

	for rows.Next() {
		var id int64
		if err := rows.Scan(&id, &task.Date, &task.Title, &task.Comment, &task.Repeat); err != nil {
			SendErrorResponse(w, ErrorResponse{Error: "GetListUpcomingTasksHandler():  Error scanning information received from the database"}, http.StatusInternalServerError)
			return
		}
		task.ID = fmt.Sprint(id)
		tasks = append(tasks, task)
	}

	// Если список задач пустой, возвращаем пустой массив
	if len(tasks) == 0 {
		tasks = []Task{} // или просто nil, но не пустой массив
	}

	// Формируем ответ в формате JSON объекта с ключом "tasks"
	responseMap := map[string][]Task{"tasks": tasks}
	response, err := json.Marshal(responseMap)
	//fmt.Printf("GetListUpcomingTasksHandler - response: %v\n", string(response))
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "GetListUpcomingTasksHandler(): JSON generation error"}, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

// Обработчик Get-запросов на получение задачи по id для редактирования
func GetTaskForEdit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendErrorResponse(w, ErrorResponse{Error: "GetTaskForEdit(): Method not supported"}, http.StatusBadRequest)
		return
	}

	idParam := r.FormValue("id")

	if idParam == "" {
		SendErrorResponse(w, ErrorResponse{Error: "GetTaskForEdit(): ID not specified"}, http.StatusBadRequest)
		return
	}

	tableName := "scheduler.db"
	db, _ := OpenDataBase(tableName)
	defer db.Close()

	var task Task
	var id int64

	err := db.QueryRow("SELECT id, date, title, comment, repeat FROM scheduler WHERE id = ?", idParam).Scan(&id, &task.Date, &task.Title, &task.Comment, &task.Repeat)
	if err == sql.ErrNoRows {
		SendErrorResponse(w, ErrorResponse{Error: "GetTaskForEdit(): Task not found"}, http.StatusNotFound)
		return
	} else if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "GetTaskForEdit(): Error retrieving task data"}, http.StatusInternalServerError)
		return
	}

	task.ID = fmt.Sprint(id)

	// Формируем ответ в формате JSON объекта с ключом "tasks"
	response, err := json.Marshal(task)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "GetTaskForEdit(): JSON generation error"}, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	responseStr := fmt.Sprintf(string(response))
	fmt.Printf("Отладка %v", responseStr)
	w.Write(response)
}

// Обработчик Put-запросов на сохранение изменений задачи
func SaveEditTaskHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем метод запроса
	if r.Method != http.MethodPut {
		SendErrorResponse(w, ErrorResponse{Error: "SaveEditTaskHandler(): Method not supported"}, http.StatusMethodNotAllowed)
		return
	}

	// Декодируем JSON-данные запроса в структуру Task
	var task Task
	err := json.NewDecoder(r.Body).Decode(&task)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "SaveEditTaskHandler(): JSON deserialization error"}, http.StatusBadRequest)
		return
	}

	// Проверка на наличие идентификатора задачи
	if task.ID == "" {
		SendErrorResponse(w, ErrorResponse{Error: "SaveEditTaskHandler(): Task ID not specified"}, http.StatusBadRequest)
		return
	}

	// Проверка корректности идентификатора задачи
	id, err := strconv.Atoi(task.ID)
	if err != nil || id <= 0 {
		SendErrorResponse(w, ErrorResponse{Error: "SaveEditTaskHandler(): Invalid task ID"}, http.StatusBadRequest)
		return
	}

	// Проверяем обязательное поле title
	if task.Title == "" {
		SendErrorResponse(w, ErrorResponse{Error: "SaveEditTaskHandler(): Task title not specified"}, http.StatusBadRequest)
		return
	}

	// Если дата не указана
	if task.Date == "" {
		task.Date = time.Now().Format("20060102")
	}

	// Проверяем формат даты
	_, err = time.Parse("20060102", task.Date)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "SaveEditTaskHandler(): Date is not in the correct format"}, http.StatusBadRequest)
		return
	}

	// Проверка формата поля Repeat
	if task.Repeat != "" {
		if _, err := strconv.Atoi(task.Repeat[2:]); err != nil || (task.Repeat[0] != 'd' && task.Repeat[0] != 'w' && task.Repeat[0] != 'm' && task.Repeat[0] != 'y') {
			SendErrorResponse(w, ErrorResponse{Error: "SaveEditTaskHandler(): Incorrect task repetition format"}, http.StatusBadRequest)
			return
		}
	}

	// Проверка существования задачи перед обновлением
	var existingID int
	tableName := "scheduler.db"
	db, _ := OpenDataBase(tableName)

	err = db.QueryRow("SELECT id FROM scheduler WHERE id = ?", task.ID).Scan(&existingID)
	if err == sql.ErrNoRows {
		SendErrorResponse(w, ErrorResponse{Error: "SaveEditTaskHandler(): Task not found"}, http.StatusNotFound)
		return
	} else if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "SaveEditTaskHandler(): Error checking task existence"}, http.StatusInternalServerError)
		return
	}

	// Выполняем запрос UPDATE в базу данных
	query := "UPDATE scheduler SET date = ?, title = ?, comment = ?, repeat =? WHERE id = ?"

	_, err = db.Exec(query, task.Date, task.Title, task.Comment, task.Repeat, task.ID)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "SaveEditTaskHandler(): Task not found"}, http.StatusInternalServerError)
		return
	}

	response, err := json.Marshal(struct{}{})
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "SaveEditTaskHandler(): JSON encoding error"}, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

// Обработчик POST-запросов для отметки выполненной задачи
func DoneTaskHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendErrorResponse(w, ErrorResponse{Error: "DoneTaskHandler(): method not supported"}, http.StatusBadRequest)
		return
	}

	idParam := r.FormValue("id")
	if idParam == "" {
		SendErrorResponse(w, ErrorResponse{Error: "DoneTaskHandler(): Task ID not specified"}, http.StatusBadRequest)
		return
	}
	fmt.Printf("Отладка 0 idParam %v \n", idParam)
	idParamNum, _ := strconv.Atoi(idParam)

	tableName := "scheduler.db"
	db, _ := OpenDataBase(tableName)
	defer db.Close()

	var task Task
	var id int64
	err := db.QueryRow("SELECT id, date, title, comment, repeat FROM scheduler WHERE id = ?", idParamNum).Scan(&id, &task.Date, &task.Title, &task.Comment, &task.Repeat)
	task.ID = fmt.Sprint(id)
	fmt.Printf("Отладка 1 task %v \n", task)
	if err == sql.ErrNoRows {
		SendErrorResponse(w, ErrorResponse{Error: "DoneTaskHandler(): Task not found"}, http.StatusNotFound)
		return
	} else if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "DoneTaskHandler(): Error retrieving task data"}, http.StatusInternalServerError)
		return
	}

	now := time.Now()
	if task.Repeat != "" {
		// Определяем следующую дату задачи
		newTaskDate, err := NextDate(now, task.Date, task.Repeat)
		if err != nil {
			SendErrorResponse(w, ErrorResponse{Error: "DoneTaskHandler(): Incorrect task repetition condition"}, http.StatusBadRequest)
			return
		}
		// Изменяем датузадачи на новую
		task.Date = newTaskDate

		// Выполняем запрос UPDATE в базу данных
		query := "UPDATE scheduler SET date = ?, title = ?, comment = ?, repeat =? WHERE id = ?"
		_, err = db.Exec(query, task.Date, &task.Title, &task.Comment, &task.Repeat, &task.ID)
		if err != nil {
			SendErrorResponse(w, ErrorResponse{Error: "DoneTaskHandler(): Task not found"}, http.StatusInternalServerError)
			return
		}
	} else {
		query := "DELETE FROM scheduler WHERE id = ?"
		task.ID = fmt.Sprint(id)
		result, err := db.Exec(query, task.ID)
		if err != nil {
			SendErrorResponse(w, ErrorResponse{Error: "DoneTaskHandler(): Error deleting task"}, http.StatusInternalServerError)
			return
		}

		// Проверка количества затронутых строк
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			SendErrorResponse(w, ErrorResponse{Error: "DoneTaskHandler(): Unable to determine the number of rows affected after deleting a task"}, http.StatusInternalServerError)
			return
		} else if rowsAffected == 0 {
			SendErrorResponse(w, ErrorResponse{Error: "DoneTaskHandler(): Task not found"}, http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{}`))
}

// Обработчик DELETE-запросов на удаление неактуальной задачи
func DeleteTaskHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		SendErrorResponse(w, ErrorResponse{Error: "DeleteTaskHandler(): method not supported"}, http.StatusBadRequest)
		return
	}

	idParam := r.FormValue("id")
	if idParam == "" {
		SendErrorResponse(w, ErrorResponse{Error: "DeleteTaskHandler(): Task ID not specified"}, http.StatusBadRequest)
		return
	}

	idParamNum, err := strconv.Atoi(idParam)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "DeleteTaskHandler(): Error converting idParam to number"}, http.StatusInternalServerError)
		return
	}

	fmt.Printf("Отладка Delete idParamNum %v \n", idParamNum)

	tableName := "scheduler.db"
	db, _ := OpenDataBase(tableName)
	defer db.Close()

	query := "DELETE FROM scheduler WHERE id = ?"
	result, err := db.Exec(query, idParamNum)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "DeleteTaskHandler(): Error deleting task"}, http.StatusInternalServerError)
		return
	}

	// Проверка количества затронутых строк
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "DeleteTaskHandler(): Unable to determine the number of rows affected after deleting a task"}, http.StatusInternalServerError)
		return
	} else if rowsAffected == 0 {
		SendErrorResponse(w, ErrorResponse{Error: "DeleteTaskHandler(): Task not found"}, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{}`))
}

// Обработчик POST-запросов для сверки введенного пароля с паролем в переменной окружения
func UserAuthorizationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendErrorResponse(w, ErrorResponse{Error: "DoneTaskHandler(): method not supported"}, http.StatusBadRequest)
		return
	}

	// Чтение тела запроса
	fmt.Printf("Отладка TODO_PASSWORD: %s\n", os.Getenv("TODO_PASSWORD"))
	body, err := io.ReadAll(r.Body)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "UserAuthorizationHandler(): Error reading request body"}, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var password Password
	err = json.Unmarshal(body, &password)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "UserAuthorizationHandler(): JSON deserialization error"}, http.StatusBadRequest)
		return
	}

	// Хеширование пароля введенного пользователем
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password.Password), bcrypt.DefaultCost)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "UserAuthorizationHandler(): Error hashing password"}, http.StatusInternalServerError)
		return
	}

	// Создание JWT токена с хешем пароля
	expirationTime := time.Now().Add(8 * time.Hour)
	claims := &jwt.StandardClaims{
		ExpiresAt: expirationTime.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, struct {
		PasswordHash string `json:"password_hash"`
		*jwt.StandardClaims
	}{
		string(hashedPassword),
		claims,
	})

	tokenString, err := token.SignedString([]byte(jwtKey))
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "UserAuthorizationHandler(): Token signing error"}, http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:    "token",
		Value:   tokenString,
		Expires: expirationTime,
	})

	w.Header().Set("Content-Type", "application/json")
	tokenResponse := Token{Token: tokenString}
	response, _ := json.Marshal(tokenResponse)
	w.Write(response)
}

// Функция сравнения паролей
func comparePasswords(currentPassword string, tokenPasswordHash string) bool {
	// Сравнение текущего пароля с хэшем пароля из токена
	err := bcrypt.CompareHashAndPassword([]byte(tokenPasswordHash), []byte(currentPassword))
	return err == nil
}

func auth(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pass := os.Getenv("TODO_PASSWORD")
		if len(pass) == 0 {
			// Если пароль не определен, возвращаем ошибку авторизации
			http.Error(w, "Authorization required", http.StatusUnauthorized)
			return
		}

		cookie, err := r.Cookie("token")
		if err != nil {
			http.Error(w, "Authorization required", http.StatusUnauthorized)
			return
		}

		tokenStr := cookie.Value

		claims := &struct {
			PasswordHash string `json:"password_hash"`
			*jwt.StandardClaims
		}{}

		// Попытка расшифровать и проверить токен
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			// Возвращаем секретный ключ для проверки подписи токена
			return []byte(jwtKey), nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Authorization required", http.StatusUnauthorized)
			return
		}

		// Дополнительная проверка на соответствие текущего пароля и пароля из токена
		if !comparePasswords(pass, claims.PasswordHash) {
			http.Error(w, "Authorization required", http.StatusUnauthorized)
			return
		}

		// Если все проверки прошли успешно, передаем запрос следующему обработчику
		next(w, r)
	})
}

func HttpServer(port, wd string) *http.Server {
	// Создание роутера
	router := mux.NewRouter()

	// Обработчики запросов
	router.HandleFunc("/api/nextdate", NextDateHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/task", auth(AddTaskHandler)).Methods(http.MethodPost)
	router.HandleFunc("/api/task", auth(GetTaskForEdit)).Methods(http.MethodGet)
	router.HandleFunc("/api/task", auth(SaveEditTaskHandler)).Methods(http.MethodPut)
	router.HandleFunc("/api/task/done", auth(DoneTaskHandler)).Methods(http.MethodPost)
	router.HandleFunc("/api/task", auth(DeleteTaskHandler)).Methods(http.MethodDelete)
	router.HandleFunc("/api/tasks", auth(GetListUpcomingTasksHandler)).Methods(http.MethodGet)
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

func OpenDataBase(tableName string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", tableName)
	if err != nil {
		log.Printf("Error opening database: %s\n", err)
		return nil, err
	}
	return db, nil
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
	log.Printf("Отладка 1 dbFile %v", dbFile)
	dbFileDefualt := filepath.Join(filepath.Dir(appPath), tableName)
	log.Printf("Отладка 2 dbFileDefualt %v", dbFileDefualt)

	if dbFile == "" {
		dbFile = dbFileDefualt
		log.Printf("Отладка 3 dbFile %v", dbFile)
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
			log.Println("1 The database file already exists")
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
		log.Println("2 The database file already exists")
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
	//fmt.Printf("Отладка daysWeekNum %v \n", daysWeekNum)

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

	return NowBeforeNextDate(now, nextDate, negativeNum, minNumDay, daysNum)
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

	return NowBeforeNextDate(now, nextDate, negativeNum, minNumDay, daysNum)
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

// Функция для получения нового токена
func getAndUpdateToken() error {
	// Определите URL и пароль для запроса
	url := "http://localhost:7540/api/signin"
	password := "finalgo"

	// Создание JSON тела запроса
	passwordData := Password{Password: password}
	body, err := json.Marshal(passwordData)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %v", err)
	}

	// Отправка POST запроса
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("error sending POST request: %v", err)
	}
	defer resp.Body.Close()

	// Проверка кода состояния
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-OK response status: %s", resp.Status)
	}

	// Чтение ответа
	var tokenResponse map[string]string
	err = json.NewDecoder(resp.Body).Decode(&tokenResponse)
	if err != nil {
		return fmt.Errorf("error decoding response body: %v", err)
	}

	// Извлечение токена
	token, ok := tokenResponse["token"]
	if !ok {
		return fmt.Errorf("token not found in response")
	}

	// Логирование полученного токена
	fmt.Printf("Отладка полученный token %s\n", token)

	// Обновление файла settings.go
	settingsFilePath := "../tests/settings.go"

	err = updateSettingsFile(settingsFilePath, token)
	if err != nil {
		return fmt.Errorf("error updating settings file: %v", err)
	}
	return nil
}

// Функция для обновления файла settings.go
func updateSettingsFile(filePath, token string) error {
	// Проверка, существует ли файл
	if !fileExists(filePath) {
		return fmt.Errorf("файл '%s' не существует", filePath)
	}

	// Открытие файла для чтения
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("ошибка открытия файла настроек: %v", err)
	}
	defer file.Close()

	// Чтение содержимого файла
	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("ошибка чтения файла настроек: %v", err)
	}

	// Логирование текущего содержимого файла
	fmt.Printf("Содержимое файла до обновления:\n%s\n", string(content))

	// Регулярное выражение для поиска строки токена
	re := regexp.MustCompile(`(?m)^var Token = ` + "`.*`")
	newTokenLine := fmt.Sprintf("var Token = `%s`", token)

	// Замена старого токена на новый
	newContent := re.ReplaceAllString(string(content), newTokenLine)

	// Открытие файла для записи
	file, err = os.OpenFile(filePath, os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("ошибка открытия файла настроек для записи: %v", err)
	}
	defer file.Close()

	// Запись обновленного содержимого обратно в файл
	_, err = file.WriteString(newContent)
	if err != nil {
		return fmt.Errorf("ошибка записи в файл настроек: %v", err)
	}

	// Логирование содержимого файла после обновления
	fmt.Printf("Содержимое файла после обновления:\n%s\n", newContent)

	return nil
}

// Функция для проверки существования файла
func fileExists(filepath string) bool {
	_, err := os.Stat(filepath)
	return !os.IsNotExist(err)
}

func main() {
	// Определение порта
	port := os.Getenv("TODO_PORT")
	defaultPort := 7540 // Порт по умолчанию
	if port == "" {
		port = strconv.Itoa(defaultPort)
	}

	// Определение директории для файлов
	webDir := os.Getenv("TODO_WEB_DIR")
	if webDir == "" {
		webDir = "../web" // Путь по умолчанию для локального запуска
	}

	// Запуск сервера
	server := HttpServer(port, webDir)
	ChekingDataBase()

	// Получение и обновление токена
	if err := getAndUpdateToken(); err != nil {
		fmt.Printf("Error updating token: %v\n", err)
	} else {
		fmt.Println("Token updated successfully")
	}

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
