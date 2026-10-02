package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shekhar396/argocd-gitops-platform/internal/model"
)

type UserHandler struct {
	DB *pgxpool.Pool
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
