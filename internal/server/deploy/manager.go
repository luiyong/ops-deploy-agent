package deploy

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"ops/internal/protocol"
	"ops/internal/server/store"
)

type Manager struct {
	mu          sync.RWMutex
	tasks       map[string]*Task
	inflight    map[string]string
	jarStore    *store.JarStore
	recordStore *store.RecordStore
	hub         *AgentHub
	jarBaseURL  string
}

func NewManager(jarStore *store.JarStore, recordStore *store.RecordStore, hub *AgentHub, jarBaseURL string) (*Manager, error) {
	m := &Manager{
		tasks:       make(map[string]*Task),
		inflight:    make(map[string]string),
		jarStore:    jarStore,
		recordStore: recordStore,
		hub:         hub,
		jarBaseURL:  jarBaseURL,
	}

	if err := m.restoreRunningTasks(); err != nil {
		return nil, err
	}
	hub.OnReport(m.HandleReport)
	return m, nil
}

func (m *Manager) CreateAndDispatch(req DeployRequest) (*Task, error) {
	if req.ServiceName == "" {
		return nil, fmt.Errorf("service_name is required")
	}
	if req.JarName == "" {
		return nil, fmt.Errorf("jar_name is required")
	}
	if len(req.TargetDeviceIDs) == 0 {
		return nil, fmt.Errorf("target_device_ids is required")
	}
	if !m.jarStore.ExistsForService(req.ServiceName, req.JarName) {
		return nil, fmt.Errorf("jar %q does not belong to service %q", req.JarName, req.ServiceName)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if taskID, exists := m.inflight[req.ServiceName]; exists {
		return nil, fmt.Errorf("service %q already has an executing task %q", req.ServiceName, taskID)
	}

	now := time.Now().UTC()
	task := &Task{
		ID:              generateTaskID(),
		ServiceName:     req.ServiceName,
		JarName:         req.JarName,
		TargetDeviceIDs: append([]string(nil), req.TargetDeviceIDs...),
		Status:          TaskStatusRunning,
		CreateTime:      now,
	}

	record := store.DeployRecord{
		TaskID:          task.ID,
		CreateTime:      now,
		ServiceName:     task.ServiceName,
		JarName:         task.JarName,
		TargetDeviceIDs: append([]string(nil), task.TargetDeviceIDs...),
		OverallStatus:   TaskStatusRunning,
		DeviceResults:   nil,
	}
	if err := m.recordStore.Create(record); err != nil {
		return nil, err
	}

	m.tasks[task.ID] = task
	m.inflight[task.ServiceName] = task.ID

	if !m.hub.IsOnline() {
		task.Status = TaskStatusFailed
		task.ErrorMessage = "Agent 离线"
		end := time.Now().UTC()
		task.EndTime = &end
		if err := m.recordStore.Update(task.ID, TaskStatusFailed, task.ErrorMessage, nil, &end); err != nil {
			return nil, err
		}
		delete(m.inflight, task.ServiceName)
		return task, nil
	}

	inst := protocol.DeployInstruction{
		Type:            protocol.MessageTypeDeploy,
		TaskID:          task.ID,
		ServiceName:     task.ServiceName,
		JarName:         task.JarName,
		JarDownloadURL:  m.buildJarDownloadURL(task.ServiceName, task.JarName),
		TargetDeviceIDs: append([]string(nil), task.TargetDeviceIDs...),
		CreateTime:      task.CreateTime,
	}
	if err := m.hub.SendInstruction(inst); err != nil {
		task.Status = TaskStatusFailed
		task.ErrorMessage = fmt.Sprintf("下发失败: %v", err)
		end := time.Now().UTC()
		task.EndTime = &end
		if updateErr := m.recordStore.Update(task.ID, TaskStatusFailed, task.ErrorMessage, nil, &end); updateErr != nil {
			return nil, updateErr
		}
		delete(m.inflight, task.ServiceName)
		return task, nil
	}

	return task, nil
}

func (m *Manager) HandleReport(report protocol.TaskReport) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[report.TaskID]
	if !exists {
		task = &Task{
			ID:              report.TaskID,
			ServiceName:     report.ServiceName,
			JarName:         report.JarName,
			TargetDeviceIDs: append([]string(nil), report.TargetDeviceIDs...),
			CreateTime:      report.StartTime,
		}
		m.tasks[report.TaskID] = task
	}

	task.ServiceName = report.ServiceName
	task.JarName = report.JarName
	task.TargetDeviceIDs = append([]string(nil), report.TargetDeviceIDs...)
	task.DeviceResults = append([]protocol.DeviceResult(nil), report.DeviceResults...)
	end := report.EndTime
	task.EndTime = &end
	task.Status = calculateTaskStatus(report.DeviceResults)
	task.ErrorMessage = aggregateTaskError(report.DeviceResults)

	_ = m.recordStore.Update(task.ID, task.Status, task.ErrorMessage, task.DeviceResults, &end)
	delete(m.inflight, task.ServiceName)
}

