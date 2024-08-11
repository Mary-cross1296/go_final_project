package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Mary-cross1296/go_final_project/auth"
	"github.com/Mary-cross1296/go_final_project/dateCalc"
	"github.com/Mary-cross1296/go_final_project/storage"
	"github.com/golang-jwt/jwt"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
)

// Лимит задач выводимых при запросе
const MaxTasksLimit = 50

// Обработчик запросов на /api/nextdate.
func NextDateHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем Get-параметры из запроса
	nowTime := r.FormValue("now")
	date := r.FormValue("date")
	repeat := r.FormValue("repeat")

	// Преобразуем параметр "now" в формат time.Time
	now, err := time.Parse(dateCalc.DateTemplate, nowTime)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "NextDateHandler(): Invalid 'now' parameter format. Use YYYYMMDD"}, http.StatusBadRequest)
		return
	}

	// Вызываем функцию NextDate для получения следующей даты
	nextDate, err := dateCalc.NextDate(now, date, repeat)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: err.Error()}, http.StatusBadRequest)
		return
	}

	// Отправляем следующий ответ клиенту
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(nextDate))
}

// Обработчик статичных файлов
func StaticFileHandler(wd string, router *mux.Router) {
	fs := http.FileServer(http.Dir(wd))
	router.PathPrefix("/").Handler(http.StripPrefix("/", fs))
}

// Обрабатчик POST-запросов на добавление задачи
func (h *Handlers) AddTaskHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем метод запроса
	if r.Method != http.MethodPost {
		SendErrorResponse(w, ErrorResponse{Error: "AddTaskHandler(): Method not supported"}, http.StatusMethodNotAllowed)
		return
	}

	// Декодируем JSON-данные запроса в структуру Task
	var task storage.Task
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
		task.Date = time.Now().Format(dateCalc.DateTemplate)
	}

	// Проверяем формат даты
	date, err := time.Parse(dateCalc.DateTemplate, task.Date)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "AddTaskHandler(): Date is not in the correct format"}, http.StatusBadRequest)
		return
	}

	// Проверка формата поля Repeat
	//fmt.Printf("Отладка task.Repeat %v \n", task.Repeat)
	if task.Repeat != "" {
		dateCheck, err := dateCalc.NextDate(time.Now(), task.Date, task.Repeat)
		//fmt.Printf("Отладка dateCheck %v \n", dateCheck)
		//fmt.Printf("Отладка err %v \n", err)
		if dateCheck == "" && err != nil {
			//fmt.Printf("Отладка 66 err %v \n", err)
			SendErrorResponse(w, ErrorResponse{Error: "AddTaskHandler() Invalid repetition condition"}, http.StatusBadRequest)
			return
		}
	}

	now := time.Now()
	if date.Before(now) {

		if task.Repeat == "" || date.Truncate(24*time.Hour) == date.Truncate(24*time.Hour) {
			task.Date = time.Now().Format(dateCalc.DateTemplate)
		} else {
			dateStr := date.Format(dateCalc.DateTemplate)
			nextDate, err := dateCalc.NextDate(now, dateStr, task.Repeat)
			if err != nil {
				SendErrorResponse(w, ErrorResponse{Error: "AddTaskHandler(): Wrong repetition rule"}, http.StatusBadRequest)
				return
			}
			task.Date = nextDate
		}
	}

	// Выполняем запрос INSERT в базу данных
	//tableName := "scheduler.db"
	//db, err := storage.OpenDataBase(tableName)
	//if err != nil {
	//SendErrorResponse(w, ErrorResponse{Error: fmt.Sprintf("Error opening database: %v", err)}, http.StatusInternalServerError)
	//return
	//}
	//defer db.Close()
	//db := h.DB

	query := "INSERT INTO scheduler (date, title, comment, repeat) VALUES (?, ?, ?, ?)"

	res, err := h.DB.Exec(query, task.Date, task.Title, task.Comment, task.Repeat)
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

	response := map[string]interface{}{"id": id}
	responseId, err := json.Marshal(response)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "AddTaskHandler(): JSON encoding error"}, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(responseId)
}

