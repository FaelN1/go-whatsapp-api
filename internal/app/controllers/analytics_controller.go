package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/faeln1/go-whatsapp-api/internal/app/services"
)

type AnalyticsController struct {
	service services.AnalyticsService
}

func NewAnalyticsController(service services.AnalyticsService) *AnalyticsController {
	return &AnalyticsController{service: service}
}

// GetMessageMetrics retorna métricas detalhadas de uma mensagem específica
// GET /analytics/messages/{trackId}/metrics
func (c *AnalyticsController) GetMessageMetrics(w http.ResponseWriter, r *http.Request, trackID string) {
	metrics, err := c.service.GetMessageMetrics(r.Context(), trackID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get message metrics", err)
		return
	}

	if metrics == nil {
		respondError(w, http.StatusNotFound, "message not found", nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(metrics)
}

// GetInstanceMetrics retorna resumo de métricas de todas as mensagens de uma instância
// GET /analytics/instances/{instanceId}/metrics?limit=50&offset=0
func (c *AnalyticsController) GetInstanceMetrics(w http.ResponseWriter, r *http.Request, instanceID string) {
	limit := 50
	offset := 0

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	metrics, err := c.service.GetInstanceMetrics(r.Context(), instanceID, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get instance metrics", err)
		return
	}

	response := map[string]any{
		"instanceId": instanceID,
		"limit":      limit,
		"offset":     offset,
		"messages":   metrics,
		"count":      len(metrics),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func respondError(w http.ResponseWriter, code int, message string, err error) {
	response := map[string]any{
		"error":   message,
		"code":    code,
		"success": false,
	}
	if err != nil {
		response["details"] = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(response)
}
