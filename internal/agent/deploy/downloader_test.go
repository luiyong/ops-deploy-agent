package deploy

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloaderDownloadScenarios(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("jar-content"))
		}))
		defer server.Close()

		destDir := t.TempDir()
		path, err := (Downloader{}).Download(server.URL+"/api/jars/download/svc-a-1.0.jar", destDir)
		if err != nil {
			t.Fatalf("download: %v", err)
		}
		if filepath.Base(path) != "svc-a-1.0.jar" {
			t.Fatalf("unexpected filename: %s", path)
		}
		info, err := os.Stat(path)
		if err != nil || info.Size() <= 0 {
			t.Fatalf("expected downloaded file with content")
		}
	})

	t.Run("empty response cleans file", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
		defer server.Close()

		destDir := t.TempDir()
		_, err := (Downloader{}).Download(server.URL+"/api/jars/download/empty.jar", destDir)
		if err == nil {
			t.Fatalf("expected empty download to fail")
		}
		if _, statErr := os.Stat(filepath.Join(destDir, "empty.jar")); !os.IsNotExist(statErr) {
			t.Fatalf("expected no residual file, got %v", statErr)
		}
	})

	t.Run("404 returns error", func(t *testing.T) {
		server := httptest.NewServer(http.NotFoundHandler())
		defer server.Close()

		destDir := t.TempDir()
		_, err := (Downloader{}).Download(server.URL+"/api/jars/download/missing.jar", destDir)
		if err == nil {
			t.Fatalf("expected 404 download to fail")
		}
	})
}
