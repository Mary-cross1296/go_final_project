package api

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse представляет структуру ошибки
type ErrorResponse struct {
	Error string `json:"error"`
}

// Функция для отправки ошибочного ответа
func SendErrorResponse(w http.ResponseWriter, errResp ErrorResponse, statusCode int) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(statusCode)
	response, _ := json.Marshal(errResp)
	w.Write(response)
}
