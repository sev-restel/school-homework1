package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/K1viyt/school-homework/internal/database"
)

type GradeView struct {
	SubmissionID int    `json:"submission_id"`
	HomeworkID   int    `json:"homework_id"`
	Subject      string `json:"subject"`
	StudentID    int    `json:"student_id"`
	FullName     string `json:"full_name"`
	Grade        int    `json:"grade"`
	SubmittedAt  string `json:"submitted_at"`
}

// GET /api/grades
// Ученик видит свои оценки, учитель/admin — все оценки всех учеников.
func GetGradesHandler(w http.ResponseWriter, r *http.Request) {
	user, err := getUserFromToken(r)
	if err != nil {
		http.Error(w, "Необходима авторизация", http.StatusUnauthorized)
		return
	}

	grades := []GradeView{}

	switch user.Role {
	case "student":
		rows, err := database.DB.Query(`
			SELECT s.id, s.homework_id, COALESCE(h.subject, ''),
			       s.student_id, u.full_name, s.grade, s.submitted_at
			FROM submissions s
			JOIN homework h ON s.homework_id = h.id
			JOIN users     u ON s.student_id  = u.id
			WHERE s.student_id = $1 AND s.status = 'graded'
			ORDER BY s.submitted_at DESC
		`, user.ID)
		if err != nil {
			http.Error(w, "Ошибка запроса к БД", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var g GradeView
			if err := rows.Scan(&g.SubmissionID, &g.HomeworkID, &g.Subject,
				&g.StudentID, &g.FullName, &g.Grade, &g.SubmittedAt); err != nil {
				http.Error(w, "Ошибка чтения данных", http.StatusInternalServerError)
				return
			}
			grades = append(grades, g)
		}

	case "teacher", "admin":
		rows, err := database.DB.Query(`
			SELECT s.id, s.homework_id, COALESCE(h.subject, ''),
			       s.student_id, u.full_name, s.grade, s.submitted_at
			FROM submissions s
			JOIN homework h ON s.homework_id = h.id
			JOIN users     u ON s.student_id  = u.id
			WHERE s.status = 'graded'
			ORDER BY s.submitted_at DESC
		`)
		if err != nil {
			http.Error(w, "Ошибка запроса к БД", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var g GradeView
			if err := rows.Scan(&g.SubmissionID, &g.HomeworkID, &g.Subject,
				&g.StudentID, &g.FullName, &g.Grade, &g.SubmittedAt); err != nil {
				http.Error(w, "Ошибка чтения данных", http.StatusInternalServerError)
				return
			}
			grades = append(grades, g)
		}

	default:
		http.Error(w, "Доступ запрещён", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(grades)
}

// GET /api/grades/{student_id}
// Только учитель / admin — оценки конкретного ученика.
func GetStudentGradesHandler(w http.ResponseWriter, r *http.Request) {
	user, err := getUserFromToken(r)
	if err != nil {
		http.Error(w, "Необходима авторизация", http.StatusUnauthorized)
		return
	}
	if user.Role != "teacher" && user.Role != "admin" {
		http.Error(w, "Доступ только для учителя или администратора", http.StatusForbidden)
		return
	}

	studentID, err := strconv.Atoi(r.PathValue("student_id"))
	if err != nil {
		http.Error(w, "Некорректный student_id", http.StatusBadRequest)
		return
	}

	rows, err := database.DB.Query(`
		SELECT s.id, s.homework_id, COALESCE(h.subject, ''),
		       s.student_id, u.full_name, s.grade, s.submitted_at
		FROM submissions s
		JOIN homework h ON s.homework_id = h.id
		JOIN users     u ON s.student_id  = u.id
		WHERE s.student_id = $1 AND s.status = 'graded'
		ORDER BY s.submitted_at DESC
	`, studentID)
	if err != nil {
		http.Error(w, "Ошибка запроса к БД", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	grades := []GradeView{}
	for rows.Next() {
		var g GradeView
		if err := rows.Scan(&g.SubmissionID, &g.HomeworkID, &g.Subject,
			&g.StudentID, &g.FullName, &g.Grade, &g.SubmittedAt); err != nil {
			http.Error(w, "Ошибка чтения данных", http.StatusInternalServerError)
			return
		}
		grades = append(grades, g)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(grades)
}
