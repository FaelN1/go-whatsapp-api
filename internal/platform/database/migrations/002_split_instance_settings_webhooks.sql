-- Migration: Split instance settings and webhook into dedicated tables
-- Version: 002
-- Description: Creates settings/webhooks tables and migrates existing JSON data
-- Database: PostgreSQL

BEGIN;

-- Ensure required tables exist before copying data
CREATE TABLE IF NOT EXISTS settings (
    id TEXT PRIMARY KEY,
    instance_id TEXT NOT NULL UNIQUE,
    reject_call BOOLEAN NOT NULL DEFAULT FALSE,
    msg_call VARCHAR(100),
    groups_ignore BOOLEAN NOT NULL DEFAULT FALSE,
    always_online BOOLEAN NOT NULL DEFAULT FALSE,
    read_messages BOOLEAN NOT NULL DEFAULT FALSE,
    read_status BOOLEAN NOT NULL DEFAULT FALSE,
    sync_full_history BOOLEAN NOT NULL DEFAULT FALSE,
    wavoip_token VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_settings_instance FOREIGN KEY (instance_id)
        REFERENCES instances(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_settings_instance_id ON settings(instance_id);

CREATE TABLE IF NOT EXISTS webhooks (
    id TEXT PRIMARY KEY,
    instance_id TEXT NOT NULL UNIQUE,
    url VARCHAR(512) NOT NULL DEFAULT '',
    headers JSONB NOT NULL DEFAULT '{}'::jsonb,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    events JSONB NOT NULL DEFAULT '[]'::jsonb,
    webhook_by_events BOOLEAN NOT NULL DEFAULT FALSE,
    webhook_base64 BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_webhooks_instance FOREIGN KEY (instance_id)
        REFERENCES instances(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_webhooks_instance_id ON webhooks(instance_id);

-- Copy existing JSON data into the new settings table
INSERT INTO settings (
    id,
    instance_id,
    reject_call,
    msg_call,
    groups_ignore,
    always_online,
    read_messages,
    read_status,
    sync_full_history,
    wavoip_token,
    created_at,
    updated_at
)
SELECT
    'setting-' || id,
    id,
    COALESCE((settings ->> 'rejectCall')::BOOLEAN, FALSE),
    NULLIF(settings ->> 'msgCall', ''),
    COALESCE((settings ->> 'groupsIgnore')::BOOLEAN, FALSE),
    COALESCE((settings ->> 'alwaysOnline')::BOOLEAN, FALSE),
    COALESCE((settings ->> 'readMessages')::BOOLEAN, FALSE),
    COALESCE((settings ->> 'readStatus')::BOOLEAN, FALSE),
    COALESCE((settings ->> 'syncFullHistory')::BOOLEAN, FALSE),
    NULLIF(settings ->> 'wavoipToken', ''),
    created_at,
    updated_at
FROM instances
WHERE settings IS NOT NULL AND settings :: TEXT <> '{}'::TEXT
ON CONFLICT (instance_id) DO UPDATE SET
    reject_call = EXCLUDED.reject_call,
    msg_call = EXCLUDED.msg_call,
    groups_ignore = EXCLUDED.groups_ignore,
    always_online = EXCLUDED.always_online,
    read_messages = EXCLUDED.read_messages,
    read_status = EXCLUDED.read_status,
    sync_full_history = EXCLUDED.sync_full_history,
    wavoip_token = EXCLUDED.wavoip_token,
    updated_at = EXCLUDED.updated_at;

-- Copy existing JSON data into the new webhooks table
INSERT INTO webhooks (
    id,
    instance_id,
    url,
    headers,
    enabled,
    events,
    webhook_by_events,
    webhook_base64,
    created_at,
    updated_at
)
SELECT
    'webhook-' || id,
    id,
    COALESCE(NULLIF(webhook ->> 'url', ''), COALESCE(NULLIF(webhook_url, ''), '')),
    COALESCE(webhook -> 'headers', '{}'::JSONB),
    COALESCE((webhook ->> 'enabled')::BOOLEAN, FALSE),
    COALESCE(webhook -> 'events', '[]'::JSONB),
    COALESCE((webhook ->> 'byEvents')::BOOLEAN, FALSE),
    COALESCE((webhook ->> 'base64')::BOOLEAN, FALSE),
    created_at,
    updated_at
FROM instances
WHERE webhook IS NOT NULL AND webhook :: TEXT <> '{}'::TEXT
ON CONFLICT (instance_id) DO UPDATE SET
    url = EXCLUDED.url,
    headers = EXCLUDED.headers,
    enabled = EXCLUDED.enabled,
    events = EXCLUDED.events,
    webhook_by_events = EXCLUDED.webhook_by_events,
    webhook_base64 = EXCLUDED.webhook_base64,
    updated_at = EXCLUDED.updated_at;

-- Optional: drop legacy JSON columns after successful migration (uncomment when confident)
-- ALTER TABLE instances DROP COLUMN IF EXISTS settings;
-- ALTER TABLE instances DROP COLUMN IF EXISTS webhook;

COMMIT;
