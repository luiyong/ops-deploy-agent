package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJarStoreUploadBasenameAndServiceMetadata(t *testing.T) {
	t.Parallel()

	store, dir := newTestJarStore(t)
	meta, err := store.Upload("svc-a", "../../etc/passwd.jar", strings.NewReader("jar-content"))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}

	if meta.Filename != "passwd.jar" {
		t.Fatalf("unexpected filename: %s", meta.Filename)
	}
	if meta.ServiceName != "svc-a" {
		t.Fatalf("unexpected service name: %s", meta.ServiceName)
	}

	targetPath := filepath.Join(dir, "svc-a", "passwd.jar")
	if _, err := os.Stat(targetPath); err != nil {
		t.Fatalf("expected uploaded file at %s: %v", targetPath, err)
	}

	metas, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(metas) != 1 || metas[0].ServiceName != "svc-a" {
		t.Fatalf("unexpected metas: %+v", metas)
	}
}

func TestJarStoreListAndExistsForService(t *testing.T) {
	t.Parallel()

	store, _ := newTestJarStore(t)
	if _, err := store.Upload("svc-a", "svc-a-1.0.jar", strings.NewReader("a")); err != nil {
		t.Fatalf("upload svc-a: %v", err)
	}
	if _, err := store.Upload("svc-b", "svc-b-1.0.jar", strings.NewReader("b")); err != nil {
		t.Fatalf("upload svc-b: %v", err)
	}

	metas, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(metas) != 2 {
		t.Fatalf("unexpected meta length: %d", len(metas))
	}
	if !store.ExistsForService("svc-a", "svc-a-1.0.jar") {
		t.Fatalf("expected jar to belong to svc-a")
	}
	if store.ExistsForService("svc-b", "svc-a-1.0.jar") {
		t.Fatalf("did not expect jar to belong to svc-b")
	}
}

func TestJarStoreDuplicateUploadRetainsMetadataEntries(t *testing.T) {
	t.Parallel()

	store, _ := newTestJarStore(t)
	if _, err := store.Upload("svc-a", "svc-a-1.0.jar", strings.NewReader("v1")); err != nil {
		t.Fatalf("first upload: %v", err)
	}
	if _, err := store.Upload("svc-a", "svc-a-1.0.jar", strings.NewReader("v2")); err != nil {
		t.Fatalf("second upload: %v", err)
	}

	metas, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(metas) != 2 {
		t.Fatalf("expected 2 metadata entries, got %d", len(metas))
	}
}

func TestJarStoreRejectsEmptyJar(t *testing.T) {
	t.Parallel()

	store, dir := newTestJarStore(t)
	if _, err := store.Upload("svc-a", "empty.jar", strings.NewReader("")); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected empty jar error, got %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "svc-a", "empty.jar")); !os.IsNotExist(err) {
		t.Fatalf("expected empty jar file to be removed, got err=%v", err)
	}

	metas, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(metas) != 0 {
		t.Fatalf("expected no metadata for empty jar, got %+v", metas)
	}
}

func newTestJarStore(t *testing.T) (*JarStore, string) {
	t.Helper()

	dir := t.TempDir()
	store, err := NewJarStore(dir)
	if err != nil {
		t.Fatalf("new jar store: %v", err)
	}
	return store, dir
}