// Обрабатчик Get-запросов на получение списка ближайших задач
func (h *Handlers) GetTasksHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем метод запроса
	if r.Method != http.MethodGet {
		SendErrorResponse(w, ErrorResponse{Error: "GetListUpcomingTasksHandler(): Method not supported"}, http.StatusMethodNotAllowed)
		return
	}

	var tasks []storage.Task
	var task storage.Task
	var rows *sql.Rows
	var err error

	//tableName := "scheduler.db"

	//db, err := storage.OpenDataBase(tableName)
	//if err != nil {
	//SendErrorResponse(w, ErrorResponse{Error: fmt.Sprintf("Error opening database: %v", err)}, http.StatusInternalServerError)
	//return
	//}
	//defer db.Close()

	search := r.FormValue("search")

	switch {
	case search == "":
		// Делаем запрос по поиску всех задач с сортировкой по дате
		rows, err = h.DB.Query("SELECT id, date, title, comment, repeat FROM scheduler ORDER BY date DESC LIMIT ?", MaxTasksLimit)
		if err != nil {
			SendErrorResponse(w, ErrorResponse{Error: "GetListUpcomingTasksHandler(): Error executing database query"}, http.StatusInternalServerError)
			return
		}
		defer rows.Close()

	default:
		var searchDate time.Time
		// Попробуем распознать дату в формате "ггггммдд"
		searchDate, err = time.Parse(dateCalc.DateTemplate, search)
		if err != nil || searchDate.Year() == 1 {
			// Если не получилось, попробуем распознать дату в формате "дд.мм.гггг"
			searchDate, err = time.Parse("02.01.2006", search)
		}

		switch {
		case err == nil:
			// Если удалось распознать дату, делаем запрос по дате
			searchDateStr := searchDate.Format(dateCalc.DateTemplate)
			rows, err = h.DB.Query("SELECT id, date, title, comment, repeat FROM scheduler WHERE date = ? ORDER BY date DESC LIMIT ?", searchDateStr, MaxTasksLimit)
			if err != nil {
				SendErrorResponse(w, ErrorResponse{Error: "GetListUpcomingTasksHandler(): Error executing database query"}, http.StatusInternalServerError)
				return
			}
			defer rows.Close()

		default:
			// Если дата не распознана, делаем запрос по подстроке в title или comment
			searchParam := "%" + search + "%"
			rows, err = h.DB.Query("SELECT id, date, title, comment, repeat FROM scheduler WHERE title LIKE ? OR comment LIKE ? ORDER BY date DESC LIMIT ?", searchParam, searchParam, MaxTasksLimit)
			if err != nil {
				SendErrorResponse(w, ErrorResponse{Error: "GetListUpcomingTasksHandler(): Error executing database query"}, http.StatusInternalServerError)
				return
			}
			defer rows.Close()
		}
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

	// Обработка ошибок, возникшик при итерации
	if rows.Err() != nil {
		SendErrorResponse(w, ErrorResponse{Error: "GetListUpcomingTasksHandler(): Error occurred during rows iteration"}, http.StatusInternalServerError)
		return
	}

	// Если список задач пустой, возвращаем пустой массив
	if len(tasks) == 0 {
		tasks = []storage.Task{} // или просто nil, но не пустой массив
	}

	// Формируем ответ в формате JSON объекта с ключом "tasks"
	responseMap := map[string][]storage.Task{"tasks": tasks}
	response, err := json.Marshal(responseMap)
	//fmt.Printf("GetListUpcomingTasksHandler - response: %v\n", string(response))
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "GetListUpcomingTasksHandler(): JSON generation error"}, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(response)
}

// Обработчик Get-запросов на получение задачи по id для редактирования
func (h *Handlers) GetTaskByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendErrorResponse(w, ErrorResponse{Error: "GetTaskForEdit(): Method not supported"}, http.StatusBadRequest)
		return
	}

	idParam := r.FormValue("id")

	if idParam == "" {
		SendErrorResponse(w, ErrorResponse{Error: "GetTaskForEdit(): ID not specified"}, http.StatusBadRequest)
		return
	}

	//tableName := "scheduler.db"
	//db, err := storage.OpenDataBase(tableName)
	//if err != nil {
	//SendErrorResponse(w, ErrorResponse{Error: fmt.Sprintf("Error opening database: %v", err)}, http.StatusInternalServerError)
	//return
	//}
	//defer db.Close()

	var task storage.Task
	var id int64

	err := h.DB.QueryRow("SELECT id, date, title, comment, repeat FROM scheduler WHERE id = ?", idParam).Scan(&id, &task.Date, &task.Title, &task.Comment, &task.Repeat)
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
	_, _ = w.Write(response)
}

// Обработчик Put-запросов на сохранение изменений задачи
func (h *Handlers) SaveTaskHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем метод запроса
	if r.Method != http.MethodPut {
		SendErrorResponse(w, ErrorResponse{Error: "SaveEditTaskHandler(): Method not supported"}, http.StatusMethodNotAllowed)
		return
	}

	// Декодируем JSON-данные запроса в структуру Task
	var task storage.Task
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
		task.Date = time.Now().Format(dateCalc.DateTemplate)
	}

	// Проверяем формат даты
	_, err = time.Parse(dateCalc.DateTemplate, task.Date)
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
	//var existingID int
	//tableName := "scheduler.db"
	//db, err := storage.OpenDataBase(tableName)
	//if err != nil {
	//SendErrorResponse(w, ErrorResponse{Error: fmt.Sprintf("Error opening database: %v", err)}, http.StatusInternalServerError)
	//return
	//}
	//defer db.Close()

	// Выполняем запрос UPDATE в базу данных
	query := "UPDATE scheduler SET date = ?, title = ?, comment = ?, repeat =? WHERE id = ?"

	result, err := h.DB.Exec(query, task.Date, task.Title, task.Comment, task.Repeat, task.ID)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "SaveEditTaskHandler(): Task not found"}, http.StatusInternalServerError)
		return
	}

	// Проверяем кол-во затронутых строк
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "SaveEditTaskHandler(): Error retrieving update result"}, http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		SendErrorResponse(w, ErrorResponse{Error: "SaveEditTaskHandler(): Task not found"}, http.StatusInternalServerError)
		return
	}

	response, err := json.Marshal(struct{}{})
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "SaveEditTaskHandler(): JSON encoding error"}, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(response)
}

