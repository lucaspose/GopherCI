# GopherCI

A minimal CI/CD backend platform built in Go. GopherCI lets you create and run pipeline jobs — cloning repositories, executing build/test steps, and tracking results — all via a secure REST API.

---

## Features

- JWT authentication (signup, login, protected routes)
- Pipeline jobs with multiple steps
- Background worker system (goroutines + buffered queue)
- Git clone support (public repos + private repos via SSH keys)
- SSH private key encryption (AES-256-GCM)
- Job logs and status tracking (pending → running → success/failed)
- Rate limiting per IP
- Request logging middleware

---

## Requirements

- Go 1.22+
- PostgreSQL 16
- Docker (optional, for the database)
- Git (must be installed on the host)

---

## Installation

```bash
git clone https://github.com/lucaspose/goci.git
cd goci
go mod download
```

---

## Configuration

Create a `.env` file at the root of the project:

```env
DATABASE_URL=postgres://goci:goci@localhost:5432/goci?sslmode=disable
JWT_SECRET=your_jwt_secret_here
JWT_EXPIRY=15
JWT_REFRESH_EXPIRY=10080
ENCRYPTION_KEY=your_32_bytes_encryption_key_here
LOG_LEVEL=INFO
LOG_SAMPLE_SUCCESS_EVERY=20
```

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL connection string |
| `JWT_SECRET` | Secret used to sign JWT tokens |
| `JWT_EXPIRY` | Access token expiry in minutes |
| `JWT_REFRESH_EXPIRY` | Refresh token expiry in minutes (default: 10080 = 7 days) |
| `ENCRYPTION_KEY` | 32-byte key for AES-256-GCM encryption of SSH keys |
| `LOG_LEVEL` | `DEBUG`, `INFO`, `WARN`, or `ERROR` (default: `INFO`) |
| `LOG_SAMPLE_SUCCESS_EVERY` | Sample one successful `GET/HEAD` log every N requests (default: `20`) |

To generate a secure `ENCRYPTION_KEY`:

```bash
openssl rand -base64 32 | head -c 32
```

---

## Run with Docker

```bash
docker run --name goci-postgres \
  -e POSTGRES_USER=goci \
  -e POSTGRES_PASSWORD=goci \
  -e POSTGRES_DB=goci \
  -p 5432:5432 \
  -d postgres:16-alpine
```

---

## Start the server

```bash
go run cmd/server/main.go
```

The server starts on `:8080`.

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
  models/          Domain models (User, Job, SSHKey)
  worker/          Background job execution
migrations/        SQL migration files
```

---

## License

MIT
