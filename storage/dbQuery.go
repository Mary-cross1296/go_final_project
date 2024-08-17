package storage

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/Mary-cross1296/go_final_project/dates"
)

func DeleteTaskByID(db *DataBase, id int) error {
	query := "DELETE FROM scheduler WHERE id = ?"
	result, err := db.Exec(query, id)
	if err != nil {
		log.Printf("DeleteTaskByID(): Error deleting task")
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("DeleteTaskByID(): Unable to determine the number of rows affected after deleting a task")
		return err
	}

	if rowsAffected == 0 {
		log.Printf("DeleteTaskHandler(): Task not found")
		return err
	}
	return nil
}

func GetTaskByID(db *DataBase, id int) (Task, error) {
	var task Task
	err := db.QueryRow("SELECT id, date, title, comment, repeat FROM scheduler WHERE id = ?", id).Scan(&id, &task.Date, &task.Title, &task.Comment, &task.Repeat)
	task.ID = fmt.Sprint(id)

	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			log.Printf("GetTaskByID(): Task not found")
			return task, err
		default:
			log.Printf("GetTaskByID(): Error retrieving task data")
			return task, err
		}
	}
	return task, nil
}

func UpdateTask(db *DataBase, task Task) error {
	id, _ := strconv.Atoi(task.ID)
	query := `UPDATE scheduler SET date = ?, title = ?, comment = ?, repeat =? WHERE id = ?`
	result, err := db.Exec(query, task.Date, task.Title, task.Comment, task.Repeat, id)
	if err != nil {
		log.Printf("UpdateTask(): Task not found")
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("UpdateTask(): Unable to determine number of affected rows after task update")
		return err
	}

	if rowsAffected == 0 {
		log.Printf("UpdateTask(): Task update error")
		return fmt.Errorf("no rows affected")
	}
	return nil
}

func GetAllTask(db *DataBase, taskLimit int) (*sql.Rows, error) {
	// Делаем запрос по поиску всех задач с сортировкой по дате
	rows, err := db.Query("SELECT id, date, title, comment, repeat FROM scheduler ORDER BY date DESC LIMIT ?", taskLimit)
	if err != nil {
		log.Printf("GetAllTask(): Error executing database query")
		return rows, err
	}
	return rows, nil
}

func GetTaskByDate(db *DataBase, taskLimit int, searchDate time.Time) (*sql.Rows, error) {
	// Делаем запрос по поиску всех задач за определенную дату
	searchDateStr := searchDate.Format(dates.DateTemplate)
	rows, err := db.Query("SELECT id, date, title, comment, repeat FROM scheduler WHERE date = ? ORDER BY date DESC LIMIT ?", searchDateStr, taskLimit)
	if err != nil {
		log.Printf("GetTaskByDate(): Error executing database query")
		return rows, err
	}
	return rows, nil
}

func GetTaskByString(db *DataBase, search string, taskLimit int) (*sql.Rows, error) {
	// Lелаем запрос по подстроке в title или comment
	searchParam := "%" + search + "%"
	rows, err := db.Query("SELECT id, date, title, comment, repeat FROM scheduler WHERE title LIKE ? OR comment LIKE ? ORDER BY date DESC LIMIT ?", searchParam, searchParam, taskLimit)
	if err != nil {
		log.Printf("GetTaskByString(): Error executing database query")
		return rows, err
	}
	return rows, nil
}

func ScanResult(rows *sql.Rows) ([]Task, error) {
	var task Task
	var tasks []Task

	for rows.Next() {
		var id int64
		if err := rows.Scan(&id, &task.Date, &task.Title, &task.Comment, &task.Repeat); err != nil {
			log.Printf("ScanResult(): Error scanning information received from the database")
			return nil, err
		}
		task.ID = fmt.Sprint(id)
		tasks = append(tasks, task)
	}

	// Обработка ошибок, возникшик при итерации
	if rows.Err() != nil {
		log.Printf("ScanResult(): Error occurred during rows iteration")
		return nil, rows.Err()
	}

	// Если список задач пустой, возвращаем пустой массив
	if len(tasks) == 0 {
		tasks = []Task{} // или просто nil, но не пустой массив
	}
	return tasks, nil
}

func InsertTask(db *DataBase, task Task) (int64, error) {
	query := "INSERT INTO scheduler (date, title, comment, repeat) VALUES (?, ?, ?, ?)"

	res, err := db.Exec(query, task.Date, task.Title, task.Comment, task.Repeat)
	if err != nil {
		log.Printf("InsertTask(): Error executing request")
		return 0, err
	}

	// Получаем ID добавленной задачи
	id, err := res.LastInsertId()
	if err != nil {
		log.Printf("InsertTask(): Error getting task ID")
		return 0, err
	}

	// Устанавливаем полученный id в качестве строки
	//task.ID = fmt.Sprint(id)
	return id, nil
}
