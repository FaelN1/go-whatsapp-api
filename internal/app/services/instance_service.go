package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/faeln1/go-whatsapp-api/internal/app/repositories"
	"github.com/faeln1/go-whatsapp-api/internal/domain/instance"
	"github.com/faeln1/go-whatsapp-api/internal/platform/whatsapp"
	"github.com/faeln1/go-whatsapp-api/pkg/storage"
	"github.com/google/uuid"
	qrcode "github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow/types"
)

type InstanceService interface {
	Create(ctx context.Context, in instance.CreateInstanceInput) (*instance.Instance, error)
	List(ctx context.Context) ([]*instance.InstanceListResponse, error)
	GetByID(ctx context.Context, id string) (*instance.InstanceListResponse, error)
	Delete(ctx context.Context, name string) error
	Logout(ctx context.Context, name string) error
	Disconnect(ctx context.Context, name string) error
	CacheQRCode(ctx context.Context, name string, code string) (string, error)
	GetCachedQRCode(ctx context.Context, name string) (string, error)
	GeneratePairingCode(ctx context.Context, name string, phone string) (string, error)
	Restart(ctx context.Context, name string) (string, error)
	ConnectionState(ctx context.Context, name string) (string, error)
	SetPresence(ctx context.Context, name string, presence string) error
	SetWebhook(ctx context.Context, name string, in instance.SetWebhookInput) (*instance.Instance, error)
	SetSettings(ctx context.Context, name string, in instance.SetSettingsInput) (*instance.Instance, error)
	GetWebhook(ctx context.Context, name string) (instance.InstanceWebhook, error)
	GetSettings(ctx context.Context, name string) (instance.InstanceSettings, error)
}

type instanceService struct {
	repo    repositories.InstanceRepository
	waMgr   *whatsapp.Manager
	storage storage.Service
}

var ErrPhoneNumberRequired = errors.New("phone number is required")

func NewInstanceService(repo repositories.InstanceRepository, waMgr *whatsapp.Manager, storage storage.Service) InstanceService {
	return &instanceService{repo: repo, waMgr: waMgr, storage: storage}
}

func (s *instanceService) Create(ctx context.Context, in instance.CreateInstanceInput) (*instance.Instance, error) {
	name := strings.TrimSpace(in.InstanceName)
	if name == "" {
		return nil, errors.New("instanceName is required")
	}
	token := strings.TrimSpace(in.Token)
	if token == "" {
		token = uuid.NewString()
	}
	now := time.Now().UTC()
	settings := instance.InstanceSettings{}
	if in.Settings != nil {
		settings = *in.Settings
	}
	if in.RejectCall != nil {
		settings.RejectCall = *in.RejectCall
	}
	if in.MsgCall != nil {
		settings.MsgCall = strings.TrimSpace(*in.MsgCall)
	}
	if in.GroupsIgnore != nil {
		settings.GroupsIgnore = *in.GroupsIgnore
	}
	if in.AlwaysOnline != nil {
		settings.AlwaysOnline = *in.AlwaysOnline
	}
	if in.ReadMessages != nil {
		settings.ReadMessages = *in.ReadMessages
	}
	if in.ReadStatus != nil {
		settings.ReadStatus = *in.ReadStatus
	}
	if in.SyncFullHistory != nil {
		settings.SyncFullHistory = *in.SyncFullHistory
	}
	webhook := instance.InstanceWebhook{}
	if in.Webhook != nil {
		webhook = *in.Webhook
	}
	webhook.URL = strings.TrimSpace(webhook.URL)
	legacyWebhook := strings.TrimSpace(in.WebhookURL)
	if webhook.URL == "" && legacyWebhook != "" {
		webhook.URL = legacyWebhook
	}
	number := strings.TrimSpace(in.Number)
	integration := strings.TrimSpace(in.Integration)
	inst := &instance.Instance{
		ID:          instance.ID(uuid.NewString()),
		Name:        name,
		Token:       token,
		Number:      number,
		Integration: integration,
		WebhookURL:  webhook.URL,
		Settings:    settings,
		Webhook:     webhook,
		CreatedAt:   now,
		UpdatedAt:   now,
		Status:      "pending_qr",
	}
	if err := s.repo.Create(ctx, inst); err != nil {
		return nil, err
	}
	sess, err := s.waMgr.Create(ctx, inst.Name, token)
	if err != nil {
		_ = s.repo.Delete(ctx, inst.Name)
		return nil, err
	}
	sess.ID = string(inst.ID)
	sess.CreatedAt = inst.CreatedAt
	sess.Token = token
	return inst, nil
}

