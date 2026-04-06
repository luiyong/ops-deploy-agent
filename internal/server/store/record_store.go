package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"ops/internal/protocol"
)

type DeployRecord struct {
	TaskID          string                  `json:"task_id"`
	CreateTime      time.Time               `json:"create_time"`
	ServiceName     string                  `json:"service_name"`
	JarName         string                  `json:"jar_name"`
	TargetDeviceIDs []string                `json:"target_device_ids"`
	OverallStatus   string                  `json:"overall_status"`
	ErrorMessage    string                  `json:"error_message,omitempty"`
	DeviceResults   []protocol.DeviceResult `json:"device_results"`
	EndTime         *time.Time              `json:"end_time,omitempty"`
}

type RecordStore struct {
	path string
	mu   sync.Mutex
}

func NewRecordStore(path string) (*RecordStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create record dir: %w", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			return nil, fmt.Errorf("create record file: %w", err)
		}
	}
	return &RecordStore{path: path}, nil
}

func (s *RecordStore) Create(record DeployRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	records, err := s.readAll()
	if err != nil {
		return err
	}
	for _, existing := range records {
		if existing.TaskID == record.TaskID {
			return nil
		}
	}

	records = append(records, record)
	return s.writeAll(records)
}

func (s *RecordStore) Update(taskID, overallStatus, errorMessage string, deviceResults []protocol.DeviceResult, endTime *time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	records, err := s.readAll()
	if err != nil {
		return err
	}
	found := false
	for i := range records {
		if records[i].TaskID == taskID {
			records[i].OverallStatus = overallStatus
			records[i].ErrorMessage = errorMessage
			records[i].DeviceResults = deviceResults
			records[i].EndTime = endTime
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("task %q not found", taskID)
	}
	return s.writeAll(records)
}

func (s *RecordStore) List() ([]DeployRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	records, err := s.readAll()
	if err != nil {
		return nil, err
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreateTime.After(records[j].CreateTime)
	})
	return records, nil
}

func (s *RecordStore) Get(taskID string) (*DeployRecord, error) {
	records, err := s.List()
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		if record.TaskID == taskID {
			copyRecord := record
			return &copyRecord, nil
		}
	}
	return nil, nil
}

func (s *RecordStore) readAll() ([]DeployRecord, error) {
	file, err := os.Open(s.path)
	if err != nil {
		return nil, fmt.Errorf("open record file: %w", err)
	}
	defer file.Close()

	var records []DeployRecord
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record DeployRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return nil, fmt.Errorf("decode record: %w", err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan record file: %w", err)
	}
	return records, nil
}

func (s *RecordStore) writeAll(records []DeployRecord) error {
	file, err := os.Create(s.path)
	if err != nil {
		return fmt.Errorf("rewrite record file: %w", err)
	}
	defer file.Close()

	for _, record := range records {
		payload, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("encode record: %w", err)
		}
		if _, err := file.Write(append(payload, '\n')); err != nil {
			return fmt.Errorf("write record: %w", err)
		}
	}
	return nil
}
