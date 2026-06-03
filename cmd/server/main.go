package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"

	"github.com/K1viyt/school-homework/internal/database"
	"github.com/K1viyt/school-homework/internal/handlers"
)

func main() {
	// ИСПРАВЛЕНИЕ БАГА #3:
	// Создаём папку uploads при старте сервера, если её нет.
	// os.MkdirAll — создаёт папку и все промежуточные папки в пути.
	// 0755 — права доступа: владелец может читать/писать/выполнять,
	//         остальные — только читать и "заходить" в папку.
	// Если папка уже существует — ошибки не будет, всё нормально.
	err := os.MkdirAll("uploads", 0755)
	if err != nil {
		log.Fatal("Не удалось создать папку uploads: ", err)
	}
	// Пытаемся подгрузить .env, а если его нет — password.env.
	// Обе ошибки молча игнорируем: если файла нет, переменные могут быть заданы в окружении.
	if err := godotenv.Load(); err != nil {
		_ = godotenv.Load("password.env")
	}

	// Диагностика: видит ли процесс invite-коды и DSN (значения НЕ печатаем).
	// strings.TrimSpace — чтобы пробелы не выглядели как "код задан".
	log.Printf("ENV check → DATABASE_URL:%t  TEACHER_INVITE_CODE:%t (len=%d)  ADMIN_INVITE_CODE:%t (len=%d)",
		strings.TrimSpace(os.Getenv("DATABASE_URL")) != "",
		strings.TrimSpace(os.Getenv("TEACHER_INVITE_CODE")) != "", len(strings.TrimSpace(os.Getenv("TEACHER_INVITE_CODE"))),
		strings.TrimSpace(os.Getenv("ADMIN_INVITE_CODE")) != "", len(strings.TrimSpace(os.Getenv("ADMIN_INVITE_CODE"))),
	)

	database.Init()

	// Отдаём фронтенд из папки web/ на все неразмеченные пути.
	// Более специфичные маршруты ниже перехватывают /login, /upload и т.д.
	http.Handle("/", http.FileServer(http.Dir("web")))
	// Раздаём загруженные файлы, чтобы их можно было скачивать по ссылке
	// /uploads/<имя>. StripPrefix убирает "/uploads/" перед поиском файла на диске.
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))
	// Auth
	http.HandleFunc("POST /api/register", handlers.RegistrHandler)
	http.HandleFunc("POST /api/login", handlers.LoginHandler)
	http.HandleFunc("POST /api/logout", handlers.LogoutHandler)
	http.HandleFunc("GET /api/profile", handlers.ProfileHandler)

	// Homework (RESTful по ТЗ)
	http.HandleFunc("GET /api/homeworks", handlers.ListHomeworksHandler)
	http.HandleFunc("POST /api/homeworks", handlers.UploadHandler)
	http.HandleFunc("PATCH /api/homeworks/{id}", handlers.UpdateHomeworkHandler)
	http.HandleFunc("PUT /api/homeworks/{id}", handlers.ReplaceHomeworkHandler)
	http.HandleFunc("DELETE /api/homeworks/{id}", handlers.DeleteHomeworkHandler)

	// Schedule (нет в ТЗ — наша фича, но держим единый /api/* стиль)
	http.HandleFunc("GET /api/schedule", handlers.GetScheduleHandler)
	http.HandleFunc("POST /api/schedule", handlers.CreateScheduleHandler)
	http.HandleFunc("PATCH /api/schedule/{id}", handlers.UpdateScheduleHandler)
	http.HandleFunc("DELETE /api/schedule/{id}", handlers.DeleteScheduleHandler)

	http.HandleFunc("POST /api/submissions", handlers.CreateSubmissionHandler)
	http.HandleFunc("GET /api/submissions/{homework_id}", handlers.GetSubmissionsHandler)
	http.HandleFunc("PATCH /api/submissions/{id}/grade", handlers.GradeSubmissionHandler)
	http.HandleFunc("POST /api/requests", handlers.CreateRequestHandler)
	http.HandleFunc("GET /api/requests", handlers.GetRequestsHandler)
	http.HandleFunc("PATCH /api/requests/{id}", handlers.ApprovedHandler)
	http.HandleFunc("GET /api/grades", handlers.GetGradesHandler)
	http.HandleFunc("GET /api/grades/{student_id}", handlers.GetStudentGradesHandler)
	http.HandleFunc("GET /api/teachers", handlers.ListTeachersHandler)

	// Управление пользователями (только admin)
	http.HandleFunc("GET /api/users", handlers.ListUsersHandler)
	http.HandleFunc("PATCH /api/users/{id}/kick", handlers.KickFromClassHandler)
	http.HandleFunc("PATCH /api/users/{id}/password", handlers.ResetPasswordHandler)
	http.HandleFunc("DELETE /api/users/{id}", handlers.DeleteUserHandler)
	port := os.Getenv("PORT")
	
	if port == "" {
		port = "8080"
	}

	log.Printf("Сервер запущен на порту %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))

}