func (s *instanceService) List(ctx context.Context) ([]*instance.InstanceListResponse, error) {
	instances, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	responses := make([]*instance.InstanceListResponse, 0, len(instances))

	// Convert each instance to the Evolution API format
	for _, inst := range instances {
		sess, _ := s.waMgr.Get(inst.Name)
		currentState := determineConnectionState(inst, sess)

		// Update status if it changed (async to not block response)
		if inst.Status != currentState {
			inst.Status = currentState
			inst.UpdatedAt = time.Now().UTC()
			go func(i *instance.Instance) {
				_ = s.repo.Update(context.Background(), i)
			}(inst)
		}

		// Build response with Evolution API structure
		response := &instance.InstanceListResponse{
			ID:                      string(inst.ID),
			Name:                    inst.Name,
			ConnectionStatus:        currentState,
			Integration:             inst.Integration,
			Number:                  inst.Number,
			Token:                   inst.Token,
			ClientName:              "evolution_exchange",
			DisconnectionReasonCode: nil,
			DisconnectionObject:     nil,
			DisconnectionAt:         nil,
			CreatedAt:               inst.CreatedAt,
			UpdatedAt:               inst.UpdatedAt,
		}

		// Add profile information if available (skip expensive network calls)
		if sess != nil && sess.Client != nil && sess.Client.Store != nil && sess.Client.Store.ID != nil {
			response.OwnerJID = sess.Client.Store.ID.String()

			// Get profile name from store (cached, fast)
			contact, err := sess.Client.Store.Contacts.GetContact(ctx, sess.Client.Store.ID.ToNonAD())
			if err == nil {
				if contact.FullName != "" {
					response.ProfileName = contact.FullName
				} else if contact.PushName != "" {
					response.ProfileName = contact.PushName
				}
			}

			// Skip profile picture fetch in list endpoint - too slow for batch operations
			// Use the dedicated profile endpoint for individual profile pictures
		}

		// Add settings details
		settingID := fmt.Sprintf("setting-%s", inst.ID)
		response.Setting = &instance.InstanceSettingDetails{
			ID:              settingID,
			RejectCall:      inst.Settings.RejectCall,
			MsgCall:         inst.Settings.MsgCall,
			GroupsIgnore:    inst.Settings.GroupsIgnore,
			AlwaysOnline:    inst.Settings.AlwaysOnline,
			ReadMessages:    inst.Settings.ReadMessages,
			ReadStatus:      inst.Settings.ReadStatus,
			SyncFullHistory: inst.Settings.SyncFullHistory,
			CreatedAt:       inst.CreatedAt,
			UpdatedAt:       inst.UpdatedAt,
			InstanceID:      string(inst.ID),
		}

		// Add counts (default to 0 for now)
		response.Count = &instance.InstanceCount{
			Message: 0,
			Contact: 0,
			Chat:    0,
		}

		responses = append(responses, response)
	}

	return responses, nil
}

