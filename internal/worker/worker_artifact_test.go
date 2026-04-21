package worker

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveBuildDirectory(t *testing.T) {
	t.Run("creates zip artifact from repo directory", func(t *testing.T) {
		repoDir := t.TempDir()
		artifactsDir := t.TempDir()

		if err := os.MkdirAll(filepath.Join(repoDir, "bin"), 0755); err != nil {
			t.Fatalf("mkdir bin: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoDir, "bin", "app"), []byte("binary-content"), 0644); err != nil {
			t.Fatalf("write artifact file: %v", err)
		}

		w := &Worker{
			ArtifactsDir:               artifactsDir,
			MaxArtifactZipSizeBytes:    10 * bytesPerMegabyte,
			MaxArtifactSourceSizeBytes: 10 * bytesPerMegabyte,
		}

		artifactPath, err := w.archiveBuildDirectory("job-1", repoDir)
		if err != nil {
			t.Fatalf("archiveBuildDirectory returned error: %v", err)
		}
		if artifactPath == "" {
			t.Fatal("expected non-empty artifact path")
		}

		if _, err := os.Stat(artifactPath); err != nil {
			t.Fatalf("expected artifact zip to exist: %v", err)
		}

		zipReader, err := zip.OpenReader(artifactPath)
		if err != nil {
			t.Fatalf("open zip artifact: %v", err)
		}
		defer zipReader.Close()

		found := false
		for _, file := range zipReader.File {
			if file.Name == "bin/app" {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("expected zip to contain file bin/app")
		}
	})

	t.Run("returns no artifact when repo directory does not exist", func(t *testing.T) {
		artifactsDir := t.TempDir()
		w := &Worker{
			ArtifactsDir:               artifactsDir,
			MaxArtifactZipSizeBytes:    10 * bytesPerMegabyte,
			MaxArtifactSourceSizeBytes: 10 * bytesPerMegabyte,
		}

		artifactPath, err := w.archiveBuildDirectory("job-2", "/nonexistent/path/that/does/not/exist")
		if err != nil {
			t.Fatalf("expected nil error when repo dir is missing, got: %v", err)
		}
		if artifactPath != "" {
			t.Fatalf("expected empty artifact path, got %q", artifactPath)
		}
	})

	t.Run("rejects artifact when source directory size exceeds limit", func(t *testing.T) {
		repoDir := t.TempDir()
		artifactsDir := t.TempDir()

		if err := os.WriteFile(filepath.Join(repoDir, "bundle.bin"), make([]byte, 2048), 0644); err != nil {
			t.Fatalf("write build file: %v", err)
		}

		w := &Worker{
			ArtifactsDir:               artifactsDir,
			MaxArtifactZipSizeBytes:    10 * bytesPerMegabyte,
			MaxArtifactSourceSizeBytes: 1024,
		}

		_, err := w.archiveBuildDirectory("job-3", repoDir)
		if err == nil {
			t.Fatal("expected error for oversized source directory")
		}
		if !errors.Is(err, errArtifactSourceTooLarge) {
			t.Fatalf("expected errArtifactSourceTooLarge, got: %v", err)
		}
	})
}
