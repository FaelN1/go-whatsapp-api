package controllers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/faeln1/go-whatsapp-api/internal/app/services"
	"github.com/faeln1/go-whatsapp-api/internal/domain/community"
)

type CommunityController struct {
	service services.CommunityService
}

func NewCommunityController(s services.CommunityService) *CommunityController {
	return &CommunityController{service: s}
}

func (c *CommunityController) Create(w http.ResponseWriter, r *http.Request, instance string) {
	var in community.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	comm, err := c.service.Create(r.Context(), instance, in)
	if err != nil {
		writeError(w, mapCommunityStatus(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, comm)
}

func (c *CommunityController) List(w http.ResponseWriter, r *http.Request, instance string) {
	items, err := c.service.List(r.Context(), instance)
	if err != nil {
		writeError(w, mapCommunityStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (c *CommunityController) Get(w http.ResponseWriter, r *http.Request, instance, communityID string) {
	jid := decodePathSegment(communityID)
	comm, err := c.service.Get(r.Context(), instance, jid)
	if err != nil {
		writeError(w, mapCommunityStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, comm)
}

func (c *CommunityController) CountMembers(w http.ResponseWriter, r *http.Request, instance, communityID string) {
	jid := decodePathSegment(communityID)
	count, err := c.service.CountMembers(r.Context(), instance, jid)
	if err != nil {
		writeError(w, mapCommunityStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"count": count})
}

func (c *CommunityController) ListMembers(w http.ResponseWriter, r *http.Request, instance, communityID string) {
	jid := decodePathSegment(communityID)
	members, err := c.service.ListMembers(r.Context(), instance, jid)
	if err != nil {
		writeError(w, mapCommunityStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, members)
}

func (c *CommunityController) InviteCode(w http.ResponseWriter, r *http.Request, instance, communityID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	jid := decodePathSegment(communityID)
	if strings.TrimSpace(jid) == "" {
		writeError(w, http.StatusBadRequest, ErrInvalidParam)
		return
	}

	reset := false
	if v := strings.TrimSpace(r.URL.Query().Get("reset")); v != "" {
		reset = strings.EqualFold(v, "true") || v == "1"
	}

	resp, err := c.service.FetchInvite(r.Context(), instance, jid, reset)
	if err != nil {
		writeError(w, mapCommunityStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (c *CommunityController) SendAnnouncement(w http.ResponseWriter, r *http.Request, instance, communityID string) {
	var in community.SendAnnouncementInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	targets := make([]string, 0, 1+len(in.Communities))
	primary := decodePathSegment(communityID)
	if strings.TrimSpace(primary) != "" {
		targets = append(targets, primary)
	}
	if len(in.Communities) > 0 {
		for _, raw := range in.Communities {
			decoded := decodePathSegment(raw)
			if strings.TrimSpace(decoded) == "" {
				continue
			}
			targets = append(targets, decoded)
		}
	}
	in.Communities = nil
	results, err := c.service.SendAnnouncement(r.Context(), instance, targets, in)
	if err != nil {
		writeError(w, mapCommunityStatus(err), err)
		return
	}
	if len(results) == 1 {
		writeJSON(w, http.StatusAccepted, results[0].Message)
		return
	}
	writeJSON(w, http.StatusAccepted, results)
}

func decodePathSegment(raw string) string {
	value, err := url.PathUnescape(raw)
	if err != nil {
		log.Printf("failed to decode path segment %s: %v", raw, err)
		return raw
	}
	return value
}

func mapCommunityStatus(err error) int {
	switch {
	case errors.Is(err, services.ErrCommunityAccessDenied):
		return http.StatusForbidden
	case errors.Is(err, services.ErrCommunityInviteLink):
		return http.StatusBadGateway
	default:
		return http.StatusBadRequest
	}
}
