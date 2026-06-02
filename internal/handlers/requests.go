package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/K1viyt/school-homework/internal/database"
)

type DecideRequestInput struct {
	Status string `json:"status"`
}

type CreateRequestInput struct {
	ClassName string `json:"class_name"`
}

type RequestView struct {
	ID        int    `json:"id"`
	StudentID int    `json:"student_id"`
	FullName  string `json:"full_name"`
	ClassName string `json:"class_name"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

func CreateRequestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Разрешён только POST", http.StatusMethodNotAllowed)
		return
	}
	var input CreateRequestInput
	user, err := getUserFromToken(r)
	if err != nil {
		http.Error(w, "Пользователь не найден", http.StatusUnauthorized)
		return
	}
	if user.Role != "student" {
		http.Error(w, "Доступ только пользователям с рулью: student", http.StatusForbidden)
		return
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Невалидный JSON", http.StatusBadRequest)
		return

	}
	if input.ClassName == "" {
		http.Error(w, "class_name обязателен", http.StatusBadRequest)
		return
	}

	var pendingCount int
	err = database.DB.QueryRow(
		`SELECT COUNT(*) FROM requests WHERE student_id=$1 AND status='pending'`,
		user.ID,
	).Scan(&pendingCount)
	if err != nil {
		http.Error(w, "Ошибка при проверке заявок", http.StatusInternalServerError)
		return
	}
	if pendingCount > 0 {
		http.Error(w, "У вас уже есть активная заявка", http.StatusConflict)
		return
	}
	var newID int64

	err = database.DB.QueryRow(`INSERT INTO requests(student_id, class_name) VALUES($1,$2) RETURNING id`, user.ID, input.ClassName).Scan(&newID)
	if err != nil {
		http.Error(w, "Ошибка записи данных в базу requests", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"id":      newID,
		"message": "Заявка отправлена",
	})

}
func GetRequestsHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		http.Error(w, "Разрешон только GET", http.StatusMethodNotAllowed)
		return
	}
	requestView := []RequestView{}
	user, err := getUserFromToken(r)
	if err != nil {
		http.Error(w, "Ваш сеанс завершён авторизуйтесь заново", http.StatusUnauthorized)
		return
	}
	if user.Role != "teacher" && user.Role != "admin" {
		http.Error(w, "Доступ открыт только доля пользователей с ролью :admin или teacher", http.StatusForbidden)
		return
	}
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "pending"
	}
	if status != "pending" && status != "accepted" && status != "rejected" {
		http.Error(w, "status должен быть pending|accepted|rejected", http.StatusBadRequest)
		return
	}

	rows, err := database.DB.Query(`SELECT r.id, r.student_id, r.class_name, r.status, r.created_at,
       u.full_name
FROM requests r
JOIN users u ON r.student_id = u.id
WHERE r.status = $1
ORDER BY r.created_at DESC`, status)
	if err != nil {
		http.Error(w, "Потеряно соединение с BD requests || users", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var req RequestView
		if err := rows.Scan(&req.ID, &req.StudentID, &req.ClassName, &req.Status, &req.CreatedAt, &req.FullName); err != nil {
			http.Error(w, "Не удалось ничего найти по данному status", http.StatusInternalServerError)
			return
		}
		requestView = append(requestView, req)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(requestView)

}

func ApprovedHandler(w http.ResponseWriter, r *http.Request) {
	var decJson DecideRequestInput
	if r.Method != http.MethodPatch {
		http.Error(w, "Разрешон только Patch", http.StatusMethodNotAllowed)
		return
	}
	user, err := getUserFromToken(r)
	if err != nil {
		http.Error(w, "Ваш сеанс завершён авторизуйтесь заново", http.StatusUnauthorized)
		return
	}
	if user.Role != "teacher" && user.Role != "admin" {
		http.Error(w, "Доступ открыт только доля пользователей с ролью :admin или teacher", http.StatusForbidden)
		return
	}
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Некоректный тип данных", http.StatusBadRequest)
		return
	}

	err = json.NewDecoder(r.Body).Decode(&decJson)
	if err != nil {
		http.Error(w, "Вы передали некоректный тип значения", http.StatusBadRequest)
		return
	}

	if decJson.Status != "accepted" && decJson.Status != "rejected" {
		http.Error(w, "Текущий статус заявки не соответствует необходимому", http.StatusBadRequest)
		return
	}
	var studentID int
	var className string
	var currentStatus string

	tx, err := database.DB.Begin()
	if err != nil {
		http.Error(w, "Не удалось начать транзакцию", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback() //если отвал до commit вернет все как было

	err = tx.QueryRow(
		`SELECT student_id, class_name, status FROM requests WHERE id=$1`,
		id,
	).Scan(&studentID, &className, &currentStatus)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Заявка не найдена", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Ошибка чтения заявки", http.StatusInternalServerError)
		return
	}
	if currentStatus != "pending" {
		http.Error(w, "Заявка уже обработана", http.StatusConflict)
		return
	}

	_, err = tx.Exec(`UPDATE requests SET status=$1, teacher_id=$2 WHERE id=$3`, decJson.Status, user.ID, id)
	if err != nil {
		http.Error(w, "Ошибка обновления заявки", http.StatusInternalServerError)
		return
	}
	if decJson.Status == "accepted" {
		_, err = tx.Exec(`UPDATE users SET class=$1 WHERE id=$2`, className, studentID)
		if err != nil {
			http.Error(w, "Ошибка обновления класса ученика", http.StatusInternalServerError)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "Ошибка коммита транзакции", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id":      id,
		"status":  decJson.Status,
		"message": "Заявка обновлена",
	})
}