func (s *instanceService) GetByID(ctx context.Context, id string) (*instance.InstanceListResponse, error) {
	instances, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	// Find instance by ID
	for _, inst := range instances {
		if string(inst.ID) == id {
			sess, _ := s.waMgr.Get(inst.Name)
			currentState := determineConnectionState(inst, sess)

			// Update status if it changed
			if inst.Status != currentState {
				inst.Status = currentState
				inst.UpdatedAt = time.Now().UTC()
				_ = s.repo.Update(ctx, inst)
			}

			// Build response with Evolution API structure
			response := &instance.InstanceListResponse{
				ID:                      string(inst.ID),
				Name:                    inst.Name,
				ConnectionStatus:        currentState,
				Integration:             inst.Integration,
				Number:                  inst.Number,
				Token:                   inst.Token,
				DisconnectionReasonCode: nil,
				DisconnectionObject:     nil,
				DisconnectionAt:         nil,
				CreatedAt:               inst.CreatedAt,
				UpdatedAt:               inst.UpdatedAt,
			}

			// Add profile information if available
			if sess != nil && sess.Client != nil && sess.Client.Store != nil && sess.Client.Store.ID != nil {
				response.OwnerJID = sess.Client.Store.ID.String()

				// Get profile name from store
				contact, err := sess.Client.Store.Contacts.GetContact(ctx, sess.Client.Store.ID.ToNonAD())
				if err == nil {
					if contact.FullName != "" {
						response.ProfileName = contact.FullName
					} else if contact.PushName != "" {
						response.ProfileName = contact.PushName
					}
				}

				// Get profile picture
				if pic, err := sess.Client.GetProfilePictureInfo(sess.Client.Store.ID.ToNonAD(), nil); err == nil && pic != nil {
					response.ProfilePicURL = pic.URL
				}
			}

			// Add settings details
			settingID := fmt.Sprintf("setting-%s", inst.ID)
			response.Setting = &instance.InstanceSettingDetails{
				ID:              settingID,
				RejectCall:      inst.Settings.RejectCall,
				MsgCall:         inst.Settings.MsgCall,
				GroupsIgnore:    inst.Settings.GroupsIgnore,
				AlwaysOnline:    inst.Settings.AlwaysOnline,
				ReadMessages:    inst.Settings.ReadMessages,
				ReadStatus:      inst.Settings.ReadStatus,
				SyncFullHistory: inst.Settings.SyncFullHistory,
				CreatedAt:       inst.CreatedAt,
				UpdatedAt:       inst.UpdatedAt,
				InstanceID:      string(inst.ID),
			}

			// Add counts (default to 0 for now)
			response.Count = &instance.InstanceCount{
				Message: 0,
				Contact: 0,
				Chat:    0,
			}

			return response, nil
		}
	}

	return nil, repositories.ErrInstanceNotFound
}

func (s *instanceService) Delete(ctx context.Context, name string) error {
	if err := s.repo.Delete(ctx, name); err != nil {
		return err
	}
	if sess, ok := s.waMgr.Get(name); ok && sess.Client != nil {
		sess.Client.Disconnect()
	}
	s.waMgr.Delete(name)
	return nil
}

func (s *instanceService) Logout(ctx context.Context, name string) error {
	inst, err := s.repo.GetByName(ctx, name)
	if err != nil {
		return err
	}
	inst.Status = "logged_out"
	inst.UpdatedAt = time.Now().UTC()
	// Encerrar sessão completamente
	if sess, ok := s.waMgr.Get(name); ok && sess.Client != nil {
		_ = sess.Client.Logout(ctx)
	}
	return s.repo.Update(ctx, inst)
}

func (s *instanceService) Disconnect(ctx context.Context, name string) error {
	inst, err := s.repo.GetByName(ctx, name)
	if err != nil {
		return err
	}
	if sess, ok := s.waMgr.Get(name); ok && sess.Client != nil {
		sess.Client.Disconnect()
	}
	inst.Status = "disconnected"
	inst.UpdatedAt = time.Now().UTC()
	return s.repo.Update(ctx, inst)
}

func (s *instanceService) CacheQRCode(ctx context.Context, name string, code string) (string, error) {
	if s.storage == nil {
		return code, s.waMgr.SetLastQR(name, code)
	}

	png, err := qrcode.Encode(code, qrcode.Medium, 256)
	if err != nil {
		return "", err
	}

	key := fmt.Sprintf("instances/%s/qr.png", name)
	url, err := s.storage.PutObject(ctx, storage.UploadInput{
		Key:         key,
		ContentType: "image/png",
		Body:        bytes.NewReader(png),
		Size:        int64(len(png)),
	})
	if err != nil {
		return "", err
	}

	return url, s.waMgr.SetLastQR(name, url)
}

