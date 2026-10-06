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
	"golang.org/x/time/rate"
)

func main() {
	// env variables
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
	refreshExpiryInt := 60 * 24 * 7
	if refreshExpiryRaw := os.Getenv("JWT_REFRESH_EXPIRY"); refreshExpiryRaw != "" {
		parsedRefreshExpiry, err := strconv.Atoi(refreshExpiryRaw)
		if err != nil {
			log.Fatal(err)
		}
		if parsedRefreshExpiry <= 0 {
			log.Fatal("JWT_REFRESH_EXPIRY must be positive")
		}
		refreshExpiryInt = parsedRefreshExpiry
	}
	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if encryptionKey == "" {
		log.Fatal("ENCRYPTION_KEY is not set")
	}
	if len([]byte(encryptionKey)) != 32 {
		log.Fatal("ENCRYPTION_KEY need to be 32 bytes length")
	}
	githubClientID := os.Getenv("GITHUB_CLIENT_ID")
	githubClientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
	githubRedirectURL := os.Getenv("GITHUB_REDIRECT_URL")
	githubEnabled := githubClientID != "" && githubClientSecret != "" && githubRedirectURL != ""
	if !githubEnabled {
		log.Println("GitHub OAuth disabled: set GITHUB_CLIENT_ID, GITHUB_CLIENT_SECRET and GITHUB_REDIRECT_URL to enable it")
	}
	rateLimit := envInt("RATE_LIMIT_RPS", 10)
	rateBurst := envInt("RATE_LIMIT_BURST", 20)

	// Secret
	accessExpiry := time.Duration(expiryInt) * time.Minute
	refreshExpiry := time.Duration(refreshExpiryInt) * time.Minute
	authService := auth.NewServiceWithRefresh(secret, accessExpiry, refreshExpiry)

	// Repository
	userRepo := repository.NewUserRepository(database)
	jobRepo := repository.NewJobRepository(database)
	sshRepo := repository.NewSSHKeyRepository(database)
	repoRepo := repository.NewRepositoryRepository(database)
	orgRepo := repository.NewOrganizationRepository(database)
	refreshTokenRepo := repository.NewRefreshTokenRepository(database)

	// Worker
	worker := worker.NewWorker(jobRepo, sshRepo, encryptionKey, 30*time.Second)
	worker.Start(3)

	// Handler
	userHandler := handler.NewUserHandler(userRepo)
	authHandler := handler.NewAuthHandler(userRepo, refreshTokenRepo, authService)
	jobHandler := handler.NewJobHandler(worker, jobRepo).WithRepositories(repoRepo, orgRepo)
	sshHandler := handler.NewSSHKeyHandler(sshRepo, encryptionKey)
	repoHandler := handler.NewRepositoryHandler(repoRepo, orgRepo)
	orgHandler := handler.NewOrganizationHandler(orgRepo)
	var githubHandler *handler.GitHubHandler
	if githubEnabled {
		githubHandler = handler.NewGitHubHandler(githubClientID, githubClientSecret, githubRedirectURL, userRepo, refreshTokenRepo, authService)
	}

	// Router
	router := api.NewRouter(&handler.Handlers{
		User:          userHandler,
		Auth:          authHandler,
		Job:           jobHandler,
		SshKey:        sshHandler,
		Repo:          repoHandler,
		Organizations: orgHandler,
		Github:        githubHandler,
	}, authService, rate.Limit(rateLimit), rateBurst)
	log.Println("server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}

// envInt reads a positive integer from the environment, or returns def when unset.
func envInt(key string, def int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		log.Fatalf("%s must be a positive integer", key)
	}
	return value
}
