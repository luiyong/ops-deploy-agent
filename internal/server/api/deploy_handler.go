package api

import (
	"encoding/json"
	"net/http"

	"ops/internal/server/deploy"
)

type DeployHandler struct {
	manager *deploy.Manager
	hub     *deploy.AgentHub
}

func NewDeployHandler(manager *deploy.Manager, hub *deploy.AgentHub) *DeployHandler {
	return &DeployHandler{manager: manager, hub: hub}
}

func (h *DeployHandler) CreateDeploy(w http.ResponseWriter, r *http.Request) {
	var req deploy.DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "decode deploy request", err)
		return
	}

	task, err := h.manager.CreateAndDispatch(req)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "create deploy task", err)
		return
	}

	statusCode := http.StatusAccepted
	if task.Status == deploy.TaskStatusFailed {
		statusCode = http.StatusConflict
	}
	writeJSON(w, statusCode, task)
}

func (h *DeployHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	task, err := h.manager.GetTask(r.PathValue("task_id"))
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "get task", err)
		return
	}
	if task == nil {
		writeJSONError(w, http.StatusNotFound, "task not found", nil)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (h *DeployHandler) ListTasks(w http.ResponseWriter, _ *http.Request) {
	records, err := h.manager.ListRecords()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "list tasks", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": records})
}

func (h *DeployHandler) AgentStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"online":  h.hub.IsOnline(),
		"devices": h.hub.GetDevices(),
	})
}