func (s *instanceService) GetCachedQRCode(ctx context.Context, name string) (string, error) {
	if qr, ok := s.waMgr.GetLastQR(name); ok {
		return qr, nil
	}
	return "", whatsapp.ErrNotFound
}

func (s *instanceService) GeneratePairingCode(ctx context.Context, name string, phone string) (string, error) {
	name = strings.TrimSpace(name)
	phone = strings.TrimSpace(phone)
	if name == "" {
		return "", errors.New("instance name is required")
	}

	inst, err := s.repo.GetByName(ctx, name)
	if err != nil {
		return "", err
	}

	if phone == "" {
		phone = strings.TrimSpace(inst.Number)
	}
	if phone == "" {
		return "", ErrPhoneNumberRequired
	}

	pairingCode, err := s.waMgr.GeneratePairingCode(ctx, name, phone)
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(inst.Number) == "" {
		inst.Number = phone
		inst.UpdatedAt = time.Now().UTC()
		if updateErr := s.repo.Update(ctx, inst); updateErr != nil {
			// Ignore update errors to avoid blocking pairing flow.
		}
	}

	return pairingCode, nil
}

func (s *instanceService) Restart(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("instance name is required")
	}
	inst, err := s.repo.GetByName(ctx, name)
	if err != nil {
		return "", err
	}
	sess, ok := s.waMgr.Get(name)
	if !ok || sess == nil || sess.Client == nil {
		return "", whatsapp.ErrClientUnavailable
	}

	// Force reconnection to refresh websocket session.
	sess.Client.Disconnect()
	if err := sess.Client.Connect(); err != nil {
		state := "disconnected"
		inst.Status = state
		inst.UpdatedAt = time.Now().UTC()
		_ = s.repo.Update(ctx, inst)
		return state, err
	}

	state := determineConnectionState(inst, sess)
	inst.Status = state
	inst.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, inst); err != nil {
		return state, err
	}
	return state, nil
}

func (s *instanceService) ConnectionState(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("instance name is required")
	}
	inst, err := s.repo.GetByName(ctx, name)
	if err != nil {
		return "", err
	}
	sess, _ := s.waMgr.Get(name)
	state := determineConnectionState(inst, sess)
	if inst.Status != state {
		inst.Status = state
		inst.UpdatedAt = time.Now().UTC()
		if err := s.repo.Update(ctx, inst); err != nil {
			return state, err
		}
	}
	return state, nil
}

func (s *instanceService) SetPresence(ctx context.Context, name string, presence string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("instance name is required")
	}
	presence = strings.TrimSpace(presence)
	if presence == "" {
		return errors.New("presence value is required")
	}
	if _, err := s.repo.GetByName(ctx, name); err != nil {
		return err
	}
	sess, ok := s.waMgr.Get(name)
	if !ok || sess == nil || sess.Client == nil {
		return whatsapp.ErrClientUnavailable
	}
	if !sess.Client.IsLoggedIn() {
		return errors.New("instance not logged in")
	}
	if !sess.Client.IsConnected() {
		return errors.New("instance not connected")
	}

	switch strings.ToLower(presence) {
	case "available", "online", "connected":
		return sess.Client.SendPresence(types.PresenceAvailable)
	case "unavailable", "offline", "disconnected":
		return sess.Client.SendPresence(types.PresenceUnavailable)
	default:
		return fmt.Errorf("unsupported presence value: %s", presence)
	}
}

