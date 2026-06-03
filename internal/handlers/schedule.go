package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/K1viyt/school-homework/internal/database"
)

type Schedule struct {
	ID         int     `json:"id"`
	ClassName  string  `json:"class_name"`
	WeekParity string  `json:"week_parity"` // "odd" | "even"
	DayOfWeek  int     `json:"day_of_week"` // 1..6 (Пн..Сб)
	LessonNum  int     `json:"lesson_num"`  // 1..10
	Subject    string  `json:"subject"`
	TeacherID  int     `json:"teacher_id"`
	Room       *string `json:"room"` // может быть NULL
	CreatedAt  string  `json:"created_at"`
}

type ScheduleInput struct {
	ClassName  string  `json:"class_name"`
	WeekParity string  `json:"week_parity"` // "odd" | "even"
	DayOfWeek  int     `json:"day_of_week"` // 1..6 (Пн..Сб)
	LessonNum  int     `json:"lesson_num"`  // 1..10
	Subject    string  `json:"subject"`
	TeacherID  int     `json:"teacher_id"`
	Room       *string `json:"room"` // может быть NULL
}

func validateScheduleInput(in ScheduleInput) error {
	if in.WeekParity != "odd" && in.WeekParity != "even" {
		return fmt.Errorf("week_parity должен быть 'odd' или 'even'")
	}
	if in.DayOfWeek < 1 || in.DayOfWeek > 6 {
		return fmt.Errorf("В неделе может быть только 1...6 дней")
	}
	if in.LessonNum < 1 || in.LessonNum > 10 {
		return fmt.Errorf("Номер урока может быть от 1...10")
	}
	if in.Room != nil && *in.Room == "" {
		return fmt.Errorf("Вы ввели пустой кабинет")
	}
	if in.Subject == "" {
		return fmt.Errorf("Вы ввели недопустимое значение для название предмета это обязательное поле и оно не может быть пустым")
	}
	if in.TeacherID <= 0 {
		return fmt.Errorf("ID учителя дожет быть 1.....")
	}
	if in.ClassName == "" {
		return fmt.Errorf("Класс должен быть указан")
	}
	return nil
}

func GetScheduleHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Разрешен только GET", http.StatusMethodNotAllowed)
		return
	}
	user, err := getUserFromToken(r)
	if err != nil {
		http.Error(w, "User не найден", http.StatusUnauthorized)
		return
	}
	week := r.URL.Query().Get("week")
	if week != "odd" && week != "even" {
		http.Error(w, "week должен быть 'odd' или 'even'", http.StatusBadRequest)
		return
	}

	// mine=1 → расписание самого учителя (его уроки во всех классах).
	// иначе → расписание конкретного класса (?class=...).
	mine := r.URL.Query().Get("mine") == "1"
	var rows *sql.Rows
	if mine {
		rows, err = database.DB.Query(`
			SELECT id, class_name, week_parity, day_of_week, lesson_num, subject, teacher_id, room
			FROM schedule
			WHERE teacher_id = $1 AND week_parity = $2
			ORDER BY day_of_week, lesson_num
		`, user.ID, week)
	} else {
		className := normClass(r.URL.Query().Get("class"))
		if className == "" {
			http.Error(w, "Класс пуст", http.StatusBadRequest)
			return
		}
		rows, err = database.DB.Query(`
			SELECT id, class_name, week_parity, day_of_week, lesson_num, subject, teacher_id, room
			FROM schedule
			WHERE class_name = $1 AND week_parity = $2
			ORDER BY day_of_week, lesson_num
		`, className, week)
	}
	if err != nil {
		http.Error(w, "Ошибка запроса к базе данных", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	schedule := []Schedule{}
	for rows.Next() {
		var sh Schedule
		if err := rows.Scan(&sh.ID, &sh.ClassName, &sh.WeekParity, &sh.DayOfWeek, &sh.LessonNum, &sh.Subject, &sh.TeacherID, &sh.Room); err != nil {
			http.Error(w, "Ошибка чтения строки", http.StatusInternalServerError)
			return
		}
		schedule = append(schedule, sh)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "Ошибка итерации по результатам", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(schedule)
}

func CreateScheduleHandler(w http.ResponseWriter, r *http.Request) {
	user, err := getUserFromToken(r)
	if err != nil {
		http.Error(w, "Пользователь по токену не обнаружен", http.StatusUnauthorized)
		return
	}
	if user.Role != "admin" {
		http.Error(w, "Доступ разрешон пользователю с правами администратора", http.StatusForbidden)
		return
	}
	var input ScheduleInput
	err = json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		http.Error(w, "Был передан невалидный JSON", http.StatusBadRequest)
		return
	}
	input.ClassName = normClass(input.ClassName)
	err = validateScheduleInput(input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var id int64

	err = database.DB.QueryRow(`INSERT INTO schedule(class_name, week_parity, day_of_week, lesson_num, subject, teacher_id, room)
     VALUES($1, $2, $3, $4, $5, $6, $7) RETURNING id`, input.ClassName, input.WeekParity, input.DayOfWeek, input.LessonNum, input.Subject, input.TeacherID, input.Room).Scan(&id)
	if err != nil {
		http.Error(w, "Ошибка записи", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"id":      id,
		"message": "расписание создано",
	})
}

func UpdateScheduleHandler(w http.ResponseWriter, r *http.Request) {
	user, err := getUserFromToken(r)
	if err != nil {
		http.Error(w, "Пользователь по токену не обнаружен", http.StatusUnauthorized)
		return
	}
	var input ScheduleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Был передан невалидный JSON", http.StatusBadRequest)
		return
	}
	input.ClassName = normClass(input.ClassName)
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Неудалось извлечь id", http.StatusNotFound)
		return
	}
	role := user.Role

	switch role {
	case "admin":
		result, err := database.DB.Exec(`UPDATE schedule SET class_name=$1, week_parity=$2, day_of_week=$3, lesson_num=$4, subject=$5, teacher_id=$6, room=$7 WHERE id=$8`, input.ClassName, input.WeekParity, input.DayOfWeek, input.LessonNum, input.Subject, input.TeacherID, input.Room, id)
		if err != nil {
			http.Error(w, "Соеденение с БД потеряно", http.StatusInternalServerError)
			return
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			http.Error(w, "Запись не найдена", http.StatusNotFound)
			return
		}
	case "teacher":
		result, err := database.DB.Exec(`UPDATE schedule SET room=$1 WHERE teacher_id=$2 AND id=$3
    `, input.Room, user.ID, id)
		if err != nil {
			http.Error(w, "Соеденение с БД потеряно", http.StatusInternalServerError)
			return
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			http.Error(w, "Запись не найдена", http.StatusNotFound)
			return
		}
	default:
		http.Error(w, "Доступ к изменения в расписании вам запрещен", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"id":      id,
		"message": "расписание обновлено",
	})
}

func DeleteScheduleHandler(w http.ResponseWriter, r *http.Request) {
	user, err := getUserFromToken(r)
	if err != nil {
		http.Error(w, "Пользователь не найден", http.StatusNotFound)
		return
	}
	role := user.Role
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Неудалось извлечь id", http.StatusNotFound)
		return
	}

	if role == "admin" {
		result, err := database.DB.Exec(`DELETE FROM schedule WHERE id=$1`, id)
		if err != nil {
			http.Error(w, "Потеряно соеденение с БД на этапе DELETE", http.StatusInternalServerError)
			return
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			http.Error(w, "id По заданному URL не найден", http.StatusNotFound)
			return
		}
	} else {
		http.Error(w, "Вам недоступна эта функция,за корректировками обратитесь к админу", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"message": "запись удалена", "id": id})

}
