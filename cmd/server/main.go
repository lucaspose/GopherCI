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
	"github.com/lucaspose/goci/internal/worker"
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
	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if encryptionKey == "" {
		log.Fatal("ENCRYPTION_KEY is not set")
	}
	if len([]byte(encryptionKey)) != 32 {
		log.Fatal("ENCRYPTION_KEY need to be 32 bytes length")
	}
	// Secret
	expiry := time.Duration(expiryInt) * time.Minute
	authService := auth.NewService(secret, expiry)

	// Repository
	userRepo := repository.NewUserRepository(database)
	jobRepo := repository.NewJobRepository(database)
	sshRepo := repository.NewSSHKeyRepository(database)

	// Worker
	worker := worker.NewWorker(jobRepo, sshRepo, encryptionKey, 30*time.Second)
	worker.Start(3)

	// Handler
	userHandler := handler.NewUserHandler(userRepo)
	authHandler := handler.NewAuthHandler(userRepo, authService)
	jobHandler := handler.NewJobHandler(worker, jobRepo)
	sshHandler := handler.NewSSHKeyHandler(sshRepo, encryptionKey)

	// Router
	router := api.NewRouter(&handler.Handlers{
		User:   userHandler,
		Auth:   authHandler,
		Job:    jobHandler,
		SshKey: sshHandler,
	}, authService)
	log.Println("server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}
