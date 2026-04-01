// gameserver-agent runs on each game VPS and exposes an HTTP API for the Lambda
// to trigger graceful shutdown: stop the container, archive the save, upload to S3.
package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	listenPort  = 8080
	saveTarPath = "/tmp/save-upload.tar.gz"
)

var (
	containerName  string
	saveDir        string
	shutdownMu     sync.Mutex
	shutdownCalled bool
)

func main() {
	secret := os.Getenv("AGENT_SECRET")
	containerName = os.Getenv("CONTAINER_NAME")
	saveDir = os.Getenv("SAVE_DIR")

	if secret == "" || containerName == "" || saveDir == "" {
		log.Fatal("AGENT_SECRET, CONTAINER_NAME, and SAVE_DIR env vars must all be set")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/shutdown", requireAuth(secret, handleShutdown))

	addr := fmt.Sprintf(":%d", listenPort)
	log.Printf("gameserver agent listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

func requireAuth(secret string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+secret {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

type shutdownRequest struct {
	UploadURL string `json:"upload_url"`
}

func handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	shutdownMu.Lock()
	if shutdownCalled {
		shutdownMu.Unlock()
		http.Error(w, "shutdown already in progress", http.StatusConflict)
		return
	}
	shutdownCalled = true
	shutdownMu.Unlock()

	var req shutdownRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.UploadURL == "" {
		http.Error(w, "upload_url is required", http.StatusBadRequest)
		return
	}

	log.Println("shutdown: stopping container...")
	if out, err := exec.Command("docker", "stop", "--time", "30", containerName).CombinedOutput(); err != nil {
		outStr := string(out)
		// If the container isn't running, proceed — the save files are still there.
		if strings.Contains(outStr, "No such container") || strings.Contains(outStr, "is not running") {
			log.Printf("container not running, proceeding with archive: %s", outStr)
		} else {
			log.Printf("docker stop output: %s", outStr)
			http.Error(w, fmt.Sprintf("failed to stop container: %v", err), http.StatusInternalServerError)
			return
		}
	}
	log.Println("shutdown: container stopped")

	log.Println("shutdown: archiving save directory...")
	if err := createTarGz(saveDir, saveTarPath); err != nil {
		http.Error(w, fmt.Sprintf("failed to archive save: %v", err), http.StatusInternalServerError)
		return
	}
	log.Println("shutdown: archive created")

	log.Println("shutdown: uploading save to S3...")
	if err := uploadFile(saveTarPath, req.UploadURL); err != nil {
		http.Error(w, fmt.Sprintf("failed to upload save: %v", err), http.StatusInternalServerError)
		return
	}
	log.Println("shutdown: save uploaded successfully")

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

func createTarGz(srcDir, destPath string) error {
	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create archive file: %w", err)
	}

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	walkErr := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		hdr, err := tar.FileInfoHeader(info, "")
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, readErr := os.Readlink(path)
			if readErr != nil {
				return fmt.Errorf("read symlink %s: %w", path, readErr)
			}
			hdr, err = tar.FileInfoHeader(info, linkTarget)
		}
		if err != nil {
			return err
		}
		hdr.Name = rel

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(tw, file)
		return err
	})

	// Close in order: tar → gzip → file. Errors here mean a corrupt archive,
	// so capture the first one even if Walk itself succeeded.
	if err := tw.Close(); err != nil && walkErr == nil {
		walkErr = fmt.Errorf("close tar writer: %w", err)
	}
	if err := gw.Close(); err != nil && walkErr == nil {
		walkErr = fmt.Errorf("close gzip writer: %w", err)
	}
	if err := f.Close(); err != nil && walkErr == nil {
		walkErr = fmt.Errorf("close archive file: %w", err)
	}
	return walkErr
}

func uploadFile(filePath, uploadURL string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat archive: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, uploadURL, f)
	if err != nil {
		return fmt.Errorf("create upload request: %w", err)
	}
	req.ContentLength = stat.Size()
	req.Header.Set("Content-Type", "application/octet-stream")

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("upload request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload returned %d: %s", resp.StatusCode, body)
	}
	return nil
}
