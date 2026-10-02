package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shekhar396/argocd-gitops-platform/internal/model"
)

type UserHandler struct {
	DB *pgxpool.Pool
}

func userID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid user ID")
	}
	return id, nil
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(), `SELECT id, name, email, created_at FROM users ORDER BY id`)
	if err != nil {
		http.Error(w, "failed to list users", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	users := make([]model.User, 0)
	for rows.Next() {
		var user model.User
		if err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt); err != nil {
			http.Error(w, "failed to list users", http.StatusInternalServerError)
			return
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "failed to list users", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := userID(r)
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	var user model.User
	err = h.DB.QueryRow(r.Context(), `
		SELECT id, name, email, created_at FROM users WHERE id = $1
	`, id).Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to get user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := userID(r)
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	var request model.UpdateUserRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid JSON request body", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "invalid JSON request body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(request.Name) == "" || strings.TrimSpace(request.Email) == "" {
		http.Error(w, "name and email are required", http.StatusBadRequest)
		return
	}

	var user model.User
	err = h.DB.QueryRow(r.Context(), `
		UPDATE users SET name = $1, email = $2 WHERE id = $3
		RETURNING id, name, email, created_at
	`, request.Name, request.Email, id).Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to update user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := userID(r)
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	result, err := h.DB.Exec(r.Context(), `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		http.Error(w, "failed to delete user", http.StatusInternalServerError)
		return
	}
	if result.RowsAffected() == 0 {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var request model.CreateUserRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid JSON request body", http.StatusBadRequest)
		return
	}
	// Require a single JSON value in the request body.
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "invalid JSON request body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(request.Name) == "" || strings.TrimSpace(request.Email) == "" {
		http.Error(w, "name and email are required", http.StatusBadRequest)
		return
	}

	var user model.User
	err := h.DB.QueryRow(r.Context(), `
		INSERT INTO users (name, email)
		VALUES ($1, $2)
		RETURNING id, name, email, created_at
	`, request.Name, request.Email).Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt)
	if err != nil {
		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}
