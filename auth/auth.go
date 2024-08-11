package auth

import (
	"net/http"

	"github.com/Mary-cross1296/go_final_project/config"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

const JwtKey = "final_progect_go" // секретный ключ для подписи JWT

// Функция сравнения паролей
func comparePasswords(currentPassword string, tokenPasswordHash string) bool {
	// Сравнение текущего пароля с хэшем пароля из токена
	err := bcrypt.CompareHashAndPassword([]byte(tokenPasswordHash), []byte(currentPassword))
	return err == nil
}

func Auth(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pass := config.PassConfig
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
			return []byte(JwtKey), nil
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
