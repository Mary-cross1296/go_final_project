package tests

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func getURL(path string) string {
	port := Port
	envPort := os.Getenv("TODO_PORT")
	if len(envPort) > 0 {
		if eport, err := strconv.ParseInt(envPort, 10, 32); err == nil {
			port = int(eport)
		}
	}
	path = strings.TrimPrefix(path, "..\\web\\")
	path = strings.TrimPrefix(path, "../web/")

	path = strings.ReplaceAll(path, `\`, `/`)

	return fmt.Sprintf("http://localhost:%d/%s", port, path)
}

func getBody(path string) ([]byte, error) {
	url := getURL(path)
	fmt.Printf("Запрашиваемый URL: %s \n", url) // Отладочный вывод
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	fmt.Printf("Полученный ответ: %s \n", string(body)) // Отладочный вывод
	return body, err
}

func walkDir(path string, f func(fname string) error) error {
	dirs, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, v := range dirs {
		fname := filepath.Join(path, v.Name())
		if v.IsDir() {
			if err = walkDir(fname, f); err != nil {
				return err
			}
			continue
		}
		if err = f(fname); err != nil {
			return err
		}
	}
	return nil
}

func TestApp(t *testing.T) {
	cmp := func(fname string) error {
		fbody, err := os.ReadFile(fname)
		if err != nil {
			return err
		}
		body, err := getBody(fname)
		if err != nil {
			return err
		}
		assert.Equal(t, len(fbody), len(body), `сервер возвращает для %s данные другого размера`, fname)
		return nil
	}
	assert.NoError(t, walkDir("../web/", cmp))
}
