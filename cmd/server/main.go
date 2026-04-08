package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/lucaspose/goci/internal/api"
	"github.com/lucaspose/goci/internal/api/handler"
	"github.com/lucaspose/goci/internal/auth"
	"github.com/lucaspose/goci/internal/db"
	"github.com/lucaspose/goci/internal/db/repository"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("no .env file found")
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}
	database, err := db.Open(databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Migrate(database); err != nil {
		log.Fatal(err)
	}
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET is not set")
	}
	jwtExpiry := os.Getenv("JWT_EXPIRY")
	if jwtExpiry == "" {
		log.Fatal("JWT_EXPIRY is not set")
	}
	expiryInt, err := strconv.Atoi(jwtExpiry)
	if err != nil {
		log.Fatal(err)
	}
	expiry := time.Duration(expiryInt) * time.Minute
	userRepo := repository.NewUserRepository(database)
	userHandler := handler.NewUserHandler(userRepo)
	authService := auth.NewService(secret, expiry)
	authHandler := handler.NewAuthHandler(userRepo, authService)
	router := api.NewRouter(&handler.Handlers{
		User: userHandler,
		Auth: authHandler,
	}, authService)
	log.Println("server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}