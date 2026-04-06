package deploy

import (
	"time"

	"ops/internal/protocol"
)

const (
	TaskStatusPending    = "待下发"
	TaskStatusRunning    = "执行中"
	TaskStatusSuccess    = "成功"
	TaskStatusPartial    = "部分失败"
	TaskStatusFailed     = "失败"
	TaskStatusRecovering = "待恢复"
)

type Task struct {
	ID              string                  `json:"task_id"`
	ServiceName     string                  `json:"service_name"`
	JarName         string                  `json:"jar_name"`
	TargetDeviceIDs []string                `json:"target_device_ids"`
	Status          string                  `json:"status"`
	ErrorMessage    string                  `json:"error_message,omitempty"`
	DeviceResults   []protocol.DeviceResult `json:"device_results,omitempty"`
	CreateTime      time.Time               `json:"create_time"`
	EndTime         *time.Time              `json:"end_time,omitempty"`
}

type DeployRequest struct {
	ServiceName     string   `json:"service_name"`
	JarName         string   `json:"jar_name"`
	TargetDeviceIDs []string `json:"target_device_ids"`
}
