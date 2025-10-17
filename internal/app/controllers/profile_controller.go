package controllers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/faeln1/go-whatsapp-api/internal/app/services"
	"github.com/faeln1/go-whatsapp-api/internal/domain/profile"
)

type ProfileController struct {
	service services.ProfileService
}

func NewProfileController(s services.ProfileService) *ProfileController {
	return &ProfileController{service: s}
}

// UpdateProfileStatus updates the profile status message
func (c *ProfileController) UpdateProfileStatus(w http.ResponseWriter, r *http.Request, instanceName string) {
	var input struct {
		Status string `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	in := profile.UpdateProfileStatusInput{
		InstanceID: instanceName,
		Status:     input.Status,
	}

	result, err := c.service.UpdateProfileStatus(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// UpdateProfilePicture updates the profile picture
func (c *ProfileController) UpdateProfilePicture(w http.ResponseWriter, r *http.Request, instanceName string) {
	var input struct {
		Picture string `json:"picture"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	in := profile.UpdateProfilePictureInput{
		InstanceID: instanceName,
		Picture:    input.Picture,
	}

	result, err := c.service.UpdateProfilePicture(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// RemoveProfilePicture removes the profile picture
func (c *ProfileController) RemoveProfilePicture(w http.ResponseWriter, r *http.Request, instanceName string) {
	in := profile.RemoveProfilePictureInput{
		InstanceID: instanceName,
	}

	result, err := c.service.RemoveProfilePicture(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// FetchPrivacySettings fetches the privacy settings
func (c *ProfileController) FetchPrivacySettings(w http.ResponseWriter, r *http.Request, instanceName string) {
	in := profile.FetchPrivacySettingsInput{
		InstanceID: instanceName,
	}

	result, err := c.service.FetchPrivacySettings(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// UpdatePrivacySettings updates the privacy settings
func (c *ProfileController) UpdatePrivacySettings(w http.ResponseWriter, r *http.Request, instanceName string) {
	var input struct {
		ReadReceipts string `json:"readreceipts"`
		Profile      string `json:"profile"`
		Status       string `json:"status"`
		Online       string `json:"online"`
		Last         string `json:"last"`
		GroupAdd     string `json:"groupadd"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	in := profile.UpdatePrivacySettingsInput{
		InstanceID:   instanceName,
		ReadReceipts: input.ReadReceipts,
		Profile:      input.Profile,
		Status:       input.Status,
		Online:       input.Online,
		Last:         input.Last,
		GroupAdd:     input.GroupAdd,
	}

	result, err := c.service.UpdatePrivacySettings(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// extractInstanceFromPath extracts instance name from the URL path
func extractInstanceFromPath(path string, prefix string) string {
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
