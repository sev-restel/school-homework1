package handlers

import (
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"

	"github.com/K1viyt/school-homework/internal/database"
	"golang.org/x/crypto/bcrypt"
)

func genarateUsername(fullName string) string {
	// берём первое слово из полного имени (имя)
	parts := strings.Fields(fullName)
	// защита от пустой строки или строки только из пробелов
	base := "user"
	if len(parts) > 0 {
		base = strings.ToLower(parts[0])
	}
	// добавляем 4 случайных цифры
	suffix := fmt.Sprintf("%04d", rand.Intn(10000))
	return base + "_" + suffix

}

// Данный токен выдается user при успешной авторизации
func genaratToken() (string, error) {
	bytes := make([]byte, 32)
	_, err := crand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func RegistrHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Разрешон только POST", http.StatusMethodNotAllowed)
		return
	}
	var role string
	var className string
	var subjectName string
	//Достаем нужные значения из БД
	password := r.FormValue("password")
	fullName := r.FormValue("full_name")
	if fullName == "" {
		http.Error(w, "Полное имя обязательное поле для заполнения", http.StatusBadRequest)
		return
	}
	if len(password) < 6 {
		http.Error(w, "Пароль должен быть не короче 6 символов", http.StatusBadRequest)
		return
	}
	login := genarateUsername(fullName)
	//Защита для роли учителя
	teacherCode := os.Getenv("TEACHER_INVITE_CODE")
	adminCode := os.Getenv("ADMIN_INVITE_CODE")
	inviteCode := r.FormValue("invite_code")

	switch {
	case adminCode != "" && inviteCode == adminCode:
		role = "admin"
	case teacherCode != "" && inviteCode == teacherCode:
		role = "teacher"
	default:
		role = "student"
	}

	// Класс ученику при регистрации НЕ назначаем (ТЗ, п.3.1): он подаёт заявку
	// в класс, и класс проставляется только при принятии заявки (см. requests.go).
	// className остаётся пустым → ученик не видит задания, пока не принят (п.6.1).
	if role == "teacher" {
		subjectName = r.FormValue("subject")
		if subjectName == "" {
			http.Error(w, "Предмет обязателен для учителя", http.StatusBadRequest)
			return
		}
	}
	//Пишем пароль в hash
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Ошибка записи pasword в hash", http.StatusBadRequest)
		return
	}
	//Записываем данные в БД
	// string(hash): bcrypt отдаёт []byte, а pgx биндит []byte как bytea —
	// вставка в text-колонку password падает. Приводим к строке.
	_, err = database.DB.Exec(`INSERT INTO users(full_name,role,password,username,class,subject) VALUES($1,$2,$3,$4,$5,$6)`, fullName, role, string(hash), login, className, subjectName)
	if err != nil {
		http.Error(w, "Ошибка загрузки данных пользователя в базу", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message":  "Регистрация успешна",
		"username": login,
	})

}
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Разрешон только Post", http.StatusMethodNotAllowed)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	var storedHash string
	var role string
	var userID int
	err := database.DB.QueryRow(
		`SELECT password, role,id FROM users WHERE username=$1`, username,
	).Scan(&storedHash, &role, &userID)
	if err != nil {
		http.Error(w, "Пользователь не найден", http.StatusNotFound)
		return
	}
	err = bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password))
	if err != nil {
		http.Error(w, "Неверный пароль", http.StatusUnauthorized)
		return
	}
	token, err := genaratToken()
	if err != nil {
		http.Error(w, "Ошибка генерации token", http.StatusInternalServerError)
		return
	}
	_, err = database.DB.Exec(`INSERT INTO sessions(user_id,token) VALUES($1,$2)`, userID, token)
	if err != nil {
		http.Error(w, "Ошибка передачи данных сессии пользователя", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message":  "Вы успешно авторизованны",
		"username": username,
		"role":     role,
		"token":    token,
	})
}
func getUserFromToken(r *http.Request) (*User, error) {
	var user User
	// 1. Достать заголовок Authorization
	authHeader := r.Header.Get("Authorization")

	// 2. Убрать префикс "Bearer "
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		return nil, fmt.Errorf("токен отсутствует")
	}

	err := database.DB.QueryRow(`SELECT users.id,users.username,users.role FROM sessions JOIN users ON sessions.user_id = users.id WHERE sessions.token = $1 AND sessions.created_at > NOW() - INTERVAL '24 hours'`, token).Scan(&user.ID, &user.Username, &user.Role)
	if err != nil {

		return nil, fmt.Errorf("сессия не найдена или истекла")
	}
	return &user, nil

}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Разрешон только Post", http.StatusMethodNotAllowed)
		return
	}
	authHeader := r.Header.Get("Authorization")

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		http.Error(w, "токен отсутствует", http.StatusBadRequest)
		return
	}
	result, err := database.DB.Exec(`DELETE FROM sessions WHERE token = $1`, token)
	if err != nil {
		http.Error(w, "Ошибка при выходе", http.StatusInternalServerError)
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "Сессия не найдена", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Успешное завершение сеанса",
	})

}
