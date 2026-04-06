package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type JarMeta struct {
	Filename    string    `json:"filename"`
	ServiceName string    `json:"service_name"`
	Size        int64     `json:"size"`
	UploadTime  time.Time `json:"upload_time"`
}

type JarStore struct {
	jarDir   string
	metaFile string
	mu       sync.Mutex
}

func NewJarStore(jarDir string) (*JarStore, error) {
	if err := os.MkdirAll(jarDir, 0o755); err != nil {
		return nil, fmt.Errorf("create jar dir: %w", err)
	}

	metaFile := filepath.Join(jarDir, "jars.json")
	if _, err := os.Stat(metaFile); os.IsNotExist(err) {
		if err := os.WriteFile(metaFile, nil, 0o644); err != nil {
			return nil, fmt.Errorf("create jar meta file: %w", err)
		}
	}

	return &JarStore{jarDir: jarDir, metaFile: metaFile}, nil
}

func (s *JarStore) Upload(serviceName, filename string, reader io.Reader) (JarMeta, error) {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return JarMeta{}, fmt.Errorf("service_name is required")
	}

	cleanName := filepath.Base(filename)
	if cleanName == "." || cleanName == "" {
		return JarMeta{}, fmt.Errorf("invalid filename")
	}
	if !strings.HasSuffix(strings.ToLower(cleanName), ".jar") {
		return JarMeta{}, fmt.Errorf("only .jar files are supported")
	}

	storedDir := filepath.Join(s.jarDir, serviceName)
	if err := os.MkdirAll(storedDir, 0o755); err != nil {
		return JarMeta{}, fmt.Errorf("create service jar dir: %w", err)
	}

	targetPath := filepath.Join(storedDir, cleanName)
	file, err := os.Create(targetPath)
	if err != nil {
		return JarMeta{}, fmt.Errorf("create jar file: %w", err)
	}
	defer file.Close()

	size, err := io.Copy(file, reader)
	if err != nil {
		return JarMeta{}, fmt.Errorf("write jar file: %w", err)
	}
	if size <= 0 {
		_ = file.Close()
		_ = os.Remove(targetPath)
		return JarMeta{}, fmt.Errorf("jar file is empty")
	}

	meta := JarMeta{
		Filename:    cleanName,
		ServiceName: serviceName,
		Size:        size,
		UploadTime:  time.Now().UTC(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	metaHandle, err := os.OpenFile(s.metaFile, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return JarMeta{}, fmt.Errorf("open jar meta file: %w", err)
	}
	defer metaHandle.Close()

	encoded, err := json.Marshal(meta)
	if err != nil {
		return JarMeta{}, fmt.Errorf("encode jar meta: %w", err)
	}
	if _, err := metaHandle.Write(append(encoded, '\n')); err != nil {
		return JarMeta{}, fmt.Errorf("append jar meta: %w", err)
	}

	return meta, nil
}

func (s *JarStore) List() ([]JarMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.readMetas()
}

func (s *JarStore) ExistsForService(serviceName, filename string) bool {
	metas, err := s.List()
	if err != nil {
		return false
	}
	for _, meta := range metas {
		if meta.ServiceName == serviceName && meta.Filename == filepath.Base(filename) {
			return true
		}
	}
	return false
}

func (s *JarStore) ResolveFilePath(serviceName, filename string) (string, error) {
	if !s.ExistsForService(serviceName, filename) {
		return "", fmt.Errorf("jar %q does not belong to service %q", filename, serviceName)
	}

	targetPath := filepath.Join(s.jarDir, serviceName, filepath.Base(filename))
	if _, err := os.Stat(targetPath); err != nil {
		return "", fmt.Errorf("stat jar file: %w", err)
	}
	return targetPath, nil
}

func (s *JarStore) GetFilePath(filename string) (string, error) {
	metas, err := s.List()
	if err != nil {
		return "", err
	}
	var matched []JarMeta
	for _, meta := range metas {
		if meta.Filename == filepath.Base(filename) {
			matched = append(matched, meta)
		}
	}
	if len(matched) == 0 {
		return "", fmt.Errorf("jar not found")
	}
	if len(matched) > 1 {
		return "", fmt.Errorf("jar filename is ambiguous; service_name is required")
	}
	return filepath.Join(s.jarDir, matched[0].ServiceName, matched[0].Filename), nil
}

func (s *JarStore) readMetas() ([]JarMeta, error) {
	file, err := os.Open(s.metaFile)
	if err != nil {
		return nil, fmt.Errorf("open jar meta file: %w", err)
	}
	defer file.Close()

	var metas []JarMeta
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var meta JarMeta
		if err := json.Unmarshal([]byte(line), &meta); err != nil {
			return nil, fmt.Errorf("decode jar meta: %w", err)
		}
		metas = append(metas, meta)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan jar meta file: %w", err)
	}

	return metas, nil
}
