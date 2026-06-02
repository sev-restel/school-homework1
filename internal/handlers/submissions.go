package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/K1viyt/school-homework/internal/database"
)

type Submission struct {
	ID          int    `json:"id"`
	StudentID   int    `json:"student_id"`
	Filepath    string `json:"filepath"`
	Grade       *int   `json:"grade"`
	Status      string `json:"status"`
	SubmittedAt string `json:"submitted_at"`
}

func CreateSubmissionHandler(w http.ResponseWriter, r *http.Request) {
	token, err := getUserFromToken(r)
	if err != nil {
		http.Error(w, "Пользователь не найдет в системе,попробуйте авторезироваться занова", http.StatusUnauthorized)
		return
	}
	role := token.Role
	if role != "student" {
		http.Error(w, "Доступ только учиникам", http.StatusForbidden)
		return
	}
	student_id := token.ID
	homework_string := r.FormValue("homework_id")
	homework_id, err := strconv.Atoi(homework_string)
	if err != nil {
		http.Error(w, "homework_id должен быть числом", http.StatusBadRequest)
		return
	}
	fUpload, h, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}
	defer fUpload.Close()
	diskName := uniqueDiskName(h.Filename)
	fDisk, err := os.Create(filepath.Join("uploads", diskName))
	if err != nil {
		http.Error(w, "Не удалось сохранить файл", http.StatusInternalServerError)
		log.Println("Ошибка создания файла:", err)
		return
	}
	defer fDisk.Close()
	_, err = io.Copy(fDisk, fUpload)
	if err != nil {
		http.Error(w, "Ошибка при получении файла", http.StatusBadRequest)
		return
	}

	_, err = database.DB.Exec(`INSERT INTO submissions(homework_id,student_id,filepath) VALUES($1,$2,$3)`, homework_id, student_id, uploadURLPath(diskName))
	if err != nil {
		http.Error(w, "Не удалось сохранить файл", http.StatusInternalServerError)
		log.Println("Ошибка создания файла:", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"message": "Работа загруженна", "id": homework_id})
}

func GetSubmissionsHandler(w http.ResponseWriter, r *http.Request) {
	token, err := getUserFromToken(r)
	if err != nil {
		http.Error(w, "Сессия окончина,авторизуйтесь заново", http.StatusUnauthorized)
		return
	}
	role := token.Role
	if role != "teacher" {
		http.Error(w, "Доступ только для учителя", http.StatusForbidden)
		return
	}
	homework_idString := r.PathValue("homework_id")
	homework_id, err := strconv.Atoi(homework_idString)
	if err != nil {
		http.Error(w, "homework_id должен быть числом", http.StatusBadRequest)
		return
	}

	// Учитель может смотреть работы только по СВОЕМУ заданию.
	var ownerID sql.NullInt64
	err = database.DB.QueryRow(`SELECT teacher_id FROM homework WHERE id = $1`, homework_id).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Задание не найдено", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Ошибка запроса к базе данных", http.StatusInternalServerError)
		return
	}
	if !ownerID.Valid || int(ownerID.Int64) != token.ID {
		http.Error(w, "Это задание принадлежит другому учителю", http.StatusForbidden)
		return
	}

	rows, err := database.DB.Query(`SELECT id, student_id, filepath, grade, status, submitted_at FROM submissions WHERE homework_id = $1`, homework_id)
	if err != nil {
		http.Error(w, "Ошибка запроса к базе данных", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var submissions []Submission

	for rows.Next() {
		var sb Submission
		err := rows.Scan(&sb.ID, &sb.StudentID, &sb.Filepath, &sb.Grade, &sb.Status, &sb.SubmittedAt)
		if err != nil {
			http.Error(w, "Некоректный тип данных", http.StatusBadRequest)
			return
		}
		submissions = append(submissions, sb)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(submissions)

}
func GradeSubmissionHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Grade int `json:"grade"`
	}

	token, err := getUserFromToken(r)
	if err != nil {
		http.Error(w, "Сессия окончина,авторизуйтесь заново", http.StatusUnauthorized)
		return
	}
	role := token.Role
	if role != "teacher" {
		http.Error(w, "Доступ только для учителя", http.StatusForbidden)
		return
	}
	err = json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		http.Error(w, "Некоректное значение", http.StatusBadRequest)
		return
	}
	if input.Grade < 1 || input.Grade > 5 {
		http.Error(w, "Недопустимо значение поля", http.StatusBadRequest)
		return
	}
	idString := r.PathValue("id")
	id, err := strconv.Atoi(idString)
	if err != nil {
		http.Error(w, "homework_id должен быть числом", http.StatusBadRequest)
		return
	}

	// Оценить можно только работу по своему заданию: ограничиваем подзапросом.
	result, err := database.DB.Exec(`UPDATE submissions SET grade=$1, status='graded'
		WHERE id=$2 AND homework_id IN (SELECT id FROM homework WHERE teacher_id=$3)`,
		input.Grade, id, token.ID)
	if err != nil {
		http.Error(w, "Ошибка обращения к базе", http.StatusInternalServerError)
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "Работа не найдена или относится к чужому заданию", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Оценка обновлена",
	})

}
