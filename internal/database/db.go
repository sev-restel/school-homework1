package database

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var DB *sql.DB

func Init() {
	db, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("Ошибка соеденения с BD: ", err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal("BD не отвечает: ", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS homework(
	id BIGSERIAL PRIMARY KEY,
	teacher_id INTEGER,
	filename TEXT NOT NULL,
	filepath TEXT NOT NULL,
	class_name TEXT,
	subject TEXT,
	description TEXT,
    uploaded_at TIMESTAMPTZ DEFAULT NOW())`)

	if err != nil {
		log.Fatal("Невозможно создать BD: ", err)
	}

	// Миграция: для уже существующих БД добавляем колонку класса, если её нет.
	_, err = db.Exec(`ALTER TABLE homework ADD COLUMN IF NOT EXISTS class_name TEXT`)
	if err != nil {
		log.Fatal("Невозможно добавить колонку class_name: ", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS users(
	id BIGSERIAL PRIMARY KEY,
	full_name TEXT NOT NULL,
	subject TEXT,
	class TEXT,
	username TEXT NOT NULL UNIQUE,
	password TEXT NOT NULL,
	role TEXT NOT NULL,
	created_at TIMESTAMPTZ DEFAULT NOW())`)
	if err != nil {
		log.Fatal("Невозможно созадть BD под users: ", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS sessions(id BIGSERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    token TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ DEFAULT NOW())`)
	if err != nil {
		log.Fatal("Невозможно созадть BD под sessions: ", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS schedule(
    id          BIGSERIAL PRIMARY KEY,
    class_name  TEXT    NOT NULL,
    week_parity TEXT    NOT NULL CHECK(week_parity IN ('odd','even')),
    day_of_week INTEGER NOT NULL CHECK(day_of_week BETWEEN 1 AND 6),  -- Пн..Сб
    lesson_num  INTEGER NOT NULL CHECK(lesson_num BETWEEN 1 AND 10),
    subject     TEXT    NOT NULL,
    teacher_id  INTEGER NOT NULL,
    room        TEXT,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    FOREIGN KEY (teacher_id) REFERENCES users(id),
    UNIQUE(class_name, week_parity, day_of_week, lesson_num)
);
`)
	if err != nil {
		log.Fatal("Невозможно созадть BD под schedule: ", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS submissions(
	id BIGSERIAL PRIMARY KEY,
	homework_id INTEGER NOT NULL,
	FOREIGN KEY (homework_id) REFERENCES homework(id),
	student_id INTEGER NOT NULL,
	FOREIGN KEY (student_id) REFERENCES users(id),
  	filepath  TEXT    NOT NULL,
  	grade INTEGER,
  	status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'graded')),
	submitted_at TIMESTAMPTZ DEFAULT NOW())`)
	if err != nil {
		log.Fatal("Невозможно созадть BD под submissions: ", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS requests(
    id          BIGSERIAL PRIMARY KEY,
    student_id  INTEGER NOT NULL,
    class_name  TEXT NOT NULL,
    teacher_id  INTEGER,
    status      TEXT NOT NULL DEFAULT 'pending'
                CHECK(status IN ('pending','accepted','rejected')),
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    FOREIGN KEY (student_id) REFERENCES users(id),
    FOREIGN KEY (teacher_id) REFERENCES users(id)
)`)
	if err != nil {
		log.Fatal("Невозможно создать таблицу requests: ", err)
	}

	DB = db

}
