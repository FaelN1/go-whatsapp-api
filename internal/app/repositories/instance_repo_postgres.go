package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/faeln1/go-whatsapp-api/internal/domain/instance"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type gormInstanceRepo struct {
	db *gorm.DB
}

func NewGormInstanceRepo(db *gorm.DB) (InstanceRepository, error) {
	if err := db.AutoMigrate(&instanceModel{}, &settingModel{}, &webhookModel{}); err != nil {
		return nil, err
	}
	return &gormInstanceRepo{db: db}, nil
}

func (r *gormInstanceRepo) Create(ctx context.Context, inst *instance.Instance) error {
	model, err := toInstanceModel(inst)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(model).Error; err != nil {
			return r.mapError(err)
		}
		return nil
	})
}

func (r *gormInstanceRepo) List(ctx context.Context) ([]*instance.Instance, error) {
	var models []instanceModel
	if err := r.db.WithContext(ctx).
		Preload("Setting").
		Preload("Webhook").
		Order("created_at ASC").
		Find(&models).Error; err != nil {
		return nil, r.mapError(err)
	}
	instances := make([]*instance.Instance, 0, len(models))
	for i := range models {
		inst, err := toDomainInstance(&models[i])
		if err != nil {
			return nil, err
		}
		instances = append(instances, inst)
	}
	return instances, nil
}

func (r *gormInstanceRepo) GetByName(ctx context.Context, name string) (*instance.Instance, error) {
	var model instanceModel
	if err := r.db.WithContext(ctx).
		Preload("Setting").
		Preload("Webhook").
		Where("name = ?", name).
		First(&model).Error; err != nil {
		return nil, r.mapError(err)
	}
	return toDomainInstance(&model)
}

func (r *gormInstanceRepo) Delete(ctx context.Context, name string) error {
	res := r.db.WithContext(ctx).Where("name = ?", name).Delete(&instanceModel{})
	if res.Error != nil {
		return r.mapError(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrInstanceNotFound
	}
	return nil
}

func (r *gormInstanceRepo) Update(ctx context.Context, inst *instance.Instance) error {
	model, err := toInstanceModel(inst)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&instanceModel{}).
			Where("id = ?", model.ID).
			Updates(map[string]any{
				"name":        model.Name,
				"token":       model.Token,
				"number":      model.Number,
				"integration": model.Integration,
				"webhook_url": model.WebhookURL,
				"status":      model.Status,
				"updated_at":  model.UpdatedAt,
			})
		if res.Error != nil {
			return r.mapError(res.Error)
		}
		if res.RowsAffected == 0 {
			return ErrInstanceNotFound
		}
		if err := r.upsertSetting(tx, model.Setting); err != nil {
			return err
		}
		if err := r.upsertWebhook(tx, model.Webhook); err != nil {
			return err
		}
		return nil
	})
}

func (r *gormInstanceRepo) mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrInstanceNotFound
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		if pqErr.Code == "23505" {
			switch pqErr.Constraint {
			case "instances_name_key":
				return ErrInstanceAlreadyExists
			case "instances_token_key":
				return ErrTokenAlreadyExists
			case "settings_instance_id_key", "webhooks_instance_id_key":
				return ErrInstanceAlreadyExists
			}
		}
	}
	return err
}

func (r *gormInstanceRepo) upsertSetting(tx *gorm.DB, setting *settingModel) error {
	if setting == nil {
		return nil
	}
	var existing settingModel
	err := tx.Where("instance_id = ?", setting.InstanceID).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if setting.ID == "" {
				setting.ID = uuid.NewString()
			}
			if setting.CreatedAt.IsZero() {
				setting.CreatedAt = time.Now().UTC()
			}
			if setting.UpdatedAt.IsZero() {
				setting.UpdatedAt = time.Now().UTC()
			}
			if err := tx.Create(setting).Error; err != nil {
				return r.mapError(err)
			}
			return nil
		}
		return err
	}
	updatedAt := setting.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	updates := map[string]any{
		"reject_call":       setting.RejectCall,
		"msg_call":          setting.MsgCall,
		"groups_ignore":     setting.GroupsIgnore,
		"always_online":     setting.AlwaysOnline,
		"read_messages":     setting.ReadMessages,
		"read_status":       setting.ReadStatus,
		"sync_full_history": setting.SyncFullHistory,
		"wavoip_token":      setting.WavoipToken,
		"updated_at":        updatedAt,
	}
	if err := tx.Model(&existing).Updates(updates).Error; err != nil {
		return r.mapError(err)
	}
	return nil
}

