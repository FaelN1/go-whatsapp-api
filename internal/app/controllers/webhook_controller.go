package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/faeln1/go-whatsapp-api/internal/app/repositories"
	"github.com/faeln1/go-whatsapp-api/internal/app/services"
	"github.com/faeln1/go-whatsapp-api/internal/domain/instance"
)

type WebhookController struct {
	service services.InstanceService
}

func NewWebhookController(s services.InstanceService) *WebhookController {
	return &WebhookController{service: s}
}

func (c *WebhookController) Set(w http.ResponseWriter, r *http.Request, instanceName string) {
	var in instance.SetWebhookInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	inst, err := c.service.SetWebhook(r.Context(), instanceName, in)
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
		"webhook": map[string]any{
			"instanceName": inst.Name,
			"webhook": map[string]any{
				"enabled":         inst.Webhook.Enabled,
				"url":             inst.Webhook.URL,
				"events":          inst.Webhook.Events,
				"webhookByEvents": inst.Webhook.ByEvents,
				"webhookBase64":   inst.Webhook.Base64,
			},
		},
	}
	if len(inst.Webhook.Headers) > 0 {
		payload["webhook"].(map[string]any)["webhook"].(map[string]any)["headers"] = inst.Webhook.Headers
	}

	writeJSON(w, http.StatusCreated, payload)
}

func (c *WebhookController) Find(w http.ResponseWriter, r *http.Request, instanceName string) {
	config, err := c.service.GetWebhook(r.Context(), instanceName)
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
		"enabled":         config.Enabled,
		"url":             config.URL,
		"events":          config.Events,
		"webhookByEvents": config.ByEvents,
		"webhookBase64":   config.Base64,
	}
	if len(config.Headers) > 0 {
		filtered := make(map[string]string, len(config.Headers))
		for k, v := range config.Headers {
			filtered[strings.TrimSpace(k)] = v
		}
		resp["headers"] = filtered
	}

	writeJSON(w, http.StatusOK, resp)
}
