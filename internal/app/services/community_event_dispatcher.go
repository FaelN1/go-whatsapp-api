package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/faeln1/go-whatsapp-api/internal/domain/community"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// CommunityEventsDispatcher envia eventos de comunidade para um webhook global.
type CommunityEventsDispatcher interface {
	Dispatch(ctx context.Context, events []community.MembershipEvent) error
}

type communityEventsDispatcher struct {
	client *http.Client
	url    string
	log    waLog.Logger
	token  string
}

// NewCommunityEventsDispatcher cria um dispatcher com URL fixa (via env).
func NewCommunityEventsDispatcher(url, token string, client *http.Client, log waLog.Logger) CommunityEventsDispatcher {
	cleanURL := strings.TrimSpace(url)
	cleanToken := strings.TrimSpace(token)
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &communityEventsDispatcher{client: client, url: cleanURL, log: log, token: cleanToken}
}

func (d *communityEventsDispatcher) Dispatch(ctx context.Context, events []community.MembershipEvent) error {
	if len(events) == 0 {
		return nil
	}
	if d == nil {
		return errors.New("dispatcher not configured")
	}
	target := strings.TrimSpace(d.url)
	if target == "" {
		if d.log != nil {
			d.log.Debugf("community events webhook ignorado: URL vazia")
		}
		return nil
	}
	payload, err := json.Marshal(events)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if d.token != "" {
		req.Header.Set("Authorization", "Bearer "+d.token)
	}

	if d.log != nil {
		d.log.Debugf("enviando %d evento(s) de comunidade para %s", len(events), target)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		if d.log != nil {
			d.log.Warnf("falha ao enviar eventos de comunidade: %v", err)
		}
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if d.log != nil {
			d.log.Warnf("webhook de comunidade retornou status %d", resp.StatusCode)
		}
		return errors.New("community events webhook returned non-2xx status")
	}

	if d.log != nil {
		d.log.Debugf("webhook de comunidade entregue com status %d", resp.StatusCode)
	}
	return nil
}
