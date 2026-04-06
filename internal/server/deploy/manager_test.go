package deploy

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"ops/internal/protocol"
	"ops/internal/server/store"
)

func TestManagerCreateAndDispatchOfflinePersistsFailedRecord(t *testing.T) {
	t.Parallel()

	manager, recordStore, _ := newTestManager(t, false)
	task, err := manager.CreateAndDispatch(DeployRequest{
		ServiceName:     "svc-a",
		JarName:         "svc-a-1.0.jar",
		TargetDeviceIDs: []string{"device-b"},
	})
	if err != nil {
		t.Fatalf("create and dispatch: %v", err)
	}
	if task.Status != TaskStatusFailed || task.ErrorMessage != "Agent 离线" {
		t.Fatalf("unexpected task: %+v", task)
	}

	records, err := recordStore.List()
	if err != nil {
		t.Fatalf("list records: %v", err)
	}
	if len(records) != 1 || records[0].OverallStatus != TaskStatusFailed {
		t.Fatalf("unexpected records: %+v", records)
	}
}

func TestManagerRejectsConcurrentTaskForSameService(t *testing.T) {
	t.Parallel()

	manager, _, cleanup := newTestManager(t, true)
	defer cleanup()

	first, err := manager.CreateAndDispatch(DeployRequest{
		ServiceName:     "svc-a",
		JarName:         "svc-a-1.0.jar",
		TargetDeviceIDs: []string{"device-b"},
	})
	if err != nil {
		t.Fatalf("first create and dispatch: %v", err)
	}
	if first.Status != TaskStatusRunning {
		t.Fatalf("expected running task, got %+v", first)
	}

	_, err = manager.CreateAndDispatch(DeployRequest{
		ServiceName:     "svc-a",
		JarName:         "svc-a-1.0.jar",
		TargetDeviceIDs: []string{"device-c"},
	})
	if err == nil || !strings.Contains(err.Error(), "already has an executing task") {
		t.Fatalf("expected concurrent task rejection, got %v", err)
	}
}

func TestManagerRejectsMismatchedJarAndService(t *testing.T) {
	t.Parallel()

	manager, recordStore, _ := newTestManager(t, false)
	_, err := manager.CreateAndDispatch(DeployRequest{
		ServiceName:     "svc-b",
		JarName:         "svc-a-1.0.jar",
		TargetDeviceIDs: []string{"device-b"},
	})
	if err == nil {
		t.Fatalf("expected mismatch rejection")
	}

	records, err := recordStore.List()
	if err != nil {
		t.Fatalf("list records: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected no records for rejected task, got %+v", records)
	}
}

func TestManagerHandleReportCalculatesFinalStatuses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result []protocol.DeviceResult
		want   string
	}{
		{
			name:   "all success",
			result: []protocol.DeviceResult{{DeviceID: "device-b", Status: protocol.DeviceStatusSuccess}, {DeviceID: "device-c", Status: protocol.DeviceStatusSuccess}},
			want:   TaskStatusSuccess,
		},
		{
			name:   "partial failure",
			result: []protocol.DeviceResult{{DeviceID: "device-b", Status: protocol.DeviceStatusSuccess}, {DeviceID: "device-c", Status: protocol.DeviceStatusFailed, ErrorMsg: "boom"}},
			want:   TaskStatusPartial,
		},
		{
			name:   "all failure",
			result: []protocol.DeviceResult{{DeviceID: "device-b", Status: protocol.DeviceStatusFailed, ErrorMsg: "boom"}},
			want:   TaskStatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, recordStore, _ := newTestManager(t, false)
			now := time.Now().UTC()
			if err := recordStore.Create(store.DeployRecord{
				TaskID:          "task-1",
				CreateTime:      now,
				ServiceName:     "svc-a",
				JarName:         "svc-a-1.0.jar",
				TargetDeviceIDs: []string{"device-b", "device-c"},
				OverallStatus:   TaskStatusRunning,
			}); err != nil {
				t.Fatalf("create record: %v", err)
			}

			manager.HandleReport(protocol.TaskReport{
				Type:            protocol.MessageTypeReport,
				TaskID:          "task-1",
				ServiceName:     "svc-a",
				JarName:         "svc-a-1.0.jar",
				TargetDeviceIDs: []string{"device-b", "device-c"},
				StartTime:       now,
				EndTime:         now.Add(time.Second),
				DeviceResults:   tt.result,
			})

			task, err := manager.GetTask("task-1")
			if err != nil {
				t.Fatalf("get task: %v", err)
			}
			if task.Status != tt.want {
				t.Fatalf("unexpected task status: got %s want %s", task.Status, tt.want)
			}
		})
	}
}

func TestManagerGetTaskAfterRestartReadsHistoricalRecord(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	jarStore, err := store.NewJarStore(filepath.Join(dir, "jars"))
	if err != nil {
		t.Fatalf("new jar store: %v", err)
	}
	if _, err := jarStore.Upload("svc-a", "svc-a-1.0.jar", strings.NewReader("jar")); err != nil {
		t.Fatalf("upload jar: %v", err)
	}
	recordStore, err := store.NewRecordStore(filepath.Join(dir, "records.json"))
	if err != nil {
		t.Fatalf("new record store: %v", err)
	}
	now := time.Now().UTC()
	if err := recordStore.Create(store.DeployRecord{
		TaskID:          "task-history",
		CreateTime:      now,
		ServiceName:     "svc-a",
		JarName:         "svc-a-1.0.jar",
		TargetDeviceIDs: []string{"device-b"},
		OverallStatus:   TaskStatusSuccess,
	}); err != nil {
		t.Fatalf("create record: %v", err)
	}

	manager, err := NewManager(jarStore, recordStore, NewAgentHub(), "http://server/api/jars/download")
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	task, err := manager.GetTask("task-history")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task == nil || task.Status != TaskStatusSuccess {
		t.Fatalf("unexpected task: %+v", task)
	}
}

func newTestManager(t *testing.T, online bool) (*Manager, *store.RecordStore, func()) {
	t.Helper()

	dir := t.TempDir()
	jarStore, err := store.NewJarStore(filepath.Join(dir, "jars"))
	if err != nil {
		t.Fatalf("new jar store: %v", err)
	}
	if _, err := jarStore.Upload("svc-a", "svc-a-1.0.jar", strings.NewReader("jar")); err != nil {
		t.Fatalf("upload jar: %v", err)
	}
	recordStore, err := store.NewRecordStore(filepath.Join(dir, "records.json"))
	if err != nil {
		t.Fatalf("new record store: %v", err)
	}

	hub := NewAgentHub()
	cleanup := func() {}
	if online {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
			serverConn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Errorf("upgrade ws: %v", err)
				return
			}
			defer serverConn.Close()
			for {
				if _, _, err := serverConn.ReadMessage(); err != nil {
					return
				}
			}
		}))
		t.Cleanup(server.Close)
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("dial ws: %v", err)
		}
		hub.Register(conn, protocol.AgentHandshake{
			AgentID: "agent-a",
			Devices: []protocol.DeviceInfo{{ID: "device-b"}, {ID: "device-c"}},
		})
		cleanup = func() {
			hub.Unregister(conn)
			_ = conn.Close()
		}
	}

	manager, err := NewManager(jarStore, recordStore, hub, "http://server/api/jars/download")
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	return manager, recordStore, cleanup
}
