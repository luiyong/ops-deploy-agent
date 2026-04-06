package protocol

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestDeployInstructionJSONRoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Round(time.Second)
	original := DeployInstruction{
		Type:            MessageTypeDeploy,
		TaskID:          "task-1",
		ServiceName:     "svc-a",
		JarName:         "svc-a-1.0.jar",
		JarDownloadURL:  "http://server/api/jars/download/svc-a-1.0.jar?service_name=svc-a",
		TargetDeviceIDs: []string{"device-b", "device-c"},
		CreateTime:      now,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got DeployInstruction
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(got, original) {
		t.Fatalf("round-trip mismatch: got %+v want %+v", got, original)
	}
}

func TestTaskReportJSONContainsCompleteContext(t *testing.T) {
	t.Parallel()

	report := TaskReport{
		Type:            MessageTypeReport,
		TaskID:          "task-1",
		ServiceName:     "svc-a",
		JarName:         "svc-a-1.0.jar",
		TargetDeviceIDs: []string{"device-b", "device-c"},
		StartTime:       time.Now().UTC().Round(time.Second),
		EndTime:         time.Now().UTC().Add(time.Second).Round(time.Second),
		DeviceResults: []DeviceResult{
			{DeviceID: "device-b", Status: DeviceStatusSuccess},
			{DeviceID: "device-c", Status: DeviceStatusFailed, ErrorMsg: "boom"},
		},
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}

	for _, key := range []string{"service_name", "jar_name", "target_device_ids", "start_time", "end_time"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("expected key %q in payload: %s", key, string(data))
		}
	}
}

func TestAgentHandshakeJSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := AgentHandshake{
		Type:    MessageTypePing,
		AgentID: "agent-a",
		Devices: []DeviceInfo{{ID: "device-b"}, {ID: "device-c"}},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got AgentHandshake
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Type != original.Type || got.AgentID != original.AgentID || len(got.Devices) != len(original.Devices) {
		t.Fatalf("unexpected handshake: got %+v want %+v", got, original)
	}
}

func TestDeviceResultFailedJSONContainsErrorMessage(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(DeviceResult{
		DeviceID: "device-b",
		Status:   DeviceStatusFailed,
		ErrorMsg: "stop timeout",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["error_msg"] != "stop timeout" {
		t.Fatalf("expected error_msg in payload: %s", string(data))
	}
}

func TestEnvelopeTypeDistinguishesMessages(t *testing.T) {
	t.Parallel()

	types := []string{MessageTypeDeploy, MessageTypeReport, MessageTypePing, MessageTypePong}
	for _, typ := range types {
		data, err := json.Marshal(Envelope{Type: typ})
		if err != nil {
			t.Fatalf("marshal %s: %v", typ, err)
		}
		var got Envelope
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("unmarshal %s: %v", typ, err)
		}
		if got.Type != typ {
			t.Fatalf("unexpected type: got %s want %s", got.Type, typ)
		}
	}
}
