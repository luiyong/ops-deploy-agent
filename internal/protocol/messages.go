package protocol

import "time"

const (
	MessageTypeDeploy = "deploy"
	MessageTypeReport = "report"
	MessageTypePing   = "ping"
	MessageTypePong   = "pong"

	DeviceStatusSuccess = "success"
	DeviceStatusFailed  = "failed"
)

type Envelope struct {
	Type string `json:"type"`
}

type DeviceInfo struct {
	ID string `json:"id"`
}

type AgentHandshake struct {
	Type    string       `json:"type"`
	AgentID string       `json:"agent_id"`
	Devices []DeviceInfo `json:"devices"`
}

type DeployInstruction struct {
	Type            string    `json:"type"`
	TaskID          string    `json:"task_id"`
	ServiceName     string    `json:"service_name"`
	JarName         string    `json:"jar_name"`
	JarDownloadURL  string    `json:"jar_download_url"`
	TargetDeviceIDs []string  `json:"target_device_ids"`
	CreateTime      time.Time `json:"create_time"`
}

type DeviceResult struct {
	DeviceID string `json:"device_id"`
	Status   string `json:"status"`
	ErrorMsg string `json:"error_msg,omitempty"`
}

type TaskReport struct {
	Type            string         `json:"type"`
	TaskID          string         `json:"task_id"`
	ServiceName     string         `json:"service_name"`
	JarName         string         `json:"jar_name"`
	TargetDeviceIDs []string       `json:"target_device_ids"`
	StartTime       time.Time      `json:"start_time"`
	EndTime         time.Time      `json:"end_time"`
	DeviceResults   []DeviceResult `json:"device_results"`
}
