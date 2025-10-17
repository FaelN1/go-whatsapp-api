package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/faeln1/go-whatsapp-api/internal/domain/profile"
	"github.com/faeln1/go-whatsapp-api/internal/platform/whatsapp"
	"go.mau.fi/whatsmeow/types"
)

var (
	ErrProfileInvalidInstanceID    = errors.New("invalid instance id")
	ErrProfileInvalidNumber        = errors.New("invalid number")
	ErrProfileInvalidName          = errors.New("invalid name")
	ErrProfileNotFound             = errors.New("profile not found")
	ErrProfileInstanceNotReady     = errors.New("instance not ready")
	ErrProfileInstanceNotFound     = errors.New("instance not found")
	ErrProfileInstanceNotConnected = errors.New("instance not connected")
)

type ProfileService interface {
	FetchProfile(ctx context.Context, in profile.FetchProfileInput) (profile.Profile, error)
	FetchBusinessProfile(ctx context.Context, in profile.FetchBusinessProfileInput) (profile.BusinessProfile, error)
	UpdateProfileName(ctx context.Context, in profile.UpdateProfileNameInput) (profile.UpdateProfileNameOutput, error)
	UpdateProfileStatus(ctx context.Context, in profile.UpdateProfileStatusInput) (profile.UpdateProfileStatusOutput, error)
	UpdateProfilePicture(ctx context.Context, in profile.UpdateProfilePictureInput) (profile.UpdateProfilePictureOutput, error)
	RemoveProfilePicture(ctx context.Context, in profile.RemoveProfilePictureInput) (profile.RemoveProfilePictureOutput, error)
	FetchPrivacySettings(ctx context.Context, in profile.FetchPrivacySettingsInput) (profile.PrivacySettings, error)
	UpdatePrivacySettings(ctx context.Context, in profile.UpdatePrivacySettingsInput) (profile.UpdatePrivacySettingsOutput, error)
}

type profileService struct {
	waMgr *whatsapp.Manager
}

func NewProfileService(waMgr *whatsapp.Manager) ProfileService {
	return &profileService{
		waMgr: waMgr,
	}
}

func (s *profileService) FetchProfile(ctx context.Context, in profile.FetchProfileInput) (profile.Profile, error) {
	var empty profile.Profile

	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return empty, err
	}

	number := strings.TrimSpace(in.Number)
	if number == "" {
		return empty, ErrProfileInvalidNumber
	}

	jid, err := s.parseUserJID(number)
	if err != nil {
		return empty, ErrProfileInvalidNumber
	}

	client := sess.Client

	// Fetch profile picture
	var pictureURL string
	pic, err := client.GetProfilePictureInfo(jid, nil)
	if err == nil && pic != nil {
		pictureURL = pic.URL
	}

	// For contact name and status, try to get from contact info
	info, err := client.Store.Contacts.GetContact(ctx, jid)
	var name string
	if err == nil {
		if info.FullName != "" {
			name = info.FullName
		} else if info.PushName != "" {
			name = info.PushName
		}
	}

	return profile.Profile{
		WID:        jid.String(),
		Name:       name,
		PictureURL: pictureURL,
		Status:     "", // Status message not directly available via whatsmeow API
	}, nil
}

func (s *profileService) FetchBusinessProfile(ctx context.Context, in profile.FetchBusinessProfileInput) (profile.BusinessProfile, error) {
	var empty profile.BusinessProfile

	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return empty, err
	}

	number := strings.TrimSpace(in.Number)
	if number == "" {
		return empty, ErrProfileInvalidNumber
	}

	jid, err := s.parseUserJID(number)
	if err != nil {
		return empty, ErrProfileInvalidNumber
	}

	client := sess.Client

	// Fetch business profile
	bizInfo, err := client.GetBusinessProfile(jid)
	if err != nil {
		return empty, fmt.Errorf("failed to fetch business profile: %w", err)
	}

	if bizInfo == nil {
		return empty, ErrProfileNotFound
	}

	// Fetch profile picture
	var pictureURL string
	pic, err := client.GetProfilePictureInfo(jid, nil)
	if err == nil && pic != nil {
		pictureURL = pic.URL
	}

	biz := profile.BusinessProfile{
		WID:        jid.String(),
		PictureURL: pictureURL,
		Email:      bizInfo.Email,
		Address:    bizInfo.Address,
	}

	// Get category from Categories slice
	if len(bizInfo.Categories) > 0 {
		biz.Category = bizInfo.Categories[0].Name
	}

	// Get description from ProfileOptions
	if bizInfo.ProfileOptions != nil {
		if desc, ok := bizInfo.ProfileOptions["description"]; ok {
			biz.Description = desc
		}
		if website, ok := bizInfo.ProfileOptions["website"]; ok {
			biz.Website = website
		}
	}

	// Try to get name from contact info
	info, err := client.Store.Contacts.GetContact(ctx, jid)
	if err == nil && info.BusinessName != "" {
		biz.Name = info.BusinessName
	} else if err == nil && info.FullName != "" {
		biz.Name = info.FullName
	}

	return biz, nil
}