// Обработчик POST-запросов для отметки выполненной задачи
func (h *Handlers) DoneTaskHandler(w http.ResponseWriter, r *http.Request) {
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

	//tableName := "scheduler.db"
	//db, err := storage.OpenDataBase(tableName)
	//if err != nil {
	//SendErrorResponse(w, ErrorResponse{Error: fmt.Sprintf("Error opening database: %v", err)}, http.StatusInternalServerError)
	//return
	//}
	//defer db.Close()

	var task storage.Task
	var id int64
	err := h.DB.QueryRow("SELECT id, date, title, comment, repeat FROM scheduler WHERE id = ?", idParamNum).Scan(&id, &task.Date, &task.Title, &task.Comment, &task.Repeat)
	task.ID = fmt.Sprint(id)

	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			SendErrorResponse(w, ErrorResponse{Error: "DoneTaskHandler(): Task not found"}, http.StatusNotFound)
			return
		default:
			SendErrorResponse(w, ErrorResponse{Error: "DoneTaskHandler(): Error retrieving task data"}, http.StatusInternalServerError)
			return
		}
	}

	now := time.Now()

	switch task.Repeat {
	case "":
		query := "DELETE FROM scheduler WHERE id = ?"
		task.ID = fmt.Sprint(id)
		result, err := h.DB.Exec(query, task.ID)
		if err != nil {
			SendErrorResponse(w, ErrorResponse{Error: "DoneTaskHandler(): Error deleting task"}, http.StatusInternalServerError)
			return
		}

		// Проверка количества затронутых строк
		rowsAffected, err := result.RowsAffected()
		if err != nil || rowsAffected == 0 {
			SendErrorResponse(w, ErrorResponse{Error: "DoneTaskHandler(): Task not found or not deleted"}, http.StatusInternalServerError)
			return
		}

	default:
		// Определяем следующую дату задачи
		newTaskDate, err := dateCalc.NextDate(now, task.Date, task.Repeat)
		if err != nil {
			SendErrorResponse(w, ErrorResponse{Error: "DoneTaskHandler(): Incorrect task repetition condition"}, http.StatusBadRequest)
			return
		}
		// Изменяем дату задачи на новую
		task.Date = newTaskDate

		// Выполняем запрос UPDATE в базу данных
		query := "UPDATE scheduler SET date = ?, title = ?, comment = ?, repeat =? WHERE id = ?"
		_, err = h.DB.Exec(query, task.Date, &task.Title, &task.Comment, &task.Repeat, &task.ID)
		if err != nil {
			SendErrorResponse(w, ErrorResponse{Error: "DoneTaskHandler(): Task not found"}, http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{}`))
}

// Обработчик DELETE-запросов на удаление неактуальной задачи
func (h *Handlers) DeleteTaskHandler(w http.ResponseWriter, r *http.Request) {
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

	//fmt.Printf("Отладка Delete idParamNum %v \n", idParamNum)

	//tableName := "scheduler.db"
	//db, err := storage.OpenDataBase(tableName)
	//if err != nil {
	//SendErrorResponse(w, ErrorResponse{Error: fmt.Sprintf("Error opening database: %v", err)}, http.StatusInternalServerError)
	//return
	//}
	//defer db.Close()

	query := "DELETE FROM scheduler WHERE id = ?"
	result, err := h.DB.Exec(query, idParamNum)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "DeleteTaskHandler(): Error deleting task"}, http.StatusInternalServerError)
		return
	}

	// Проверка количества затронутых строк
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "DeleteTaskHandler(): Unable to determine the number of rows affected after deleting a task"}, http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		SendErrorResponse(w, ErrorResponse{Error: "DeleteTaskHandler(): Task not found"}, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{}`))
}

// Обработчик POST-запросов для сверки введенного пароля с паролем в переменной окружения
func UserAuthorizationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendErrorResponse(w, ErrorResponse{Error: "DoneTaskHandler(): method not supported"}, http.StatusBadRequest)
		return
	}

	// Чтение тела запроса
	//fmt.Printf("Отладка TODO_PASSWORD: %s\n", os.Getenv("TODO_PASSWORD"))
	body, err := io.ReadAll(r.Body)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "UserAuthorizationHandler(): Error reading request body"}, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var password auth.Password
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

	tokenString, err := token.SignedString([]byte(auth.JwtKey))
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
	tokenResponse := auth.Token{Token: tokenString}
	response, _ := json.Marshal(tokenResponse)
	_, _ = w.Write(response)
}
