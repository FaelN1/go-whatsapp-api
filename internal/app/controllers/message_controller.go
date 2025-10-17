package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/faeln1/go-whatsapp-api/internal/app/services"
	"github.com/faeln1/go-whatsapp-api/internal/domain/message"
)

type MessageController struct {
	service services.MessageService
}

func NewMessageController(s services.MessageService) *MessageController {
	return &MessageController{service: s}
}

// SendText replica o comportamento Evolution API para mensagens de texto.
func (c *MessageController) SendText(w http.ResponseWriter, r *http.Request) {
	var in message.SendTextInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !c.bindInstanceID(w, r, &in.InstanceID) {
		return
	}

	out, err := c.service.SendText(r.Context(), in)
	if err != nil {
		writeError(w, mapMessageStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// SendStatus replica o comportamento Evolution API para status (stories).
func (c *MessageController) SendStatus(w http.ResponseWriter, r *http.Request) {
	var in message.SendStatusInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !c.bindInstanceID(w, r, &in.InstanceID) {
		return
	}

	out, err := c.service.SendStatus(r.Context(), in)
	if err != nil {
		writeError(w, mapMessageStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// SendMedia replica o comportamento Evolution API para envio de mídia.
func (c *MessageController) SendMedia(w http.ResponseWriter, r *http.Request) {
	var in message.SendMediaInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !c.bindInstanceID(w, r, &in.InstanceID) {
		return
	}

	out, err := c.service.SendMedia(r.Context(), in)
	if err != nil {
		writeError(w, mapMessageStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// SendAudio replica o comportamento Evolution API para envio de áudio.
func (c *MessageController) SendAudio(w http.ResponseWriter, r *http.Request) {
	var in message.SendAudioInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !c.bindInstanceID(w, r, &in.InstanceID) {
		return
	}

	out, err := c.service.SendAudio(r.Context(), in)
	if err != nil {
		writeError(w, mapMessageStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// SendSticker replica o comportamento Evolution API para envio de figurinha.
func (c *MessageController) SendSticker(w http.ResponseWriter, r *http.Request) {
	var in message.SendStickerInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !c.bindInstanceID(w, r, &in.InstanceID) {
		return
	}

	out, err := c.service.SendSticker(r.Context(), in)
	if err != nil {
		writeError(w, mapMessageStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// SendLocation replica o comportamento Evolution API para envio de localização.
func (c *MessageController) SendLocation(w http.ResponseWriter, r *http.Request) {
	var in message.SendLocationInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !c.bindInstanceID(w, r, &in.InstanceID) {
		return
	}

	out, err := c.service.SendLocation(r.Context(), in)
	if err != nil {
		writeError(w, mapMessageStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// SendContact replica o comportamento Evolution API para envio de contatos.
func (c *MessageController) SendContact(w http.ResponseWriter, r *http.Request) {
	var in message.SendContactInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !c.bindInstanceID(w, r, &in.InstanceID) {
		return
	}

	out, err := c.service.SendContact(r.Context(), in)
	if err != nil {
		writeError(w, mapMessageStatus(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// SendReaction replica o comportamento Evolution API para envio de reações.
func (c *MessageController) SendReaction(w http.ResponseWriter, r *http.Request) {
	var in message.SendReactionInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !c.bindInstanceID(w, r, &in.InstanceID) {
		return
	}

	out, err := c.service.SendReaction(r.Context(), in)
	if err != nil {
		writeError(w, mapMessageStatus(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// SendPoll replica o comportamento Evolution API para envio de enquetes.
func (c *MessageController) SendPoll(w http.ResponseWriter, r *http.Request) {
	var in message.SendPollInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !c.bindInstanceID(w, r, &in.InstanceID) {
		return
	}

	out, err := c.service.SendPoll(r.Context(), in)
	if err != nil {
		writeError(w, mapMessageStatus(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// SendList replica o comportamento Evolution API para envio de listas.
func (c *MessageController) SendList(w http.ResponseWriter, r *http.Request) {
	var in message.SendListInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !c.bindInstanceID(w, r, &in.InstanceID) {
		return
	}

	out, err := c.service.SendList(r.Context(), in)
	if err != nil {
		writeError(w, mapMessageStatus(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// SendButtons replica o comportamento Evolution API para envio de botões interativos.
func (c *MessageController) SendButtons(w http.ResponseWriter, r *http.Request) {
	var in message.SendButtonInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !c.bindInstanceID(w, r, &in.InstanceID) {
		return
	}

	out, err := c.service.SendButtons(r.Context(), in)
	if err != nil {
		writeError(w, mapMessageStatus(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (c *MessageController) bindInstanceID(w http.ResponseWriter, r *http.Request, dst *string) bool {
	path := strings.Trim(r.URL.Path, "/")
	segments := strings.Split(path, "/")
	if len(segments) < 3 {
		writeError(w, http.StatusBadRequest, ErrInvalidParam)
		return false
	}
	instance := strings.TrimSpace(segments[2])
	if instance == "" {
		writeError(w, http.StatusBadRequest, ErrInvalidParam)
		return false
	}
	*dst = instance
	return true
}

func mapMessageStatus(err error) int {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "not implemented"):
		return http.StatusNotImplemented
	case strings.Contains(msg, "not found"):
		return http.StatusNotFound
	case strings.Contains(msg, "not ready"), strings.Contains(msg, "not connected"):
		return http.StatusConflict
	case errors.Is(err, ErrInvalidParam):
		return http.StatusBadRequest
	default:
		return http.StatusBadRequest
	}
}
