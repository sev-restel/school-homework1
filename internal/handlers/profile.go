package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/K1viyt/school-homework/internal/database"
)

func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Разрешён только GET", http.StatusMethodNotAllowed)
		return
	}

	user, err := getUserFromToken(r)
	if err != nil {
		http.Error(w, "Пользователь по данному токену не найден", http.StatusUnauthorized)
		return
	}

	var fullName, username, role string
	var class, subject sql.NullString

	err = database.DB.QueryRow(
		`SELECT full_name, username, role, class, subject FROM users WHERE id = $1`,
		user.ID,
	).Scan(&fullName, &username, &role, &class, &subject)
	if err != nil {
		http.Error(w, "Профиль не найден", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id":        user.ID,
		"full_name": fullName,
		"username":  username,
		"role":      role,
		"class":     class.String,
		"subject":   subject.String,
	})
}
