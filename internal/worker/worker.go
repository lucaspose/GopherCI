package worker

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/lucaspose/goci/internal/crypto"
	"github.com/lucaspose/goci/internal/db/repository"
	"github.com/lucaspose/goci/internal/models"
)

const (
	defaultArtifactsDir            = "artifacts"
	defaultMaxArtifactZipSizeMB    = int64(25)
	defaultMaxArtifactSourceSizeMB = int64(200)
	bytesPerMegabyte               = int64(1024 * 1024)
)

var (
	workerDebugLogsEnabled    = strings.EqualFold(os.Getenv("LOG_LEVEL"), "DEBUG")
	errArtifactSourceTooLarge = errors.New("artifact source directory too large")
	errArtifactZipTooLarge    = errors.New("artifact zip too large")
)

type Worker struct {
	JobQueue                   chan models.Job
	JobRepo                    repository.JobsRepository
	SSHkeyRepo                 repository.SSHKeyRepository
	EncryptionKey              string
	Timeout                    time.Duration
	ArtifactsDir               string
	MaxArtifactZipSizeBytes    int64
	MaxArtifactSourceSizeBytes int64
}

func NewWorker(jobRepo repository.JobsRepository, sshKeyRepo repository.SSHKeyRepository, encryptionKey string, timeout time.Duration) *Worker {
	artifactsDir := strings.TrimSpace(os.Getenv("ARTIFACTS_DIR"))
	if artifactsDir == "" {
		artifactsDir = defaultArtifactsDir
	}

	return &Worker{
		JobQueue:                   make(chan models.Job, 100),
		JobRepo:                    jobRepo,
		SSHkeyRepo:                 sshKeyRepo,
		EncryptionKey:              encryptionKey,
		Timeout:                    timeout,
		ArtifactsDir:               artifactsDir,
		MaxArtifactZipSizeBytes:    parseArtifactSizeBytes("ARTIFACT_MAX_ZIP_SIZE_MB", defaultMaxArtifactZipSizeMB),
		MaxArtifactSourceSizeBytes: parseArtifactSizeBytes("ARTIFACT_MAX_SOURCE_SIZE_MB", defaultMaxArtifactSourceSizeMB),
	}
}

func parseArtifactSizeBytes(envKey string, fallbackMB int64) int64 {
	raw := strings.TrimSpace(os.Getenv(envKey))
	if raw == "" {
		return fallbackMB * bytesPerMegabyte
	}
	mbValue, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || mbValue <= 0 {
		log.Printf("[WARN] invalid %s value %q, using default %d MB", envKey, raw, fallbackMB)
		return fallbackMB * bytesPerMegabyte
	}
	return mbValue * bytesPerMegabyte
}

func (w *Worker) Start(n int) {
	for range n {
		go func() {
			for job := range w.JobQueue {
				w.executeJob(job)
			}
		}()
	}
}

