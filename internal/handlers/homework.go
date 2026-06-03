package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/K1viyt/school-homework/internal/database"
)

type Homework struct {
	ID          int     `json:"id"`
	Filename    string  `json:"filename"`
	Filepath    string  `json:"filepath"`
	ClassName   *string `json:"class_name"` // класс, для которого выдано задание
	Subject     *string `json:"subject"`
	Description *string `json:"description"`
	UploadedAt  string  `json:"uploaded_at"`
}

type User struct {
	ID       int
	Username string
	Role     string
}

func UploadHandler(w http.ResponseWriter, r *http.Request) {
	//Проверка на POST
	if r.Method != http.MethodPost {
		http.Error(w, "Разрешён только POST", http.StatusMethodNotAllowed)
		return // ← без этого код ниже выполнится!
	}
	// Проверка авторизации
	user, err := getUserFromToken(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	// Проверка роли (только для хэндлеров учителя)
	if user.Role != "teacher" {
		http.Error(w, "Доступ запрещён", http.StatusForbidden)
		return
	}
	//Установка ограничение на чтение
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	//Класс, для которого выдаётся задание — обязателен, иначе его увидят все классы.
	className := normClass(r.FormValue("class"))
	if className == "" {
		http.Error(w, "Укажите класс, для которого задание", http.StatusBadRequest)
		return
	}
	//Получаем мето данные файла и закидывем сам файл в память
	file, h, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Ошибка при получении файла", http.StatusBadRequest)
		return
	}
	defer file.Close()
	//Уникальное имя на диске, оригинальное имя сохраним отдельно для показа.
	diskName := uniqueDiskName(h.Filename)
	//Создаем файл по пути...
	f, err := os.Create(filepath.Join("uploads", diskName))
	if err != nil {
		http.Error(w, "Не удалось сохранить файл", http.StatusInternalServerError)
		log.Println("Ошибка создания файла:", err)
		return
	}
	defer f.Close()
	//Копируем file в f
	_, err = io.Copy(f, file)
	if err != nil {
		http.Error(w, "Ошибка при получении файла", http.StatusBadRequest)
		return
	}
	//Считываем инфу из таблицы
	subject := r.FormValue("subject")
	description := r.FormValue("description")
	var subjectVal any
	var descriptionVal any

	if subject != "" {
		subjectVal = subject
	} else {
		subjectVal = nil
	}
	if description != "" {
		descriptionVal = description

	} else {
		descriptionVal = nil
	}
	displayName := filepath.Base(h.Filename)
	_, err = database.DB.Exec(`INSERT INTO homework(filename, filepath, class_name, subject, description, teacher_id) VALUES($1, $2, $3, $4, $5, $6)`,
		displayName, uploadURLPath(diskName), className, subjectVal, descriptionVal, user.ID)
	if err != nil {
		http.Error(w, "Ошибка записи в базу данных", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "файл %s загружен", displayName)

}

func ListHomeworksHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Проверяем что метод GET
	if r.Method != http.MethodGet {
		http.Error(w, "Разрешон только GET", http.StatusMethodNotAllowed)
		return
	}
	// Проверка авторизации
	user, err := getUserFromToken(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	// Строим запрос в зависимости от роли:
	//  - teacher → только свои задания
	//  - student → все задания, но ТОЛЬКО если он принят в класс (есть class)
	//  - admin   → все задания
	// Необязательный фильтр по предмету: GET /api/homeworks?subject=Математика
	subject := r.URL.Query().Get("subject")

	conds := []string{}
	args := []any{}

	switch user.Role {
	case "teacher":
		conds = append(conds, fmt.Sprintf("teacher_id = $%d", len(args)+1))
		args = append(args, user.ID)
	case "student":
		var class sql.NullString
		if err := database.DB.QueryRow(`SELECT class FROM users WHERE id = $1`, user.ID).Scan(&class); err != nil {
			http.Error(w, "Ошибка запроса к базе данных", http.StatusInternalServerError)
			return
		}
		// Не принят ни в один класс — заданий пока не видит (ТЗ, п.6.1).
		if !class.Valid || strings.TrimSpace(class.String) == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]Homework{})
			return
		}
		// Видит только задания СВОЕГО класса.
		conds = append(conds, fmt.Sprintf("class_name = $%d", len(args)+1))
		args = append(args, normClass(class.String))
	}

	if subject != "" {
		conds = append(conds, fmt.Sprintf("subject = $%d", len(args)+1))
		args = append(args, subject)
	}

	query := `SELECT id, filename, filepath, class_name, subject, description FROM homework`
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY id DESC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "Ошибка запроса к базе данных", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	homeworks := []Homework{}
	for rows.Next() {
		var hw Homework
		if err := rows.Scan(&hw.ID, &hw.Filename, &hw.Filepath, &hw.ClassName, &hw.Subject, &hw.Description); err != nil {
			http.Error(w, "Некоректный тип данных", http.StatusBadRequest)
			return
		}
		homeworks = append(homeworks, hw)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(homeworks)
}

func DeleteHomeworkHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Разрешон только Delete", http.StatusMethodNotAllowed)
		return
	}
	// Проверка авторизации
	user, err := getUserFromToken(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	// Проверка роли (только для хэндлеров учителя)
	if user.Role != "teacher" {
		http.Error(w, "Доступ запрещён", http.StatusForbidden)
		return
	}
	//Получаю ID из пути /api/homeworks/{id}
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Некорректный id", http.StatusBadRequest)
		return
	}
	//Удаляем по ID,filepath
	var fpath string
	err = database.DB.QueryRow(`SELECT filepath FROM homework WHERE id=$1 AND teacher_id=$2`, id, user.ID).Scan(&fpath)

	if err != nil {
		http.Error(w, "Задание не найдено", http.StatusNotFound)
		return
	}
	_, err = database.DB.Exec(`DELETE FROM homework WHERE id=$1 AND teacher_id=$2`, id, user.ID)
	if err != nil {
		http.Error(w, "Ошибка удаления из базы", http.StatusInternalServerError)
		return
	}
	os.Remove(fpath)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Задание удалено",
	})

}
func UpdateHomeworkHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, "Разрешон только Patch", http.StatusMethodNotAllowed)
		return
	}
	//Ограничение на загрузку
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	// Проверка авторизации
	user, err := getUserFromToken(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	// Проверка роли (только для хэндлеров учителя)
	if user.Role != "teacher" {
		http.Error(w, "Доступ запрещён", http.StatusForbidden)
		return
	}
	var input struct {
		Subject     *string `json:"subject"`
		Description *string `json:"description"`
	}

	idSrs := r.PathValue("id")
	id, err := strconv.Atoi(idSrs)
	if err != nil {
		http.Error(w, "Некорректный id", http.StatusBadRequest)
		return
	}
	err = json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		http.Error(w, "Некорректный JSON", http.StatusBadRequest)
		return
	}

	result, err := database.DB.Exec(`UPDATE homework SET subject=$1, description=$2 WHERE id=$3 AND teacher_id=$4`,
		input.Subject, input.Description, id, user.ID)
	if err != nil {
		http.Error(w, "Ошибка обновления в базе", http.StatusInternalServerError)
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "Задание не найдено", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Задание обновлено",
	})
}

func ReplaceHomeworkHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Разрешон только PUT", http.StatusMethodNotAllowed)
		return
	}
	// Проверка авторизации
	user, err := getUserFromToken(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	// Проверка роли (только для хэндлеров учителя)
	if user.Role != "teacher" {
		http.Error(w, "Доступ запрещён", http.StatusForbidden)
		return
	}

	idSrs := r.PathValue("id")
	id, err := strconv.Atoi(idSrs)
	if err != nil {
		http.Error(w, "Некоректный тип данных", http.StatusBadRequest)
		return
	}
	var fpath string
	err = database.DB.QueryRow(`SELECT filepath FROM homework WHERE id=$1 AND teacher_id=$2`, id, user.ID).Scan(&fpath)
	if err != nil {
		http.Error(w, "Такого id нету", http.StatusNotFound)
		return
	}

	//Установка ограничение на чтение
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

	file, h, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Ошибка при получении файла", http.StatusBadRequest)
		return
	}
	defer file.Close()

	diskName := uniqueDiskName(h.Filename)
	f, err := os.Create(filepath.Join("uploads", diskName))
	if err != nil {
		http.Error(w, "Не удалось сохранить файл", http.StatusInternalServerError)
		log.Println("Ошибка создания файла:", err)
		return
	}
	defer f.Close()

	_, err = io.Copy(f, file)
	if err != nil {
		http.Error(w, "Ошибка копирования в новый файл", http.StatusBadRequest)
		return
	}
	subject := r.FormValue("subject")
	description := r.FormValue("description")
	var subjectVal any
	var descriptionVal any

	if subject != "" {
		subjectVal = subject
	} else {
		subjectVal = nil
	}
	if description != "" {
		descriptionVal = description

	} else {
		descriptionVal = nil
	}
	displayName := filepath.Base(h.Filename)
	_, err = database.DB.Exec(`UPDATE homework SET filename=$1,filepath=$2,subject=$3,description=$4 WHERE id=$5 AND teacher_id=$6`, displayName, uploadURLPath(diskName), subjectVal, descriptionVal, id, user.ID)
	if err != nil {
		http.Error(w, "Ошибка перезаписи файла в базу", http.StatusInternalServerError)
		return
	}
	os.Remove(fpath)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Задание заменено",
	})
}
