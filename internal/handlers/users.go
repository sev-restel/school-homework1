package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/K1viyt/school-homework/internal/database"
)

type TeacherView struct {
	ID       int    `json:"id"`
	FullName string `json:"full_name"`
	Subject  string `json:"subject"`
}

// GET /api/teachers — список учителей (любой авторизованный).
// Нужен фронтенду admin-а, чтобы заполнить выпадающий список при создании расписания.
func ListTeachersHandler(w http.ResponseWriter, r *http.Request) {
	_, err := getUserFromToken(r)
	if err != nil {
		http.Error(w, "Необходима авторизация", http.StatusUnauthorized)
		return
	}

	rows, err := database.DB.Query(`
		SELECT id, full_name, COALESCE(subject, '')
		FROM users
		WHERE role = 'teacher'
		ORDER BY full_name
	`)
	if err != nil {
		http.Error(w, "Ошибка запроса к БД", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	teachers := []TeacherView{}
	for rows.Next() {
		var t TeacherView
		if err := rows.Scan(&t.ID, &t.FullName, &t.Subject); err != nil {
			http.Error(w, "Ошибка чтения данных", http.StatusInternalServerError)
			return
		}
		teachers = append(teachers, t)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(teachers)
}