func (w *Worker) executeJob(job models.Job) {
	if err := w.JobRepo.Create(context.Background(), &job); err != nil {
		log.Printf("[ERROR] failed to create job [%s]: %v", job.ID, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), w.Timeout)
	defer cancel()
	repoURL := job.CloneURL
	if repoURL == "" {
		log.Printf("[ERROR] repo url not found for job [%s]", job.ID)
		return
	}
	err := w.JobRepo.UpdateStatus(ctx, job.ID, models.JobRunning)
	if err != nil {
		log.Printf("[ERROR] failed to update status [%s]: %v", job.ID, err)
		return
	}
	dir, err := os.MkdirTemp("", "goci-*")
	if err != nil {
		log.Printf("[ERROR] failed to created temp folder [%s]: %v", job.ID, err)
		return
	}
	defer os.RemoveAll(dir)
	repoName := path.Base(repoURL)
	repoDir := filepath.Join(dir, strings.TrimSuffix(repoName, ".git"))
	cmdClone := exec.CommandContext(ctx, "git", "clone", repoURL)
	cmdClone.Env = append(jobEnv(), "GIT_TERMINAL_PROMPT=0")
	sshKeyPath := ""
	if job.SSHKeyID != "" {
		key, err := w.SSHkeyRepo.GetByID(ctx, job.SSHKeyID)
		if err != nil {
			log.Printf("[ERROR] failed to get ssh keys from id [%s]: %v", job.ID, err)
			return
		}
		dercryptedKey, err := crypto.Decrypt(key.PrivateKey, []byte(w.EncryptionKey))
		if err != nil {
			log.Printf("[ERROR] failed to get decrypted key [%s]: %v", job.ID, err)
			return
		}
		tempKeyPath, err := w.writeTempSSHKey(dercryptedKey)
		if err != nil {
			log.Printf("[ERROR] failed write ssh key in temp file [%s]: %v", job.ID, err)
			return
		}
		sshKeyPath = tempKeyPath
		defer os.Remove(tempKeyPath)
		cmdClone.Env = append(cmdClone.Env, fmt.Sprintf("GIT_SSH_COMMAND=ssh -i %s -o StrictHostKeyChecking=no", tempKeyPath))
	}
	cmdClone.Dir = dir
	outputClone, err := cmdClone.CombinedOutput()
	// The decrypted key is only needed for the clone: delete it before any
	// user-provided step runs so steps cannot read it.
	if sshKeyPath != "" {
		os.Remove(sshKeyPath)
	}
	if workerDebugLogsEnabled {
		log.Printf("[DEBUG] clone output for job [%s]: %s", job.ID, strings.TrimSpace(string(outputClone)))
	}
	logsClone := string(outputClone)
	job.Logs = append(job.Logs, logsClone)
	if err != nil {
		err := w.JobRepo.UpdateLogs(ctx, job.ID, job.Logs)
		if err != nil {
			log.Printf("[ERROR] failed to update logs [%s]: %v", job.ID, err)
			return
		}
		err = w.JobRepo.UpdateStatus(ctx, job.ID, models.JobFailed)
		if err != nil {
			log.Printf("[ERROR] failed to update status [%s]: %v", job.ID, err)
			return
		}
		log.Printf("[ERROR] job failed: ID: [%s], Status: [%s]", job.ID, models.JobFailed)
		return
	}
	for _, step := range job.Steps {
		commands := step.Cmd
		if len(commands) == 0 {
			log.Printf("[ERROR] job [%s] has no command", job.ID)
			return
		}
		program := commands[0]
		args := commands[1:]
		log.Printf("[INFO] step [%s] running: job [%s]", step.Name, job.ID)
		cmd := exec.CommandContext(ctx, program, args...)
		cmd.Dir = repoDir
		cmd.Env = jobEnv()
		output, err := cmd.CombinedOutput()
		if workerDebugLogsEnabled {
			log.Printf("[DEBUG] step [%s] output for job [%s]: %s", step.Name, job.ID, strings.TrimSpace(string(output)))
		}
		logs := string(output)
		job.Logs = append(job.Logs, logs)
		if err != nil {
			err := w.JobRepo.UpdateLogs(ctx, job.ID, job.Logs)
			if err != nil {
				log.Printf("[ERROR] failed to update logs [%s]: %v", job.ID, err)
				return
			}
			err = w.JobRepo.UpdateStatus(ctx, job.ID, models.JobFailed)
			if err != nil {
				log.Printf("[ERROR] failed to update status [%s]: %v", job.ID, err)
				return
			}
			log.Printf("[ERROR] step [%s] failed: job [%s]", step.Name, job.ID)
			return
		}
		err = w.JobRepo.UpdateLogs(ctx, job.ID, job.Logs)
		if err != nil {
			log.Printf("[ERROR] failed to update logs [%s]: %v", job.ID, err)
			return
		}
	}

	artifactPath, artifactErr := w.archiveBuildDirectory(job.ID, repoDir)
	if artifactPath != "" {
		log.Printf("[INFO] build artifact stored for job [%s]: %s", job.ID, artifactPath)
		job.Logs = append(job.Logs, fmt.Sprintf("build artifact: %s", artifactPath))
	}
	if artifactErr != nil {
		log.Printf("[WARN] build artifact skipped for job [%s]: %v", job.ID, artifactErr)
		job.Logs = append(job.Logs, fmt.Sprintf("build artifact skipped: %v", artifactErr))
	}
	if artifactPath != "" || artifactErr != nil {
		err = w.JobRepo.UpdateLogs(ctx, job.ID, job.Logs)
		if err != nil {
			log.Printf("[ERROR] failed to update logs [%s]: %v", job.ID, err)
			return
		}
	}

	err = w.JobRepo.UpdateStatus(ctx, job.ID, models.JobSuccess)
	if err != nil {
		log.Printf("[ERROR] failed to update status [%s]: %v", job.ID, err)
		return
	}
	log.Printf("[INFO] job success: ID: [%s], Status: [%s]", job.ID, models.JobSuccess)
}

