package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"

	"github.com/Mary-cross1296/go_final_project/auth"
	"github.com/Mary-cross1296/go_final_project/config"
)

// Функция для получения нового токена
func GetAndUpdateToken() error {
	// Определите URL и пароль для запроса
	url := "http://localhost:7540/api/signin"

	password := config.PassConfig
	if password == "" {
		password = "finalgo"
	}

	// Создание JSON тела запроса
	passwordData := auth.Password{Password: password}
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

	// Обновление файла settings.go
	settingsFilePath := "../tests/settings.go"

	err = UpdateSettingsFile(settingsFilePath, token)
	if err != nil {
		return fmt.Errorf("error updating settings file: %v", err)
	}
	return nil
}

// Функция для обновления файла settings.go
func UpdateSettingsFile(filePath, token string) error {
	// Проверка, существует ли файл
	if !FileExists(filePath) {
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
	return nil
}

// Функция для проверки существования файла
func FileExists(filepath string) bool {
	_, err := os.Stat(filepath)
	return !os.IsNotExist(err)
}
