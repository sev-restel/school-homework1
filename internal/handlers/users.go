package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/K1viyt/school-homework/internal/database"
	"golang.org/x/crypto/bcrypt"
)

type TeacherView struct {
	ID       int    `json:"id"`
	FullName string `json:"full_name"`
	Subject  string `json:"subject"`
}

type UserView struct {
	ID       int    `json:"id"`
	FullName string `json:"full_name"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Class    string `json:"class"`
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

// requireAdmin — общая проверка: авторизован и роль admin.
// Возвращает пользователя и true, если всё ок; иначе сам пишет ответ и false.
func requireAdmin(w http.ResponseWriter, r *http.Request) (*User, bool) {
	user, err := getUserFromToken(r)
	if err != nil {
		http.Error(w, "Необходима авторизация", http.StatusUnauthorized)
		return nil, false
	}
	if user.Role != "admin" {
		http.Error(w, "Доступ только для администратора", http.StatusForbidden)
		return nil, false
	}
	return user, true
}

// GET /api/users — список всех пользователей (только admin).
// Необязательный фильтр по роли: ?role=student|teacher|admin.
func ListUsersHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}

	roleFilter := r.URL.Query().Get("role")
	var rows *sql.Rows
	var err error
	if roleFilter != "" {
		rows, err = database.DB.Query(`
			SELECT id, full_name, username, role, COALESCE(class, ''), COALESCE(subject, '')
			FROM users WHERE role = $1 ORDER BY full_name`, roleFilter)
	} else {
		rows, err = database.DB.Query(`
			SELECT id, full_name, username, role, COALESCE(class, ''), COALESCE(subject, '')
			FROM users ORDER BY role, full_name`)
	}
	if err != nil {
		http.Error(w, "Ошибка запроса к БД", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	users := []UserView{}
	for rows.Next() {
		var u UserView
		if err := rows.Scan(&u.ID, &u.FullName, &u.Username, &u.Role, &u.Class, &u.Subject); err != nil {
			http.Error(w, "Ошибка чтения данных", http.StatusInternalServerError)
			return
		}
		users = append(users, u)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// PATCH /api/users/{id}/kick — исключить ученика из класса (class → NULL).
func KickFromClassHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Некорректный id", http.StatusBadRequest)
		return
	}

	res, err := database.DB.Exec(`UPDATE users SET class = NULL WHERE id = $1 AND role = 'student'`, id)
	if err != nil {
		http.Error(w, "Ошибка обновления", http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		http.Error(w, "Ученик не найден", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"id": id, "message": "Ученик исключён из класса"})
}

// PATCH /api/users/{id}/password — админ задаёт новый пароль пользователю.
// Нужен для восстановления доступа: ученик/учитель забыл пароль — админ сбрасывает.
func ResetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Некорректный id", http.StatusBadRequest)
		return
	}
	var input struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Невалидный JSON", http.StatusBadRequest)
		return
	}
	if len(input.Password) < 6 {
		http.Error(w, "Пароль должен быть не короче 6 символов", http.StatusBadRequest)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Ошибка хэширования пароля", http.StatusInternalServerError)
		return
	}
	res, err := database.DB.Exec(`UPDATE users SET password = $1 WHERE id = $2`, string(hash), id)
	if err != nil {
		http.Error(w, "Ошибка обновления", http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		http.Error(w, "Пользователь не найден", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"id": id, "message": "Пароль обновлён"})
}

// DELETE /api/users/{id} — удалить учётную запись ученика/учителя (только admin).
// Удаляем зависимые строки в транзакции, чтобы не нарушить внешние ключи.
func DeleteUserHandler(w http.ResponseWriter, r *http.Request) {
	admin, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Некорректный id", http.StatusBadRequest)
		return
	}
	if id == admin.ID {
		http.Error(w, "Нельзя удалить самого себя", http.StatusBadRequest)
		return
	}

	var role string
	err = database.DB.QueryRow(`SELECT role FROM users WHERE id = $1`, id).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Пользователь не найден", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Ошибка запроса к БД", http.StatusInternalServerError)
		return
	}
	if role == "admin" {
		http.Error(w, "Нельзя удалить администратора", http.StatusForbidden)
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		http.Error(w, "Не удалось начать транзакцию", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Порядок важен: сначала дети, потом родитель (из-за внешних ключей).
	stmts := []string{
		`DELETE FROM sessions WHERE user_id = $1`,
		`DELETE FROM submissions WHERE student_id = $1`,
		`DELETE FROM submissions WHERE homework_id IN (SELECT id FROM homework WHERE teacher_id = $1)`,
		`DELETE FROM requests WHERE student_id = $1`,
		`UPDATE requests SET teacher_id = NULL WHERE teacher_id = $1`,
		`DELETE FROM homework WHERE teacher_id = $1`,
		`DELETE FROM schedule WHERE teacher_id = $1`,
		`DELETE FROM users WHERE id = $1`,
	}
	for _, q := range stmts {
		if _, err := tx.Exec(q, id); err != nil {
			http.Error(w, "Ошибка удаления связанных данных", http.StatusInternalServerError)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, "Ошибка фиксации транзакции", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"id": id, "message": "Учётная запись удалена"})
}