func (w *Worker) writeTempSSHKey(privateKey string) (string, error) {
	tmpKey, err := os.CreateTemp("", "goci-key-*")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	defer tmpKey.Close()
	if _, err = tmpKey.Write([]byte(privateKey)); err != nil {
		return "", fmt.Errorf("writing ssh key in temp file: %w", err)
	}
	if err = os.Chmod(tmpKey.Name(), 0600); err != nil {
		return "", fmt.Errorf("chmod ssh key: %w", err)
	}
	return tmpKey.Name(), nil
}

func (w *Worker) archiveBuildDirectory(jobID, repoDir string) (string, error) {
	repoInfo, err := os.Stat(repoDir)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("stat repo directory: %w", err)
	}
	if !repoInfo.IsDir() {
		return "", fmt.Errorf("repo path is not a directory")
	}

	if err := os.MkdirAll(w.ArtifactsDir, 0755); err != nil {
		return "", fmt.Errorf("create artifacts directory: %w", err)
	}

	artifactName := fmt.Sprintf("%s-build.zip", jobID)
	artifactPath := filepath.Join(w.ArtifactsDir, artifactName)
	temporaryZipPath := artifactPath + ".tmp"

	if err := w.zipDirectory(repoDir, temporaryZipPath); err != nil {
		_ = os.Remove(temporaryZipPath)
		return "", err
	}

	zipInfo, err := os.Stat(temporaryZipPath)
	if err != nil {
		_ = os.Remove(temporaryZipPath)
		return "", fmt.Errorf("stat artifact zip: %w", err)
	}
	if zipInfo.Size() > w.MaxArtifactZipSizeBytes {
		_ = os.Remove(temporaryZipPath)
		return "", fmt.Errorf("%w: generated zip size %d bytes exceeds limit %d bytes", errArtifactZipTooLarge, zipInfo.Size(), w.MaxArtifactZipSizeBytes)
	}

	if err := os.Rename(temporaryZipPath, artifactPath); err != nil {
		_ = os.Remove(temporaryZipPath)
		return "", fmt.Errorf("store artifact zip: %w", err)
	}

	absArtifactPath, err := filepath.Abs(artifactPath)
	if err != nil {
		return artifactPath, nil
	}
	return absArtifactPath, nil
}

func (w *Worker) zipDirectory(sourceDir, zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("create artifact zip: %w", err)
	}
	zipWriter := zip.NewWriter(zipFile)

	var totalSourceSize int64
	walkErr := filepath.WalkDir(sourceDir, func(currentPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}

		fileInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if !fileInfo.Mode().IsRegular() {
			return nil
		}

		totalSourceSize += fileInfo.Size()
		if totalSourceSize > w.MaxArtifactSourceSizeBytes {
			return fmt.Errorf("%w: source size exceeded limit %d bytes", errArtifactSourceTooLarge, w.MaxArtifactSourceSizeBytes)
		}

		relPath, err := filepath.Rel(sourceDir, currentPath)
		if err != nil {
			return err
		}
		entryName := filepath.ToSlash(relPath)

		header, err := zip.FileInfoHeader(fileInfo)
		if err != nil {
			return err
		}
		header.Name = entryName
		header.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		file, err := os.Open(currentPath)
		if err != nil {
			return err
		}

		_, copyErr := io.Copy(writer, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		return nil
	})

	closeZipErr := zipWriter.Close()
	closeFileErr := zipFile.Close()

	if walkErr != nil {
		return fmt.Errorf("zip build directory: %w", walkErr)
	}
	if closeZipErr != nil {
		return fmt.Errorf("close artifact zip writer: %w", closeZipErr)
	}
	if closeFileErr != nil {
		return fmt.Errorf("close artifact zip file: %w", closeFileErr)
	}
	return nil
}

// jobEnv returns the environment given to git and pipeline steps.
// It is built from an allowlist so that server secrets (JWT_SECRET,
// ENCRYPTION_KEY, DATABASE_URL, ...) never reach user-provided commands.
func jobEnv() []string {
	env := []string{"CI=true"}
	for _, key := range []string{"PATH", "HOME", "LANG", "TMPDIR", "GOPATH", "GOCACHE", "GOMODCACHE"} {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return env
}
