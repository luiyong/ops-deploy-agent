package deploy

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

type Downloader struct{}

func (d Downloader) Download(downloadURL, destDir string) (string, error) {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("create workspace: %w", err)
	}

	parsed, err := url.Parse(downloadURL)
	if err != nil {
		return "", fmt.Errorf("parse download url: %w", err)
	}
	filename := filepath.Base(parsed.Path)
	targetPath := filepath.Join(destDir, filename)

	resp, err := http.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("download jar: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download jar: unexpected status %s", resp.Status)
	}

	file, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("create downloaded file: %w", err)
	}

	size, copyErr := io.Copy(file, resp.Body)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(targetPath)
		return "", fmt.Errorf("save downloaded jar: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(targetPath)
		return "", fmt.Errorf("close downloaded jar: %w", closeErr)
	}
	if size <= 0 {
		_ = os.Remove(targetPath)
		return "", fmt.Errorf("downloaded jar is empty")
	}
	if _, err := os.ReadFile(targetPath); err != nil {
		_ = os.Remove(targetPath)
		return "", fmt.Errorf("read downloaded jar: %w", err)
	}

	return targetPath, nil
}
