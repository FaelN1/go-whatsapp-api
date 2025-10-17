package controllers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/faeln1/go-whatsapp-api/internal/app/repositories"
	"github.com/faeln1/go-whatsapp-api/internal/app/services"
	"github.com/faeln1/go-whatsapp-api/internal/domain/instance"
)

type SettingsController struct {
	service services.InstanceService
}

func NewSettingsController(s services.InstanceService) *SettingsController {
	return &SettingsController{service: s}
}

func (c *SettingsController) Set(w http.ResponseWriter, r *http.Request, instanceName string) {
	var in instance.SetSettingsInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	inst, err := c.service.SetSettings(r.Context(), instanceName, in)
	if err != nil {
		switch {
		case errors.Is(err, repositories.ErrInstanceNotFound):
			writeError(w, http.StatusNotFound, err)
		default:
			writeError(w, http.StatusBadRequest, err)
		}
		return
	}

	payload := map[string]any{
		"settings": map[string]any{
			"instanceName": inst.Name,
			"settings": map[string]any{
				"reject_call":       inst.Settings.RejectCall,
				"msg_call":          inst.Settings.MsgCall,
				"groups_ignore":     inst.Settings.GroupsIgnore,
				"always_online":     inst.Settings.AlwaysOnline,
				"read_messages":     inst.Settings.ReadMessages,
				"read_status":       inst.Settings.ReadStatus,
				"sync_full_history": inst.Settings.SyncFullHistory,
			},
		},
	}

	writeJSON(w, http.StatusCreated, payload)
}

func (c *SettingsController) Find(w http.ResponseWriter, r *http.Request, instanceName string) {
	settings, err := c.service.GetSettings(r.Context(), instanceName)
	if err != nil {
		switch {
		case errors.Is(err, repositories.ErrInstanceNotFound):
			writeError(w, http.StatusNotFound, err)
		default:
			writeError(w, http.StatusBadRequest, err)
		}
		return
	}

	resp := map[string]any{
		"reject_call":       settings.RejectCall,
		"msg_call":          settings.MsgCall,
		"groups_ignore":     settings.GroupsIgnore,
		"always_online":     settings.AlwaysOnline,
		"read_messages":     settings.ReadMessages,
		"read_status":       settings.ReadStatus,
		"sync_full_history": settings.SyncFullHistory,
	}

	writeJSON(w, http.StatusOK, resp)
}
