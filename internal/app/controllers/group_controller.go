package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/faeln1/go-whatsapp-api/internal/app/services"
	"github.com/faeln1/go-whatsapp-api/internal/domain/group"
)

type GroupController struct {
	service services.GroupService
}

func NewGroupController(s services.GroupService) *GroupController {
	return &GroupController{service: s}
}

func (c *GroupController) Create(w http.ResponseWriter, r *http.Request, instanceName string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var in group.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	in.InstanceID = instanceName

	out, err := c.service.Create(r.Context(), in)
	if err != nil {
		writeError(w, mapGroupStatus(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"group": out})
}

func (c *GroupController) UpdatePicture(w http.ResponseWriter, r *http.Request, instanceName string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	groupJID := strings.TrimSpace(r.URL.Query().Get("groupJid"))
	if groupJID == "" {
		groupJID = strings.TrimSpace(r.URL.Query().Get("groupJID"))
	}
	if groupJID == "" {
		writeError(w, http.StatusBadRequest, ErrInvalidParam)
		return
	}

	var in group.UpdatePictureInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	in.InstanceID = instanceName
	in.GroupJID = groupJID

	out, err := c.service.UpdatePicture(r.Context(), in)
	if err != nil {
		writeError(w, mapGroupStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"group": out})
}

func (c *GroupController) UpdateDescription(w http.ResponseWriter, r *http.Request, instanceName string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	groupJID := strings.TrimSpace(r.URL.Query().Get("groupJid"))
	if groupJID == "" {
		groupJID = strings.TrimSpace(r.URL.Query().Get("groupJID"))
	}
	if groupJID == "" {
		writeError(w, http.StatusBadRequest, ErrInvalidParam)
		return
	}

	var in group.UpdateDescriptionInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	in.InstanceID = instanceName
	in.GroupJID = groupJID

	out, err := c.service.UpdateDescription(r.Context(), in)
	if err != nil {
		writeError(w, mapGroupStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"group": out})
}

func (c *GroupController) SendInvite(w http.ResponseWriter, r *http.Request, instanceName string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	groupJID := strings.TrimSpace(r.URL.Query().Get("groupJid"))
	if groupJID == "" {
		groupJID = strings.TrimSpace(r.URL.Query().Get("groupJID"))
	}
	if groupJID == "" {
		writeError(w, http.StatusBadRequest, ErrInvalidParam)
		return
	}

	var in group.SendInviteInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	in.InstanceID = instanceName
	in.GroupJID = groupJID

	out, err := c.service.SendInvite(r.Context(), in)
	if err != nil {
		writeError(w, mapGroupStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func mapGroupStatus(err error) int {
	switch {
	case errors.Is(err, services.ErrGroupInstanceNotFound):
		return http.StatusNotFound
	case errors.Is(err, services.ErrGroupInstanceNotReady), errors.Is(err, services.ErrGroupInstanceNotConnected):
		return http.StatusConflict
	case errors.Is(err, services.ErrGroupInvalidInstanceID),
		errors.Is(err, services.ErrGroupInvalidParticipant),
		errors.Is(err, services.ErrGroupInvalidSubject),
		errors.Is(err, services.ErrGroupInvalidGroupJID),
		errors.Is(err, services.ErrGroupInvalidImage),
		errors.Is(err, services.ErrGroupInvalidDescription):
		return http.StatusBadRequest
	case errors.Is(err, services.ErrGroupCreate),
		errors.Is(err, services.ErrGroupPicture),
		errors.Is(err, services.ErrGroupDescription):
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}