func (s *profileService) UpdateProfileName(ctx context.Context, in profile.UpdateProfileNameInput) (profile.UpdateProfileNameOutput, error) {
	var empty profile.UpdateProfileNameOutput

	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return empty, err
	}

	name := strings.TrimSpace(in.Name)
	if name == "" {
		return empty, ErrProfileInvalidName
	}

	err = sess.Client.SetStatusMessage(name)
	if err != nil {
		return empty, fmt.Errorf("failed to update profile name: %w", err)
	}

	return profile.UpdateProfileNameOutput{
		Update: "success",
	}, nil
}

func (s *profileService) UpdateProfileStatus(ctx context.Context, in profile.UpdateProfileStatusInput) (profile.UpdateProfileStatusOutput, error) {
	var empty profile.UpdateProfileStatusOutput

	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return empty, err
	}

	status := strings.TrimSpace(in.Status)
	if status == "" {
		return empty, errors.New("status is required")
	}

	err = sess.Client.SetStatusMessage(status)
	if err != nil {
		return empty, fmt.Errorf("failed to update profile status: %w", err)
	}

	return profile.UpdateProfileStatusOutput{
		Update: "success",
	}, nil
}

func (s *profileService) UpdateProfilePicture(ctx context.Context, in profile.UpdateProfilePictureInput) (profile.UpdateProfilePictureOutput, error) {
	var empty profile.UpdateProfilePictureOutput

	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return empty, err
	}

	picture := strings.TrimSpace(in.Picture)
	if picture == "" {
		return empty, errors.New("picture URL is required")
	}

	// Download image from URL
	imageBytes, err := downloadImageFromURL(picture)
	if err != nil {
		return empty, fmt.Errorf("failed to download image: %w", err)
	}

	// Set profile picture using SetGroupPhoto with empty JID (means own profile)
	_, err = sess.Client.SetGroupPhoto(types.EmptyJID, imageBytes)
	if err != nil {
		return empty, fmt.Errorf("failed to update profile picture: %w", err)
	}

	return profile.UpdateProfilePictureOutput{
		Update: "success",
	}, nil
}

func (s *profileService) RemoveProfilePicture(ctx context.Context, in profile.RemoveProfilePictureInput) (profile.RemoveProfilePictureOutput, error) {
	var empty profile.RemoveProfilePictureOutput

	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return empty, err
	}

	// Remove profile picture by setting nil image
	_, err = sess.Client.SetGroupPhoto(types.EmptyJID, nil)
	if err != nil {
		return empty, fmt.Errorf("failed to remove profile picture: %w", err)
	}

	return profile.RemoveProfilePictureOutput{
		Update: "success",
	}, nil
}

func (s *profileService) FetchPrivacySettings(ctx context.Context, in profile.FetchPrivacySettingsInput) (profile.PrivacySettings, error) {
	var empty profile.PrivacySettings

	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return empty, err
	}

	// Get privacy settings from WhatsApp
	settings, err := sess.Client.TryFetchPrivacySettings(ctx, false)
	if err != nil {
		return empty, fmt.Errorf("failed to fetch privacy settings: %w", err)
	}

	return profile.PrivacySettings{
		ReadReceipts: mapPrivacySettingToString(settings.ReadReceipts),
		Profile:      mapPrivacySettingToString(settings.Profile),
		Status:       mapPrivacySettingToString(settings.Status),
		Online:       mapPrivacySettingToString(settings.Online),
		Last:         mapPrivacySettingToString(settings.LastSeen),
		GroupAdd:     mapPrivacySettingToString(settings.GroupAdd),
	}, nil
}

