package worker

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/lucaspose/goci/internal/db/repository"
	"github.com/lucaspose/goci/internal/models"
	"github.com/lucaspose/goci/internal/crypto"
)

type Worker struct {
	JobQueue chan models.Job
	JobRepo  repository.JobsRepository
	SSHkeyRepo repository.SSHKeyRepository
	EncryptionKey string
	Timeout  time.Duration
}

func NewWorker(jobRepo repository.JobsRepository, sshKeyRepo repository.SSHKeyRepository, encryptionKey string, timeout time.Duration) *Worker {
	return &Worker{
		JobQueue: make(chan models.Job, 100),
		JobRepo:  jobRepo,
		SSHkeyRepo: sshKeyRepo,
		EncryptionKey: encryptionKey,
		Timeout:  timeout,
	}
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
		log.Printf("failed to create job [%s]: %v", job.ID, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), w.Timeout)
	defer cancel()
	if job.Repo == "" {
		err := w.JobRepo.UpdateStatus(ctx, job.ID, models.JobFailed)
		if err != nil {
			log.Printf("failed to update status [%s]: %v", job.ID, err)
			return
		}
		log.Printf("job [%s] has no repository", job.ID)
		return
	}
	err := w.JobRepo.UpdateStatus(ctx, job.ID, models.JobRunning)
	if err != nil {
		log.Printf("failed to update status [%s]: %v", job.ID, err)
		return
	}
	dir, err := os.MkdirTemp("", "goci-*")
	if err != nil {
		log.Printf("failed to created temp folder [%s]: %v", job.ID, err)
		return
	}
	defer os.RemoveAll(dir)
	repoName := path.Base(job.Repo)
	repoDir := filepath.Join(dir, strings.TrimSuffix(repoName, ".git"))
	cmdClone := exec.CommandContext(ctx, "git", "clone", job.Repo)
	cmdClone.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if job.SSHKeyID != "" {
		key, err := w.SSHkeyRepo.GetByID(ctx, job.SSHKeyID)
		if err != nil {
			log.Printf("failed to get ssh keys from id [%s]: %v", job.ID, err)
			return
		}
		dercryptedKey, err := crypto.Decrypt(key.PrivateKey, []byte(w.EncryptionKey))
		if err != nil {
			log.Printf("failed to get decrypted key [%s]: %v", job.ID, err)
			return
		}
		tempKeyPath, err := w.writeTempSSHKey(dercryptedKey)
		if err != nil {
			log.Printf("failed write ssh key in temp file [%s]: %v", job.ID, err)
			return
		}
		defer os.Remove(tempKeyPath)
		cmdClone.Env = append(cmdClone.Env, fmt.Sprintf("GIT_SSH_COMMAND=ssh -i %s -o StrictHostKeyChecking=no", tempKeyPath))
	}
	cmdClone.Dir = dir
	outputClone, err := cmdClone.CombinedOutput()
	log.Printf("%s", outputClone)
	logsClone := string(outputClone)
	job.Logs = append(job.Logs, logsClone)
	if err != nil {
		err := w.JobRepo.UpdateLogs(ctx, job.ID, job.Logs)
		if err != nil {
			log.Printf("failed to update logs [%s]: %v", job.ID, err)
			return
		}
		err = w.JobRepo.UpdateStatus(ctx, job.ID, models.JobFailed)
		if err != nil {
			log.Printf("failed to update status [%s]: %v", job.ID, err)
			return
		}
		log.Printf("job failed: ID: [%s], Status: [%s]", job.ID, models.JobFailed)
		return
	}
	for _, step := range job.Steps {
		commands := step.Cmd
		if len(commands) == 0 {
			log.Printf("job [%s] has no command", job.ID)
			return
		}
		program := commands[0]
		args := commands[1:]
		log.Printf("step [%s] running: job [%s]", step.Name, job.ID)
		cmd := exec.CommandContext(ctx, program, args...)
		cmd.Dir = repoDir
		output, err := cmd.CombinedOutput()
		log.Printf("%s", output)
		logs := string(output)
		job.Logs = append(job.Logs, logs)
		if err != nil {
			err := w.JobRepo.UpdateLogs(ctx, job.ID, job.Logs)
			if err != nil {
				log.Printf("failed to update logs [%s]: %v", job.ID, err)
				return
			}
			err = w.JobRepo.UpdateStatus(ctx, job.ID, models.JobFailed)
			if err != nil {
				log.Printf("failed to update status [%s]: %v", job.ID, err)
				return
			}
			log.Printf("step [%s] failed: job [%s]", step.Name, job.ID)
			return
		}
		err = w.JobRepo.UpdateLogs(ctx, job.ID, job.Logs)
		if err != nil {
			log.Printf("failed to update logs [%s]: %v", job.ID, job.Logs)
			return
		}
	}
	err = w.JobRepo.UpdateStatus(ctx, job.ID, models.JobSuccess)
	if err != nil {
		log.Printf("failed to update status [%s]: %v", job.ID, err)
		return
	}
	log.Printf("job success: ID: [%s], Status: [%s]", job.ID, models.JobSuccess)
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
