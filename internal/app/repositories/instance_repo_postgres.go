package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/faeln1/go-whatsapp-api/internal/domain/instance"
	"github.com/lib/pq"
)

type postgresInstanceRepo struct {
	db *sql.DB
}

func NewPostgresInstanceRepo(db *sql.DB) (InstanceRepository, error) {
	repo := &postgresInstanceRepo{db: db}
	if err := repo.ensureSchema(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *postgresInstanceRepo) ensureSchema() error {
	const createTable = `
        CREATE TABLE IF NOT EXISTS instances (
            id TEXT PRIMARY KEY,
            name TEXT NOT NULL UNIQUE,
            webhook_url TEXT NOT NULL DEFAULT '',
            token TEXT NOT NULL UNIQUE,
            number TEXT NOT NULL DEFAULT '',
            integration TEXT NOT NULL DEFAULT '',
            settings JSONB NOT NULL DEFAULT '{}'::jsonb,
            webhook JSONB NOT NULL DEFAULT '{}'::jsonb,
            status TEXT NOT NULL DEFAULT 'active',
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        )`
	if _, err := r.db.Exec(createTable); err != nil {
		return err
	}
	alterStatements := []string{
		"ALTER TABLE instances ADD COLUMN IF NOT EXISTS number TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE instances ADD COLUMN IF NOT EXISTS integration TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE instances ADD COLUMN IF NOT EXISTS settings JSONB NOT NULL DEFAULT '{}'::jsonb",
		"ALTER TABLE instances ADD COLUMN IF NOT EXISTS webhook JSONB NOT NULL DEFAULT '{}'::jsonb",
	}
	for _, stmt := range alterStatements {
		if _, err := r.db.Exec(stmt); err != nil {
			return err
		}
	}
	if _, err := r.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_instances_name ON instances (name)`); err != nil {
		return err
	}
	if _, err := r.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_instances_token ON instances (token)`); err != nil {
		return err
	}
	return nil
}

func (r *postgresInstanceRepo) Create(ctx context.Context, inst *instance.Instance) error {
	const query = `
        INSERT INTO instances (id, name, webhook_url, token, number, integration, settings, webhook, status, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	settingsJSON, err := json.Marshal(inst.Settings)
	if err != nil {
		return err
	}
	webhookPayload := inst.Webhook
	if webhookPayload.URL == "" {
		webhookPayload.URL = inst.WebhookURL
	}
	webhookJSON, err := json.Marshal(webhookPayload)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, query,
		string(inst.ID),
		inst.Name,
		inst.WebhookURL,
		inst.Token,
		inst.Number,
		inst.Integration,
		settingsJSON,
		webhookJSON,
		inst.Status,
		inst.CreatedAt.UTC(),
		inst.UpdatedAt.UTC(),
	)
	return r.mapError(err)
}

func (r *postgresInstanceRepo) List(ctx context.Context) ([]*instance.Instance, error) {
	const query = `
        SELECT id, name, webhook_url, token, number, integration, settings, webhook, status, created_at, updated_at
        FROM instances
        ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, r.mapError(err)
	}
	defer rows.Close()

	var results []*instance.Instance
	for rows.Next() {
		var (
			id          string
			name        string
			webhook     string
			token       string
			number      string
			integration string
			settingsRaw []byte
			webhookRaw  []byte
			status      string
			created     time.Time
			updated     time.Time
		)
		if err := rows.Scan(&id, &name, &webhook, &token, &number, &integration, &settingsRaw, &webhookRaw, &status, &created, &updated); err != nil {
			return nil, err
		}
		inst := &instance.Instance{
			ID:          instance.ID(id),
			Name:        name,
			Token:       token,
			Number:      number,
			Integration: integration,
			WebhookURL:  webhook,
			Status:      status,
			CreatedAt:   created,
			UpdatedAt:   updated,
		}
		if len(settingsRaw) > 0 {
			_ = json.Unmarshal(settingsRaw, &inst.Settings)
		}
		if len(webhookRaw) > 0 {
			_ = json.Unmarshal(webhookRaw, &inst.Webhook)
		}
		if inst.Webhook.URL == "" {
			inst.Webhook.URL = inst.WebhookURL
		}
		results = append(results, inst)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *postgresInstanceRepo) GetByName(ctx context.Context, name string) (*instance.Instance, error) {
	const query = `
        SELECT id, name, webhook_url, token, number, integration, settings, webhook, status, created_at, updated_at
        FROM instances
        WHERE name = $1`
	var (
		id          string
		webhook     string
		token       string
		number      string
		integration string
		settingsRaw []byte
		webhookRaw  []byte
		status      string
		created     time.Time
		updated     time.Time
	)
	err := r.db.QueryRowContext(ctx, query, name).Scan(&id, &name, &webhook, &token, &number, &integration, &settingsRaw, &webhookRaw, &status, &created, &updated)
	if err != nil {
		return nil, r.mapError(err)
	}
	inst := &instance.Instance{
		ID:          instance.ID(id),
		Name:        name,
		Token:       token,
		Number:      number,
		Integration: integration,
		WebhookURL:  webhook,
		Status:      status,
		CreatedAt:   created,
		UpdatedAt:   updated,
	}
	if len(settingsRaw) > 0 {
		_ = json.Unmarshal(settingsRaw, &inst.Settings)
	}
	if len(webhookRaw) > 0 {
		_ = json.Unmarshal(webhookRaw, &inst.Webhook)
	}
	if inst.Webhook.URL == "" {
		inst.Webhook.URL = inst.WebhookURL
	}
	return inst, nil
}

func (r *postgresInstanceRepo) Delete(ctx context.Context, name string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM instances WHERE name = $1`, name)
	if err != nil {
		return r.mapError(err)
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		return ErrInstanceNotFound
	}
	return err
}

func (r *postgresInstanceRepo) Update(ctx context.Context, inst *instance.Instance) error {
	const query = `
        UPDATE instances
        SET webhook_url = $1,
            token = $2,
            number = $3,
            integration = $4,
            settings = $5,
            webhook = $6,
            status = $7,
            updated_at = $8
        WHERE name = $9`
	settingsJSON, err := json.Marshal(inst.Settings)
	if err != nil {
		return err
	}
	webhookPayload := inst.Webhook
	if webhookPayload.URL == "" {
		webhookPayload.URL = inst.WebhookURL
	}
	webhookJSON, err := json.Marshal(webhookPayload)
	if err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx, query,
		inst.WebhookURL,
		inst.Token,
		inst.Number,
		inst.Integration,
		settingsJSON,
		webhookJSON,
		inst.Status,
		inst.UpdatedAt.UTC(),
		inst.Name,
	)
	if err != nil {
		return r.mapError(err)
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		return ErrInstanceNotFound
	}
	return err
}

func (r *postgresInstanceRepo) mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
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
			default:
				return ErrInstanceAlreadyExists
			}
		}
	}
	return err
}