func (s *profileService) UpdatePrivacySettings(ctx context.Context, in profile.UpdatePrivacySettingsInput) (profile.UpdatePrivacySettingsOutput, error) {
	var empty profile.UpdatePrivacySettingsOutput

	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return empty, err
	}

	client := sess.Client

	// Update each privacy setting
	if in.ReadReceipts != "" {
		_, err := client.SetPrivacySetting(ctx, types.PrivacySettingTypeReadReceipts, parsePrivacySetting(in.ReadReceipts))
		if err != nil {
			return empty, fmt.Errorf("failed to update read receipts: %w", err)
		}
	}

	if in.Profile != "" {
		_, err := client.SetPrivacySetting(ctx, types.PrivacySettingTypeProfile, parsePrivacySetting(in.Profile))
		if err != nil {
			return empty, fmt.Errorf("failed to update profile: %w", err)
		}
	}

	if in.Status != "" {
		_, err := client.SetPrivacySetting(ctx, types.PrivacySettingTypeStatus, parsePrivacySetting(in.Status))
		if err != nil {
			return empty, fmt.Errorf("failed to update status: %w", err)
		}
	}

	if in.Online != "" {
		_, err := client.SetPrivacySetting(ctx, types.PrivacySettingTypeOnline, parsePrivacySetting(in.Online))
		if err != nil {
			return empty, fmt.Errorf("failed to update online: %w", err)
		}
	}

	if in.Last != "" {
		_, err := client.SetPrivacySetting(ctx, types.PrivacySettingTypeLastSeen, parsePrivacySetting(in.Last))
		if err != nil {
			return empty, fmt.Errorf("failed to update last seen: %w", err)
		}
	}

	if in.GroupAdd != "" {
		_, err := client.SetPrivacySetting(ctx, types.PrivacySettingTypeGroupAdd, parsePrivacySetting(in.GroupAdd))
		if err != nil {
			return empty, fmt.Errorf("failed to update group add: %w", err)
		}
	}

	return profile.UpdatePrivacySettingsOutput{
		Update: "success",
	}, nil
}

// Helper methods

func (s *profileService) readySession(instanceID string) (*whatsapp.Session, error) {
	cleaned := strings.TrimSpace(instanceID)
	if cleaned == "" {
		return nil, ErrProfileInvalidInstanceID
	}
	sess, ok := s.waMgr.Get(cleaned)
	if !ok {
		return nil, ErrProfileInstanceNotFound
	}
	if sess.Client == nil {
		return nil, ErrProfileInstanceNotReady
	}
	if !sess.Client.IsConnected() {
		return nil, ErrProfileInstanceNotConnected
	}
	return sess, nil
}

func (s *profileService) parseUserJID(raw string) (types.JID, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return types.EmptyJID, errors.New("empty jid")
	}

	// If already a full JID, parse it
	if strings.Contains(trimmed, "@") {
		parsed, err := types.ParseJID(trimmed)
		if err != nil {
			return types.EmptyJID, err
		}
		return parsed, nil
	}

	// Otherwise, treat as phone number
	cleaned := strings.ReplaceAll(trimmed, "+", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	cleaned = strings.ReplaceAll(cleaned, "-", "")

	if cleaned == "" {
		return types.EmptyJID, errors.New("invalid phone number")
	}

	return types.NewJID(cleaned, types.DefaultUserServer), nil
}

// downloadImageFromURL downloads an image from a URL and returns the bytes
func downloadImageFromURL(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download image: status %d", resp.StatusCode)
	}

	imageBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}

	return imageBytes, nil
}

// mapPrivacySettingToString converts whatsmeow privacy setting to string
func mapPrivacySettingToString(setting types.PrivacySetting) string {
	switch setting {
	case types.PrivacySettingUndefined:
		return "undefined"
	case types.PrivacySettingAll:
		return "all"
	case types.PrivacySettingContacts:
		return "contacts"
	case types.PrivacySettingContactBlacklist:
		return "contact_blacklist"
	case types.PrivacySettingMatchLastSeen:
		return "match_last_seen"
	case types.PrivacySettingNone:
		return "none"
	default:
		return "undefined"
	}
}

// parsePrivacySetting converts string to whatsmeow privacy setting
func parsePrivacySetting(value string) types.PrivacySetting {
	switch strings.ToLower(value) {
	case "all":
		return types.PrivacySettingAll
	case "contacts":
		return types.PrivacySettingContacts
	case "contact_blacklist":
		return types.PrivacySettingContactBlacklist
	case "match_last_seen":
		return types.PrivacySettingMatchLastSeen
	case "none":
		return types.PrivacySettingNone
	default:
		return types.PrivacySettingUndefined
	}
}