func (s *instanceService) SetWebhook(ctx context.Context, name string, in instance.SetWebhookInput) (*instance.Instance, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("instance name is required")
	}
	if in.Enabled && strings.TrimSpace(in.URL) == "" {
		return nil, errors.New("url is required when webhook is enabled")
	}
	if in.Enabled && len(in.Events) == 0 {
		return nil, errors.New("at least one event is required")
	}
	inst, err := s.repo.GetByName(ctx, name)
	if err != nil {
		return nil, err
	}

	events := make([]string, 0, len(in.Events))
	for _, evt := range in.Events {
		trimmed := strings.TrimSpace(evt)
		if trimmed == "" {
			continue
		}
		events = append(events, strings.ToUpper(trimmed))
	}
	if in.Enabled && len(events) == 0 {
		return nil, errors.New("events payload is invalid")
	}

	inst.Webhook.Enabled = in.Enabled
	inst.Webhook.URL = strings.TrimSpace(in.URL)
	inst.Webhook.ByEvents = in.WebhookByEvents
	inst.Webhook.Base64 = in.WebhookBase64
	inst.Webhook.Events = events
	if len(in.Headers) > 0 {
		headers := make(map[string]string)
		for k, v := range in.Headers {
			key := strings.TrimSpace(k)
			if key == "" {
				continue
			}
			headers[key] = strings.TrimSpace(v)
		}
		if len(headers) > 0 {
			inst.Webhook.Headers = headers
		} else {
			inst.Webhook.Headers = nil
		}
	} else {
		inst.Webhook.Headers = nil
	}
	inst.WebhookURL = inst.Webhook.URL
	inst.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, inst); err != nil {
		return nil, err
	}
	return inst, nil
}

func (s *instanceService) SetSettings(ctx context.Context, name string, in instance.SetSettingsInput) (*instance.Instance, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("instance name is required")
	}
	inst, err := s.repo.GetByName(ctx, name)
	if err != nil {
		return nil, err
	}

	inst.Settings = instance.InstanceSettings{
		RejectCall:      in.RejectCall,
		MsgCall:         strings.TrimSpace(in.MsgCall),
		GroupsIgnore:    in.GroupsIgnore,
		AlwaysOnline:    in.AlwaysOnline,
		ReadMessages:    in.ReadMessages,
		ReadStatus:      in.ReadStatus,
		SyncFullHistory: in.SyncFullHistory,
	}
	if inst.Settings.MsgCall == "" {
		return nil, errors.New("msgCall is required")
	}
	inst.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, inst); err != nil {
		return nil, err
	}
	return inst, nil
}

func (s *instanceService) GetWebhook(ctx context.Context, name string) (instance.InstanceWebhook, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return instance.InstanceWebhook{}, errors.New("instance name is required")
	}
	inst, err := s.repo.GetByName(ctx, name)
	if err != nil {
		return instance.InstanceWebhook{}, err
	}
	if inst.Webhook.URL == "" && inst.WebhookURL != "" {
		inst.Webhook.URL = inst.WebhookURL
	}
	return inst.Webhook, nil
}

func (s *instanceService) GetSettings(ctx context.Context, name string) (instance.InstanceSettings, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return instance.InstanceSettings{}, errors.New("instance name is required")
	}
	inst, err := s.repo.GetByName(ctx, name)
	if err != nil {
		return instance.InstanceSettings{}, err
	}
	return inst.Settings, nil
}

func determineConnectionState(inst *instance.Instance, sess *whatsapp.Session) string {
	if inst != nil && strings.EqualFold(inst.Status, "logged_out") {
		return "closed"
	}
	if sess == nil || sess.Client == nil {
		if inst != nil && strings.EqualFold(inst.Status, "disconnected") {
			return "disconnected"
		}
		return "disconnected"
	}
	if !sess.Client.IsLoggedIn() {
		return "disconnected"
	}
	if sess.Client.IsConnected() {
		return "open"
	}
	return "disconnected"
}

// mapToEvolutionStatus maps internal states to Evolution API status format
// Returns: "open" (connected), "closed" (logged out), "disconnected" (not connected)
func mapToEvolutionStatus(internalState string) string {
	switch strings.ToLower(internalState) {
	case "connected", "open":
		return "open"
	case "logged_out", "closed":
		return "closed"
	case "pending_qr", "disconnected":
		return "disconnected"
	default:
		return "disconnected"
	}
}
