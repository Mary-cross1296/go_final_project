package api

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Mary-cross1296/go_final_project/auth"
	"github.com/Mary-cross1296/go_final_project/config"
	"github.com/Mary-cross1296/go_final_project/dates"
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
	now, err := time.Parse(dates.DateTemplate, nowTime)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "NextDateHandler(): Invalid 'now' parameter format. Use YYYYMMDD"}, http.StatusBadRequest)
		return
	}

	// Вызываем функцию NextDate для получения следующей даты
	nextDate, err := dates.NextDate(now, date, repeat)
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
		task.Date = time.Now().Format(dates.DateTemplate)
	}

	// Проверяем формат даты
	date, err := time.Parse(dates.DateTemplate, task.Date)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "AddTaskHandler(): Date is not in the correct format"}, http.StatusBadRequest)
		return
	}

	// Проверка формата поля Repeat
	if task.Repeat != "" {
		dateCheck, err := dates.NextDate(time.Now(), task.Date, task.Repeat)
		if dateCheck == "" && err != nil {
			SendErrorResponse(w, ErrorResponse{Error: "AddTaskHandler() Invalid repetition condition"}, http.StatusBadRequest)
			return
		}
	}

	now := time.Now()
	if date.Before(now) {

		if task.Repeat == "" || date.Truncate(24*time.Hour) == date.Truncate(24*time.Hour) {
			task.Date = time.Now().Format(dates.DateTemplate)
		} else {
			dateStr := date.Format(dates.DateTemplate)
			nextDate, err := dates.NextDate(now, dateStr, task.Repeat)
			if err != nil {
				SendErrorResponse(w, ErrorResponse{Error: "AddTaskHandler(): Wrong repetition rule"}, http.StatusBadRequest)
				return
			}
			task.Date = nextDate
		}
	}

	id, err := storage.InsertTask(h.DB, task)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "AddTaskHandler():" + err.Error()}, http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{"id": id}
	responseId, err := json.Marshal(response)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "AddTaskHandler(): JSON encoding error"}, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(responseId)
}

func (h *Handlers) GetTasksHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем метод запроса
	if r.Method != http.MethodGet {
		SendErrorResponse(w, ErrorResponse{Error: "GetTasksHandler(): Method not supported"}, http.StatusMethodNotAllowed)
		return
	}

	search := r.FormValue("search")

	var rows *sql.Rows
	var err error

	switch {
	case search == "":
		// Запрос всех задач с сортировкой по дате
		rows, err = storage.GetAllTask(h.DB, MaxTasksLimit)
		if err != nil {
			SendErrorResponse(w, ErrorResponse{Error: "GetTasksHandler():" + err.Error()}, http.StatusInternalServerError)
			return
		}
	case err == nil && search != "":
		var searchDate time.Time
		// Попробуем распознать дату в формате "ггггммдд"
		searchDate, err = time.Parse(dates.DateTemplate, search)
		if err != nil || searchDate.Year() == 1 {
			// Если не получилось, попробуем распознать дату в формате "дд.мм.гггг"
			searchDate, err = time.Parse("02.01.2006", search)
		}

		if err == nil {
			// Если удалось распознать дату, делаем запрос по дате
			rows, err = storage.GetTaskByDate(h.DB, MaxTasksLimit, searchDate)
			if err != nil {
				SendErrorResponse(w, ErrorResponse{Error: "GetTasksHandler():" + err.Error()}, http.StatusInternalServerError)
				return
			}
		} else {
			// Иначе выполняем поиск по подстроке в title или comment
			rows, err = storage.GetTaskByString(h.DB, search, MaxTasksLimit)
			if err != nil {
				SendErrorResponse(w, ErrorResponse{Error: "GetTasksHandler():" + err.Error()}, http.StatusInternalServerError)
				return
			}
		}
	}

	defer func() {
		if rows != nil {
			rows.Close()
		}
	}()

	tasks, err := storage.ScanResult(rows)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "GetTasksHandler():" + err.Error()}, http.StatusInternalServerError)
		return
	}

	responseMap := map[string][]storage.Task{"tasks": tasks}
	response, err := json.Marshal(responseMap)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "GetTasksHandler(): JSON generation error"}, http.StatusInternalServerError)
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

	idParamNum, _ := strconv.Atoi(idParam)

	task, err := storage.GetTaskByID(h.DB, idParamNum)
	if err == sql.ErrNoRows {
		SendErrorResponse(w, ErrorResponse{Error: "GetTaskForEdit(): Task not found"}, http.StatusNotFound)
		return
	} else if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "GetTaskForEdit(): Error retrieving task data"}, http.StatusInternalServerError)
		return
	}

	// Формируем ответ в формате JSON объекта с ключом "tasks"
	response, err := json.Marshal(task)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "GetTaskForEdit(): JSON generation error"}, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
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
		task.Date = time.Now().Format(dates.DateTemplate)
	}

	// Проверяем формат даты
	_, err = time.Parse(dates.DateTemplate, task.Date)
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

	// Выполняем запрос UPDATE в базу данных
	err = storage.UpdateTask(h.DB, task)
	if err != nil {
		if err.Error() == "no rows affected" {
			SendErrorResponse(w, ErrorResponse{Error: "SaveEditTaskHandler():" + err.Error()}, http.StatusNotFound)
		} else {
			SendErrorResponse(w, ErrorResponse{Error: "SaveEditTaskHandler(): Internal server error"}, http.StatusInternalServerError)
		}
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
	idParamNum, _ := strconv.Atoi(idParam)

	task, err := storage.GetTaskByID(h.DB, idParamNum)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "DoneTaskHandler():" + err.Error()}, http.StatusBadRequest)
		return
	}

	now := time.Now()

	switch task.Repeat {
	case "":
		err = storage.DeleteTaskByID(h.DB, idParamNum)
		if err != nil {
			SendErrorResponse(w, ErrorResponse{Error: "DoneTaskHandler():" + err.Error()}, http.StatusBadRequest)
		}

	default:
		// Определяем следующую дату задачи
		newTaskDate, err := dates.NextDate(now, task.Date, task.Repeat)
		if err != nil {
			SendErrorResponse(w, ErrorResponse{Error: "DoneTaskHandler(): Incorrect task repetition condition"}, http.StatusBadRequest)
			return
		}
		// Изменяем дату задачи на новую
		task.Date = newTaskDate

		err = storage.UpdateTask(h.DB, task)
		if err != nil {
			SendErrorResponse(w, ErrorResponse{Error: "DoneTaskHandler():" + err.Error()}, http.StatusBadRequest)
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

	err = storage.DeleteTaskByID(h.DB, idParamNum)
	if err != nil {
		SendErrorResponse(w, ErrorResponse{Error: "DeleteTaskHandler():" + err.Error()}, http.StatusInternalServerError)
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

	tokenString, err := token.SignedString([]byte(config.JwtKeyConfig))
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
