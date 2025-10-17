package controllers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/faeln1/go-whatsapp-api/internal/app/repositories"
	"github.com/faeln1/go-whatsapp-api/internal/app/services"
	"github.com/faeln1/go-whatsapp-api/internal/domain/instance"
	"github.com/faeln1/go-whatsapp-api/internal/platform/whatsapp"
	qrcode "github.com/skip2/go-qrcode"
	wa "go.mau.fi/whatsmeow"
)

type InstanceController struct {
	service   services.InstanceService
	bootstrap *services.SessionBootstrap
	webhook   services.WebhookDispatcher
}

func NewInstanceController(s services.InstanceService, b *services.SessionBootstrap, w services.WebhookDispatcher) *InstanceController {
	return &InstanceController{service: s, bootstrap: b, webhook: w}
}

type createInstanceResponse struct {
	Instance *instanceResponseData `json:"instance"`
	Hash     *hashResponseData     `json:"hash"`
	Settings *settingsResponseData `json:"settings"`
	QRCode   *qrPayload            `json:"qrcode,omitempty"`
}

type instanceResponseData struct {
	InstanceName          string  `json:"instanceName"`
	InstanceID            string  `json:"instanceId"`
	WebhookWaBusiness     *string `json:"webhook_wa_business"`
	AccessTokenWaBusiness string  `json:"access_token_wa_business"`
	Status                string  `json:"status"`
}

type hashResponseData struct {
	APIKey string `json:"apikey"`
}

type settingsResponseData struct {
	RejectCall      bool   `json:"reject_call"`
	MsgCall         string `json:"msg_call"`
	GroupsIgnore    bool   `json:"groups_ignore"`
	AlwaysOnline    bool   `json:"always_online"`
	ReadMessages    bool   `json:"read_messages"`
	ReadStatus      bool   `json:"read_status"`
	SyncFullHistory bool   `json:"sync_full_history"`
}

type qrPayload struct {
	Event          string `json:"event"`
	Code           string `json:"code,omitempty"`
	Link           string `json:"link,omitempty"`
	Image          string `json:"image,omitempty"`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`
}

type connectResponse struct {
	Status         string `json:"status,omitempty"`
	PairingCode    string `json:"pairingCode,omitempty"`
	Code           string `json:"code,omitempty"`
	Link           string `json:"link,omitempty"`
	Count          int    `json:"count,omitempty"`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`
	Message        string `json:"message,omitempty"`
}

func (c *InstanceController) Create(w http.ResponseWriter, r *http.Request) {
	var in instance.CreateInstanceInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	inst, err := c.service.Create(r.Context(), in)
	if err != nil {
		switch {
		case errors.Is(err, repositories.ErrInstanceAlreadyExists), errors.Is(err, repositories.ErrTokenAlreadyExists):
			writeError(w, http.StatusConflict, err)
		default:
			writeError(w, http.StatusBadRequest, err)
		}
		return
	}

	// Build Evolution API compatible response
	resp := createInstanceResponse{
		Instance: &instanceResponseData{
			InstanceName:          inst.Name,
			InstanceID:            string(inst.ID),
			WebhookWaBusiness:     nil,
			AccessTokenWaBusiness: "",
			Status:                "created",
		},
		Hash: &hashResponseData{
			APIKey: inst.Token,
		},
		Settings: &settingsResponseData{
			RejectCall:      inst.Settings.RejectCall,
			MsgCall:         inst.Settings.MsgCall,
			GroupsIgnore:    inst.Settings.GroupsIgnore,
			AlwaysOnline:    inst.Settings.AlwaysOnline,
			ReadMessages:    inst.Settings.ReadMessages,
			ReadStatus:      inst.Settings.ReadStatus,
			SyncFullHistory: inst.Settings.SyncFullHistory,
		},
	}

	if in.QRCode {
		qrData, qrErr := c.generateQRCode(r.Context(), inst.Name)
		if qrErr != nil {
			_ = c.service.Delete(r.Context(), inst.Name)
			writeError(w, http.StatusInternalServerError, qrErr)
			return
		}
		resp.QRCode = qrData
		if qrData != nil && qrData.Event == "code" {
			payload := map[string]any{}
			if qrData.Link != "" {
				payload["link"] = qrData.Link
			} else if qrData.Code != "" {
				payload["code"] = qrData.Code
			}
			if qrData.Image != "" {
				payload["image"] = qrData.Image
			}
			if qrData.TimeoutSeconds > 0 {
				payload["timeoutSeconds"] = qrData.TimeoutSeconds
			}
			c.dispatchWebhookAsync(inst, "QRCODE_UPDATED", payload)
		}
	}

	writeJSON(w, http.StatusCreated, resp)
}

