package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/faeln1/go-whatsapp-api/internal/domain/instance"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type WebhookDispatcher interface {
	Dispatch(ctx context.Context, inst *instance.Instance, event string, payload map[string]any) (bool, error)
}

type webhookDispatcher struct {
	client *http.Client
	log    waLog.Logger
}

func NewWebhookDispatcher(client *http.Client, log waLog.Logger) WebhookDispatcher {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &webhookDispatcher{client: client, log: log}
}

func (d *webhookDispatcher) Dispatch(ctx context.Context, inst *instance.Instance, event string, payload map[string]any) (bool, error) {
	if inst == nil {
		return false, errors.New("instance is nil")
	}
	cfg := inst.Webhook
	targetURL := strings.TrimSpace(cfg.URL)
	if targetURL == "" {
		if d.log != nil {
			d.log.Debugf("webhook skipping instance=%s event=%s: no URL configured", inst.Name, event)
		}
		return false, nil
	}
	if cfg.ByEvents && len(cfg.Events) > 0 && !containsEvent(cfg.Events, event) && !containsEvent(cfg.Events, "ALL") {
		if d.log != nil {
			d.log.Debugf("webhook skipping instance=%s event=%s: filtered by events", inst.Name, event)
		}
		return false, nil
	}
	body := map[string]any{
		"event":     event,
		"instance":  inst.Name,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"data":      payload,
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(buf))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}
	if d.log != nil {
		d.log.Debugf("webhook dispatch start instance=%s event=%s url=%s", inst.Name, event, targetURL)
	}
	resp, err := d.client.Do(req)
	if err != nil {
		if d.log != nil {
			d.log.Warnf("webhook dispatch error instance=%s event=%s url=%s err=%v", inst.Name, event, targetURL, err)
		}
		return false, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if d.log != nil {
			d.log.Warnf("webhook dispatch failed instance=%s event=%s url=%s status=%d", inst.Name, event, targetURL, resp.StatusCode)
		}
		return false, fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	if d.log != nil {
		d.log.Debugf("webhook dispatch success instance=%s event=%s url=%s status=%d", inst.Name, event, targetURL, resp.StatusCode)
	}
	return true, nil
}

func containsEvent(list []string, target string) bool {
	canonicalTarget := canonicalEventName(target)
	if canonicalTarget == "" {
		return false
	}
	for _, item := range list {
		if canonicalEventName(item) == canonicalTarget {
			return true
		}
	}
	return false
}

func canonicalEventName(value string) string {
	cleaned := strings.TrimSpace(value)
	if cleaned == "" {
		return ""
	}
	lower := strings.ToLower(cleaned)
	lower = strings.ReplaceAll(lower, "_", ".")
	lower = strings.ReplaceAll(lower, " ", "")
	return lower
}
