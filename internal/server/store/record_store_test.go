package store

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"ops/internal/protocol"
)

func TestRecordStoreCreateUpdateAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records.json")
	store, err := NewRecordStore(path)
	if err != nil {
		t.Fatalf("new record store: %v", err)
	}

	record := DeployRecord{
		TaskID:        "task-1",
		CreateTime:    time.Now().UTC(),
		ServiceName:   "svc-a",
		JarName:       "svc-a.jar",
		OverallStatus: "执行中",
	}
	if err := store.Create(record); err != nil {
		t.Fatalf("create: %v", err)
	}

	records, err := store.List()
	if err != nil {
		t.Fatalf("list after create: %v", err)
	}
	if len(records) != 1 || records[0].OverallStatus != "执行中" {
		t.Fatalf("unexpected records after create: %+v", records)
	}

	endTime := time.Now().UTC()
	results := []protocol.DeviceResult{{DeviceID: "device-b", Status: protocol.DeviceStatusSuccess}}
	if err := store.Update("task-1", "成功", "", results, &endTime); err != nil {
		t.Fatalf("update: %v", err)
	}

	reloaded, err := NewRecordStore(path)
	if err != nil {
		t.Fatalf("reload record store: %v", err)
	}
	records, err = reloaded.List()
	if err != nil {
		t.Fatalf("list after reload: %v", err)
	}
	if len(records) != 1 || records[0].OverallStatus != "成功" || len(records[0].DeviceResults) != 1 {
		t.Fatalf("unexpected records after reload: %+v", records)
	}
}

func TestRecordStoreCreatesMissingFileAndNDJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "records.json")
	store, err := NewRecordStore(path)
	if err != nil {
		t.Fatalf("new record store: %v", err)
	}
	if err := store.Create(DeployRecord{TaskID: "task-1", CreateTime: time.Now().UTC(), OverallStatus: "执行中"}); err != nil {
		t.Fatalf("create: %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open records file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lines := 0
	for scanner.Scan() {
		lines++
		var record DeployRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("invalid ndjson line: %v", err)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if lines != 1 {
		t.Fatalf("expected 1 line, got %d", lines)
	}
}

func TestRecordStoreConcurrentUpdateDoesNotCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records.json")
	store, err := NewRecordStore(path)
	if err != nil {
		t.Fatalf("new record store: %v", err)
	}

	if err := store.Create(DeployRecord{TaskID: "task-1", CreateTime: time.Now().UTC(), OverallStatus: "执行中"}); err != nil {
		t.Fatalf("create: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			end := time.Now().UTC()
			_ = store.Update("task-1", "成功", "", []protocol.DeviceResult{{DeviceID: "device-b", Status: protocol.DeviceStatusSuccess}}, &end)
		}(i)
	}
	wg.Wait()

	records, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected single record, got %d", len(records))
	}
	if records[0].OverallStatus != "成功" {
		t.Fatalf("unexpected status: %+v", records[0])
	}
}