func (c *InstanceController) List(w http.ResponseWriter, r *http.Request) {
	// Check if instanceId parameter is provided
	instanceID := strings.TrimSpace(r.URL.Query().Get("instanceId"))

	if instanceID != "" {
		// Return single instance by ID
		item, err := c.service.GetByID(r.Context(), instanceID)
		if err != nil {
			if errors.Is(err, repositories.ErrInstanceNotFound) {
				writeError(w, http.StatusNotFound, err)
			} else {
				writeError(w, http.StatusInternalServerError, err)
			}
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}

	// Return all instances
	items, err := c.service.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (c *InstanceController) Delete(w http.ResponseWriter, r *http.Request) {
	name := extractInstanceName(r)
	if name == "" {
		writeError(w, http.StatusBadRequest, ErrInvalidParam)
		return
	}
	if err := c.service.Delete(r.Context(), name); err != nil {
		if errors.Is(err, repositories.ErrInstanceNotFound) {
			writeError(w, http.StatusNotFound, err)
		} else {
			writeError(w, http.StatusInternalServerError, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"Details": "Instance deleted"})
}

func (c *InstanceController) Logout(w http.ResponseWriter, r *http.Request) {
	name := extractInstanceName(r)
	if name == "" {
		writeError(w, http.StatusBadRequest, ErrInvalidParam)
		return
	}
	if err := c.service.Logout(r.Context(), name); err != nil {
		if errors.Is(err, repositories.ErrInstanceNotFound) {
			writeError(w, http.StatusNotFound, err)
		} else {
			writeError(w, http.StatusInternalServerError, err)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /instances/{name}/disconnect: desconecta do websocket, mas mantém a sessão
func (c *InstanceController) Disconnect(w http.ResponseWriter, r *http.Request) {
	name := extractInstanceName(r)
	if name == "" {
		writeError(w, http.StatusBadRequest, ErrInvalidParam)
		return
	}
	if err := c.service.Disconnect(r.Context(), name); err != nil {
		if errors.Is(err, repositories.ErrInstanceNotFound) {
			writeError(w, http.StatusNotFound, err)
		} else {
			writeError(w, http.StatusInternalServerError, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"Details": "Disconnected"})
}

// POST /instances/{name}/connect inicia ou retorna status de conexão (QR events curto polling)
func (c *InstanceController) Connect(w http.ResponseWriter, r *http.Request) {
	if c.bootstrap == nil {
		writeError(w, http.StatusServiceUnavailable, ErrInvalidParam)
		return
	}
	name := extractInstanceName(r)
	if name == "" {
		writeError(w, http.StatusBadRequest, ErrInvalidParam)
		return
	}
	phone := strings.TrimSpace(r.URL.Query().Get("number"))
	if phone == "" {
		phone = strings.TrimSpace(r.URL.Query().Get("phone"))
	}
	ctx := r.Context()
	qrChan, already, err := c.bootstrap.InitNewSession(ctx, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if already {
		writeJSON(w, http.StatusOK, connectResponse{Status: "already_logged", Message: "instance already paired"})
		return
	}

	// Ler até 1 evento (ou timeout) para fornecer código rapidamente.
	select {
	case item, ok := <-qrChan:
		if !ok {
			writeError(w, http.StatusInternalServerError, errors.New("qr channel closed"))
			return
		}
		resp := connectResponse{Status: item.Event}
		if item.Timeout > 0 {
			resp.TimeoutSeconds = int(item.Timeout.Seconds())
		}
		if item.Event == "code" && item.Code != "" {
			resp.Code = item.Code
			resp.Count = 1
			if link, err := c.service.CacheQRCode(ctx, name, item.Code); err == nil {
				if link != "" {
					resp.Link = link
				}
			} else {
				log.Printf("[QR] failed to cache code for %s: %v", name, err)
			}
			// Também imprime o QR em ASCII no terminal para facilitar o scan
			whatsapp.PrintQRASCII(item.Code)
			pairingCode, pairErr := c.service.GeneratePairingCode(ctx, name, phone)
			if pairErr != nil {
				switch {
				case errors.Is(pairErr, services.ErrPhoneNumberRequired):
					// Número não disponível: apenas omite o pairing code
				case errors.Is(pairErr, repositories.ErrInstanceNotFound):
					writeError(w, http.StatusNotFound, pairErr)
					return
				case errors.Is(pairErr, whatsapp.ErrClientUnavailable):
					writeError(w, http.StatusConflict, pairErr)
					return
				case errors.Is(pairErr, whatsapp.ErrNotFound):
					writeError(w, http.StatusNotFound, pairErr)
					return
				case errors.Is(pairErr, wa.ErrPhoneNumberTooShort), errors.Is(pairErr, wa.ErrPhoneNumberIsNotInternational):
					writeError(w, http.StatusBadRequest, pairErr)
					return
				default:
					writeError(w, http.StatusInternalServerError, pairErr)
					return
				}
			} else if pairingCode != "" {
				resp.PairingCode = strings.ReplaceAll(pairingCode, "-", "")
			}
		} else if item.Event == "timeout" {
			resp.Message = "pairing timeout"
		} else if item.Event == wa.QRChannelEventError && item.Error != nil {
			resp.Message = item.Error.Error()
		}
		writeJSON(w, http.StatusOK, resp)
	case <-time.After(5 * time.Second):
		writeJSON(w, http.StatusAccepted, connectResponse{Status: "pending"})
	}
}

// GET /instances/{name}/qr retorna imagem base64 data URI com o último QR disponível
func (c *InstanceController) QR(w http.ResponseWriter, r *http.Request) {
	name := extractInstanceName(r)
	if name == "" {
		writeError(w, http.StatusBadRequest, ErrInvalidParam)
		return
	}
	value, err := c.service.GetCachedQRCode(r.Context(), name)
	if err != nil {
		switch {
		case errors.Is(err, repositories.ErrInstanceNotFound), errors.Is(err, whatsapp.ErrNotFound):
			writeError(w, http.StatusNotFound, err)
		default:
			writeError(w, http.StatusInternalServerError, err)
		}
		return
	}
	if value == "" {
		writeJSON(w, http.StatusOK, map[string]any{"QRCode": ""})
		return
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "/") {
		writeJSON(w, http.StatusOK, map[string]any{"QRCodeURL": value})
		return
	}
	// Gera PNG base64 data URI a partir do código (fallback)
	png, err := qrcode.Encode(value, qrcode.Medium, 256)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	writeJSON(w, http.StatusOK, map[string]any{"QRCode": dataURI})
}

func (c *InstanceController) SetWebhook(w http.ResponseWriter, r *http.Request, name string) {
	if name == "" {
		name = extractInstanceNameFromPath(r.URL.Path, "/webhook/set/")
	}
	if name == "" {
		writeError(w, http.StatusBadRequest, ErrInvalidParam)
		return
	}
	var in instance.SetWebhookInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	inst, err := c.service.SetWebhook(r.Context(), name, in)
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
				"url":             inst.Webhook.URL,
				"events":          inst.Webhook.Events,
				"headers":         inst.Webhook.Headers,
				"enabled":         inst.Webhook.Enabled,
				"webhookByEvents": inst.Webhook.ByEvents,
				"webhookBase64":   inst.Webhook.Base64,
			},
		},
	}
	writeJSON(w, http.StatusCreated, payload)
}

func (c *InstanceController) SetSettings(w http.ResponseWriter, r *http.Request, name string) {
	if name == "" {
		name = extractInstanceNameFromPath(r.URL.Path, "/settings/set/")
	}
	if name == "" {
		writeError(w, http.StatusBadRequest, ErrInvalidParam)
		return
	}
	var in instance.SetSettingsInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	inst, err := c.service.SetSettings(r.Context(), name, in)
	if err != nil {
		switch {
		case errors.Is(err, repositories.ErrInstanceNotFound):
			writeError(w, http.StatusNotFound, err)
		default:
			writeError(w, http.StatusBadRequest, err)
		}
		return
	}
	settingsPayload := map[string]any{
		"reject_call":       inst.Settings.RejectCall,
		"msg_call":          inst.Settings.MsgCall,
		"groups_ignore":     inst.Settings.GroupsIgnore,
		"always_online":     inst.Settings.AlwaysOnline,
		"read_messages":     inst.Settings.ReadMessages,
		"read_status":       inst.Settings.ReadStatus,
		"sync_full_history": inst.Settings.SyncFullHistory,
	}
	payload := map[string]any{
		"settings": map[string]any{
			"instanceName": inst.Name,
			"settings":     settingsPayload,
		},
	}
	writeJSON(w, http.StatusCreated, payload)
}

func (c *InstanceController) generateQRCode(ctx context.Context, name string) (*qrPayload, error) {
	if c.bootstrap == nil {
		return &qrPayload{Event: "bootstrap_unavailable"}, nil
	}
	qrChan, already, err := c.bootstrap.InitNewSession(ctx, name)
	if err != nil {
		return nil, err
	}
	if already {
		return &qrPayload{Event: "already_logged"}, nil
	}

	// Aguardar primeiro evento do QR channel (similar ao wuzapi)
	for {
		select {
		case item, ok := <-qrChan:
			if !ok {
				return nil, errors.New("qr channel closed")
			}

			payload := &qrPayload{Event: item.Event}
			if item.Timeout > 0 {
				payload.TimeoutSeconds = int(item.Timeout.Seconds())
			}

			// Se for código QR, processar e retornar
			if item.Event == "code" && item.Code != "" {
				if link, err := c.service.CacheQRCode(ctx, name, item.Code); err == nil {
					if link != "" {
						payload.Link = link
					} else {
						payload.Code = item.Code
					}
				} else {
					payload.Code = item.Code
					log.Printf("[QR] failed to cache code for %s: %v", name, err)
				}
				whatsapp.PrintQRASCII(item.Code)
				if payload.Link == "" {
					if png, err := qrcode.Encode(item.Code, qrcode.Medium, 256); err == nil {
						payload.Image = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
					}
				}
				return payload, nil
			}

			// Se for timeout ou erro, retornar imediatamente
			if item.Event == "timeout" || item.Event == "error" {
				return payload, nil
			}

			// Se for success, significa que foi pareado
			if item.Event == "success" {
				return &qrPayload{Event: "paired_successfully"}, nil
			}

			// Outros eventos, continuar aguardando
			log.Printf("[QR] Evento recebido: %s, aguardando código...", item.Event)

		case <-time.After(30 * time.Second):
			return &qrPayload{Event: "timeout"}, nil
		}
	}
}

func (c *InstanceController) dispatchWebhookAsync(inst *instance.Instance, event string, data map[string]any) {
	if c.webhook == nil || inst == nil {
		return
	}
	ctx := context.Background()
	go func() {
		if delivered, err := c.webhook.Dispatch(ctx, inst, event, data); err != nil {
			log.Printf("webhook dispatch error for instance %s event %s: %v", inst.Name, event, err)
		} else if !delivered {
			log.Printf("webhook dispatch skipped for instance %s event %s", inst.Name, event)
		}
	}()
}

// extractInstanceName tenta obter o identificador da instância tanto via PathValue (quando disponível)
// quanto por parsing direto do path em cenários de roteamento manual.
func extractInstanceName(r *http.Request) string {
	if v := r.PathValue("name"); v != "" {
		return v
	}
	p := r.URL.Path
	if !strings.HasPrefix(p, "/instances/") {
		return ""
	}
	rest := strings.TrimPrefix(p, "/instances/")
	rest = strings.TrimSuffix(rest, "/")
	if rest == "" {
		return ""
	}
	parts := strings.Split(rest, "/")
	if len(parts) == 0 {
		return ""
	}
	// parts[0] é sempre o nome; ignorar qualquer sufixo (logout|connect|...)
	return parts[0]
}

func extractInstanceNameFromPath(path string, prefix string) string {
	clean := strings.TrimSpace(path)
	if clean == "" {
		return ""
	}
	if strings.HasPrefix(clean, prefix) {
		clean = strings.TrimPrefix(clean, prefix)
	}
	clean = strings.Trim(clean, "/")
	if idx := strings.IndexByte(clean, '/'); idx >= 0 {
		clean = clean[:idx]
	}
	return clean
}