func (m *Manager) GetTask(taskID string) (*Task, error) {
	m.mu.RLock()
	if task, ok := m.tasks[taskID]; ok {
		copyTask := *task
		if copyTask.Status == TaskStatusRecovering {
			copyTask.Status = TaskStatusRunning
		}
		copyTask.TargetDeviceIDs = append([]string(nil), task.TargetDeviceIDs...)
		copyTask.DeviceResults = append([]protocol.DeviceResult(nil), task.DeviceResults...)
		m.mu.RUnlock()
		return &copyTask, nil
	}
	m.mu.RUnlock()

	record, err := m.recordStore.Get(taskID)
	if err != nil || record == nil {
		return nil, err
	}
	task := &Task{
		ID:              record.TaskID,
		ServiceName:     record.ServiceName,
		JarName:         record.JarName,
		TargetDeviceIDs: append([]string(nil), record.TargetDeviceIDs...),
		Status:          record.OverallStatus,
		ErrorMessage:    record.ErrorMessage,
		DeviceResults:   append([]protocol.DeviceResult(nil), record.DeviceResults...),
		CreateTime:      record.CreateTime,
		EndTime:         record.EndTime,
	}
	return task, nil
}

func (m *Manager) ListRecords() ([]store.DeployRecord, error) {
	return m.recordStore.List()
}

func (m *Manager) restoreRunningTasks() error {
	records, err := m.recordStore.List()
	if err != nil {
		return err
	}
	for _, record := range records {
		if record.OverallStatus != TaskStatusRunning {
			continue
		}
		task := &Task{
			ID:              record.TaskID,
			ServiceName:     record.ServiceName,
			JarName:         record.JarName,
			TargetDeviceIDs: append([]string(nil), record.TargetDeviceIDs...),
			Status:          TaskStatusRecovering,
			CreateTime:      record.CreateTime,
		}
		m.tasks[record.TaskID] = task
		m.inflight[record.ServiceName] = record.TaskID
	}
	return nil
}

func (m *Manager) buildJarDownloadURL(serviceName, jarName string) string {
	values := url.Values{}
	values.Set("service_name", serviceName)
	return fmt.Sprintf("%s/%s?%s", strings.TrimRight(m.jarBaseURL, "/"), url.PathEscape(jarName), values.Encode())
}

func calculateTaskStatus(results []protocol.DeviceResult) string {
	if len(results) == 0 {
		return TaskStatusFailed
	}
	successes := 0
	failures := 0
	for _, result := range results {
		if result.Status == protocol.DeviceStatusSuccess {
			successes++
		} else {
			failures++
		}
	}
	switch {
	case failures == 0:
		return TaskStatusSuccess
	case successes == 0:
		return TaskStatusFailed
	default:
		return TaskStatusPartial
	}
}

func aggregateTaskError(results []protocol.DeviceResult) string {
	var failed []string
	for _, result := range results {
		if result.Status == protocol.DeviceStatusFailed && result.ErrorMsg != "" {
			failed = append(failed, fmt.Sprintf("%s: %s", result.DeviceID, result.ErrorMsg))
		}
	}
	return strings.Join(failed, "; ")
}

func generateTaskID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("task-%d", time.Now().UnixNano())
	}
	return "task-" + hex.EncodeToString(buf[:])
}
