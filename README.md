# GopherCI

[![CI](https://github.com/lucaspose/GopherCI/actions/workflows/ci.yml/badge.svg)](https://github.com/lucaspose/GopherCI/actions/workflows/ci.yml)
![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)
![License](https://img.shields.io/badge/license-MIT-blue)

A minimal CI/CD backend written in Go. GopherCI lets you create pipeline jobs that clone a repository, run build/test steps and keep track of their status, logs and artifacts — all through a secure REST API.

A terminal client is available in [GopherCI-CLI](https://github.com/lucaspose/GopherCI-CLI).

---

## How it works

```mermaid
flowchart LR
    C[Client / CLI] -->|REST + JWT| API[HTTP API]
    API --> DB[(PostgreSQL)]
    API -->|enqueue job| Q[[Job queue]]
    Q --> W1[Worker 1]
    Q --> W2[Worker 2]
    Q --> W3[Worker 3]
    W1 & W2 & W3 -->|git clone + run steps| R[Repository workspace]
    W1 & W2 & W3 -->|status, logs, artifacts| DB
    API -.->|live job updates| C
```

1. An authenticated user submits a job: a repository URL and a list of steps.
2. The API stores it as `pending` and pushes it to an in-memory queue.
3. A pool of worker goroutines picks it up, clones the repository (using an encrypted SSH key for private repos) and runs each step in order, under a global job timeout.
4. Status (`pending → running → success / failed`), logs and a zipped build artifact are saved and can be fetched — or followed live — through the API.

---

## Features

- **Auth** — signup / login with JWT access tokens and rotating refresh tokens, logout revocation
- **GitHub OAuth** — connect a GitHub account, list its organizations and repositories
- **Pipelines** — multi-step jobs executed by a background worker pool (goroutines + buffered queue)
- **Private repos** — SSH keys stored encrypted with AES-256-GCM, never returned by the API
- **Live updates** — job progress streamed with Server-Sent Events (`GET /jobs/stream`)
- **Artifacts** — build output zipped and downloadable per job
- **Organizations & repositories** management
- **Hardening** — per-IP rate limiting, structured request logging with sampling
- **Database migrations** applied automatically at startup

---

## Tech stack

Go 1.26 (`net/http`, no web framework) · PostgreSQL 16 · JWT (`golang-jwt`) · `golang.org/x/crypto` · Docker (local database)

---

## Getting started

### Requirements

- Go 1.26+
- PostgreSQL 16 (or Docker)
- Git installed on the host

### 1. Clone

```bash
git clone https://github.com/lucaspose/GopherCI.git
cd GopherCI
go mod download
```

### 2. Start PostgreSQL

```bash
docker run --name goci-postgres \
  -e POSTGRES_USER=goci \
  -e POSTGRES_PASSWORD=goci \
  -e POSTGRES_DB=goci \
  -p 5432:5432 \
  -d postgres:16-alpine
```

### 3. Configure

```bash
cp .env.example .env
```

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL connection string |
| `JWT_SECRET` | Secret used to sign JWT tokens |
| `JWT_EXPIRY` | Access token expiry in minutes |
| `JWT_REFRESH_EXPIRY` | Refresh token expiry in minutes (default: 10080 = 7 days) |
| `ENCRYPTION_KEY` | 32-byte key for AES-256-GCM encryption of SSH keys |
| `GITHUB_CLIENT_ID` / `GITHUB_CLIENT_SECRET` | Credentials of your GitHub OAuth app |
| `GITHUB_REDIRECT_URL` | OAuth callback URL (e.g. `http://localhost:8080/auth/github/callback`) |
| `ARTIFACTS_DIR` | Where job artifacts are stored (default: `artifacts`) |
| `LOG_LEVEL` | `DEBUG`, `INFO`, `WARN`, or `ERROR` (default: `INFO`) |
| `LOG_SAMPLE_SUCCESS_EVERY` | Sample one successful `GET/HEAD` log every N requests (default: `20`) |

To generate a secure `ENCRYPTION_KEY`:

```bash
openssl rand -base64 32 | head -c 32
```

### 4. Run

```bash
go run ./cmd/server
```

The server listens on `:8080` and applies the SQL migrations on startup.

### Tests

```bash
go test ./... -race
```

---

## API Documentation

All protected routes require the `Authorization: Bearer <token>` header.

---

### Auth

#### `POST /users` — Create a user

**Request:**
```json
{
  "email": "user@example.com",
  "password": "password123"
}
```

**Response `201`:**
```json
{
  "id": "uuid",
  "email": "user@example.com"
}
```

---

#### `POST /login` — Login

**Request:**
```json
{
  "email": "user@example.com",
  "password": "password123"
}
```

**Response `200`:**
```json
{
  "access_token": "eyJ...",
  "refresh_token": "9w2...",
  "expires_in": 900,
  "refresh_expires_in": 604800
}
```

#### `POST /auth/refresh` — Rotate token pair

**Request:**
```json
{
  "refresh_token": "9w2..."
}
```

**Response `200`:**
```json
{
  "access_token": "eyJ...",
  "refresh_token": "d8k...",
  "expires_in": 900,
  "refresh_expires_in": 604800
}
```

#### `POST /auth/logout` — Revoke refresh token

**Request:**
```json
{
  "refresh_token": "d8k..."
}
```

**Response `204`:** No content.

---

#### `GET /me` — Get current user 🔒

**Response `200`:**
```json
{
  "id": "uuid",
  "email": "user@example.com"
}
```

---

#### `DELETE /users/{id}` — Delete current user 🔒

**Response `204`:** No content.

> A user can only delete their own account.

---

### Jobs

#### `POST /jobs` — Create a job 🔒

**Request:**
```json
{
  "repo": "git@github.com:user/repo.git",
  "ssh_key_id": "uuid",
  "steps": [
    {
      "name": "build",
      "cmd": ["go", "build", "./..."]
    },
    {
      "name": "test",
      "cmd": ["go", "test", "./..."]
    }
  ]
}
```

> `ssh_key_id` is optional. Required only for private repositories.

**Response `202`:**
```json
"job created"
```

---

#### `GET /jobs` — List jobs 🔒

**Response `200`:**
```json
[
  {
    "id": "uuid",
    "repo": "git@github.com:user/repo.git",
    "steps": [...],
    "user_id": "uuid",
    "status": "success",
    "created_at": "2026-04-09T14:54:59Z",
    "logs": ["Cloning...", "Build output..."]
  }
]
```

---

#### `GET /jobs/{id}` — Get a job 🔒

**Response `200`:**
```json
{
  "id": "uuid",
  "repo": "git@github.com:user/repo.git",
  "steps": [...],
  "user_id": "uuid",
  "status": "success",
  "created_at": "2026-04-09T14:54:59Z",
  "logs": ["Cloning...", "Build output..."]
}
```

---

#### `DELETE /jobs/{id}` — Delete a job 🔒

**Response `204`:** No content.

---

### SSH Keys

#### `POST /ssh-keys` — Add an SSH key 🔒

**Request:**
```json
{
  "name": "my-key",
  "private_key": "-----BEGIN OPENSSH PRIVATE KEY-----\n..."
}
```

> The private key is encrypted with AES-256-GCM before being stored.

**Response `201`:** No content.

---

#### `GET /ssh-keys` — List SSH keys 🔒

**Response `200`:**
```json
[
  {
    "id": "uuid",
    "user_id": "uuid",
    "name": "my-key",
    "created_at": "2026-04-09T11:50:19Z"
  }
]
```

> The `private_key` field is never returned by the API.

---

#### `DELETE /ssh-keys/{id}` — Delete an SSH key 🔒

**Response `204`:** No content.

> A user can only delete their own SSH keys.

---

### Other endpoints

| Method | Route | Description |
|--------|-------|-------------|
| `GET` | `/jobs/stream` 🔒 | Live job updates (Server-Sent Events) |
| `GET` | `/jobs/{id}/artifact` 🔒 | Download the job's build artifact (zip) |
| `POST` / `GET` | `/organizations` 🔒 | Create / list organizations |
| `DELETE` | `/organizations/{id}` 🔒 | Delete an organization |
| `POST` / `GET` | `/organizations/{orgId}/repositories` 🔒 | Add / list repositories |
| `DELETE` | `/organizations/{orgId}/repositories/{id}` 🔒 | Remove a repository |
| `GET` | `/auth/github` | Start the GitHub OAuth flow |
| `GET` | `/auth/github/organizations`, `/auth/github/repositories` | List the connected account's GitHub orgs and repos |

---

## Job Status

| Status | Description |
|--------|-------------|
| `pending` | Job created, waiting to be processed |
| `running` | Job is currently being executed |
| `success` | All steps completed successfully |
| `failed` | One or more steps failed |

---

## Architecture

```
cmd/
  server/
    main.go
internal/
  api/
    handler/       HTTP handlers
    middleware/    Auth, logging, rate limiting
    response/      JSON response helpers
    context/       Context keys
  auth/            JWT service
  crypto/          AES-256-GCM encryption
  db/
    repository/    PostgreSQL repositories
  models/          Domain models (User, Job, SSHKey, Organization, Repository)
  worker/          Background job execution
migrations/        SQL migration files
```

---

## License

[MIT](LICENSE)