func (r *gormInstanceRepo) upsertWebhook(tx *gorm.DB, webhook *webhookModel) error {
	if webhook == nil {
		return nil
	}
	var existing webhookModel
	err := tx.Where("instance_id = ?", webhook.InstanceID).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if webhook.ID == "" {
				webhook.ID = uuid.NewString()
			}
			if webhook.CreatedAt.IsZero() {
				webhook.CreatedAt = time.Now().UTC()
			}
			if webhook.UpdatedAt.IsZero() {
				webhook.UpdatedAt = time.Now().UTC()
			}
			if err := tx.Create(webhook).Error; err != nil {
				return r.mapError(err)
			}
			return nil
		}
		return err
	}
	updatedAt := webhook.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	updates := map[string]any{
		"url":               webhook.URL,
		"headers":           webhook.Headers,
		"enabled":           webhook.Enabled,
		"events":            webhook.Events,
		"webhook_by_events": webhook.WebhookByEvents,
		"webhook_base64":    webhook.WebhookBase64,
		"updated_at":        updatedAt,
	}
	if err := tx.Model(&existing).Updates(updates).Error; err != nil {
		return r.mapError(err)
	}
	return nil
}

type instanceModel struct {
	ID          string        `gorm:"primaryKey;size:64"`
	Name        string        `gorm:"size:120;uniqueIndex"`
	Token       string        `gorm:"size:255;uniqueIndex"`
	Number      string        `gorm:"size:32"`
	Integration string        `gorm:"size:64"`
	WebhookURL  string        `gorm:"size:512"`
	Status      string        `gorm:"size:32"`
	CreatedAt   time.Time     `gorm:"autoCreateTime"`
	UpdatedAt   time.Time     `gorm:"autoUpdateTime"`
	Setting     *settingModel `gorm:"constraint:OnDelete:CASCADE"`
	Webhook     *webhookModel `gorm:"constraint:OnDelete:CASCADE"`
}

func (instanceModel) TableName() string { return "instances" }

type settingModel struct {
	ID              string    `gorm:"primaryKey;size:64"`
	RejectCall      bool      `gorm:"default:false"`
	MsgCall         string    `gorm:"size:100"`
	GroupsIgnore    bool      `gorm:"default:false"`
	AlwaysOnline    bool      `gorm:"default:false"`
	ReadMessages    bool      `gorm:"default:false"`
	ReadStatus      bool      `gorm:"default:false"`
	SyncFullHistory bool      `gorm:"default:false"`
	WavoipToken     string    `gorm:"size:100"`
	CreatedAt       time.Time `gorm:"autoCreateTime"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime"`
	InstanceID      string    `gorm:"uniqueIndex;size:64"`
}

func (settingModel) TableName() string { return "settings" }

type webhookModel struct {
	ID              string         `gorm:"primaryKey;size:64"`
	URL             string         `gorm:"size:512"`
	Headers         datatypes.JSON `gorm:"type:jsonb"`
	Enabled         bool           `gorm:"default:true"`
	Events          datatypes.JSON `gorm:"type:jsonb"`
	WebhookByEvents bool           `gorm:"default:false"`
	WebhookBase64   bool           `gorm:"default:false"`
	CreatedAt       time.Time      `gorm:"autoCreateTime"`
	UpdatedAt       time.Time      `gorm:"autoUpdateTime"`
	InstanceID      string         `gorm:"uniqueIndex;size:64"`
}

func (webhookModel) TableName() string { return "webhooks" }

