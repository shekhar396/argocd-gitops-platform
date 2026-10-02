package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/shekhar396/argocd-gitops-platform/internal/database"
	"github.com/shekhar396/argocd-gitops-platform/internal/handler"
)

type healthResponse struct {
	Status string `json:"status"`
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(healthResponse{
		Status: "ok",
	})
}

func main() {
	ctx := context.Background()

	db, err := database.Connect(ctx)
	if err != nil {
		log.Fatal("database connection failed: ", err)
	}
	defer db.Close()

	log.Println("database connection established")

	mux := http.NewServeMux()
	userHandler := &handler.UserHandler{
		DB: db,
	}

	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("POST /users", userHandler.Create)
	mux.HandleFunc("GET /users", userHandler.List)
	mux.HandleFunc("GET /users/{id}", userHandler.Get)
	mux.HandleFunc("PUT /users/{id}", userHandler.Update)
	mux.HandleFunc("DELETE /users/{id}", userHandler.Delete)

	log.Println("API starting on :8080")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