func toInstanceModel(src *instance.Instance) (*instanceModel, error) {
	if src == nil {
		return nil, nil
	}
	if src.ID == "" {
		src.ID = instance.ID(uuid.NewString())
	}
	createdAt := src.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	updatedAt := src.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}
	headers, err := encodeHeaders(src.Webhook.Headers)
	if err != nil {
		return nil, err
	}
	events, err := encodeEvents(src.Webhook.Events)
	if err != nil {
		return nil, err
	}
	setting := &settingModel{
		ID:              uuid.NewString(),
		RejectCall:      src.Settings.RejectCall,
		MsgCall:         src.Settings.MsgCall,
		GroupsIgnore:    src.Settings.GroupsIgnore,
		AlwaysOnline:    src.Settings.AlwaysOnline,
		ReadMessages:    src.Settings.ReadMessages,
		ReadStatus:      src.Settings.ReadStatus,
		SyncFullHistory: src.Settings.SyncFullHistory,
		WavoipToken:     src.Settings.WavoipToken,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
		InstanceID:      string(src.ID),
	}
	webhook := &webhookModel{
		ID:              uuid.NewString(),
		URL:             src.Webhook.URL,
		Headers:         headers,
		Enabled:         src.Webhook.Enabled,
		Events:          events,
		WebhookByEvents: src.Webhook.ByEvents,
		WebhookBase64:   src.Webhook.Base64,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
		InstanceID:      string(src.ID),
	}
	return &instanceModel{
		ID:          string(src.ID),
		Name:        src.Name,
		Token:       src.Token,
		Number:      src.Number,
		Integration: src.Integration,
		WebhookURL:  src.WebhookURL,
		Status:      src.Status,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		Setting:     setting,
		Webhook:     webhook,
	}, nil
}

func toDomainInstance(model *instanceModel) (*instance.Instance, error) {
	if model == nil {
		return nil, nil
	}
	inst := &instance.Instance{
		ID:          instance.ID(model.ID),
		Name:        model.Name,
		Token:       model.Token,
		Number:      model.Number,
		Integration: model.Integration,
		WebhookURL:  model.WebhookURL,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
		Status:      model.Status,
	}
	if model.Setting != nil {
		inst.Settings = instance.InstanceSettings{
			RejectCall:      model.Setting.RejectCall,
			MsgCall:         model.Setting.MsgCall,
			GroupsIgnore:    model.Setting.GroupsIgnore,
			AlwaysOnline:    model.Setting.AlwaysOnline,
			ReadMessages:    model.Setting.ReadMessages,
			ReadStatus:      model.Setting.ReadStatus,
			SyncFullHistory: model.Setting.SyncFullHistory,
			WavoipToken:     model.Setting.WavoipToken,
		}
	}
	if model.Webhook != nil {
		headers, err := decodeHeaders(model.Webhook.Headers)
		if err != nil {
			return nil, err
		}
		events, err := decodeEvents(model.Webhook.Events)
		if err != nil {
			return nil, err
		}
		inst.Webhook = instance.InstanceWebhook{
			URL:      model.Webhook.URL,
			ByEvents: model.Webhook.WebhookByEvents,
			Base64:   model.Webhook.WebhookBase64,
			Headers:  headers,
			Events:   events,
			Enabled:  model.Webhook.Enabled,
		}
		if inst.Webhook.URL != "" {
			inst.WebhookURL = inst.Webhook.URL
		}
	}
	return inst, nil
}

func encodeHeaders(headers map[string]string) (datatypes.JSON, error) {
	if len(headers) == 0 {
		return datatypes.JSON([]byte("{}")), nil
	}
	return encodeJSON(headers)
}

func encodeEvents(events []string) (datatypes.JSON, error) {
	if len(events) == 0 {
		return datatypes.JSON([]byte("[]")), nil
	}
	return encodeJSON(events)
}

func encodeJSON(value any) (datatypes.JSON, error) {
	bytes, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(bytes), nil
}

func decodeHeaders(data datatypes.JSON) (map[string]string, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var headers map[string]string
	if err := json.Unmarshal(data, &headers); err != nil {
		return nil, err
	}
	return headers, nil
}

func decodeEvents(data datatypes.JSON) ([]string, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var events []string
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, err
	}
	return events, nil
}
