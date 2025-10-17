package tests

import (
	"context"
	"testing"

	"github.com/faeln1/go-whatsapp-api/internal/app/repositories"
	"github.com/faeln1/go-whatsapp-api/internal/app/services"
	"github.com/faeln1/go-whatsapp-api/internal/domain/instance"
	"github.com/faeln1/go-whatsapp-api/internal/platform/whatsapp"
	"github.com/faeln1/go-whatsapp-api/pkg/logger"
)

func TestCreateInstance(t *testing.T) {
	repo := repositories.NewInMemoryInstanceRepo()
	waMgr := whatsapp.NewManager(logger.New("DEBUG").App)
	svc := services.NewInstanceService(repo, waMgr, nil)
	in := instance.CreateInstanceInput{
		InstanceName: "test1",
		Integration:  "WHATSAPP-MEOW",
		Settings: &instance.InstanceSettings{
			RejectCall: true,
		},
		Webhook: &instance.InstanceWebhook{
			URL:    "http://example",
			Events: []string{"MESSAGES_UPSERT"},
		},
	}
	ctx := context.Background()
	inst, err := svc.Create(ctx, in)
	if err != nil {
		t.Fatalf("expected nil err got %v", err)
	}
	if inst.Name != in.InstanceName {
		t.Fatalf("name mismatch")
	}
	if inst.Token == "" {
		t.Fatalf("expected generated token")
	}
	if !inst.Settings.RejectCall {
		t.Fatalf("expected rejectCall true")
	}
	if inst.Webhook.URL != in.Webhook.URL {
		t.Fatalf("expected webhook url propagated")
	}
	if inst.WebhookURL != in.Webhook.URL {
		t.Fatalf("expected legacy webhook url propagated")
	}
	list, _ := svc.List(ctx)
	if len(list) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(list))
	}
}

func TestCreateInstanceFlatSettings(t *testing.T) {
	repo := repositories.NewInMemoryInstanceRepo()
	waMgr := whatsapp.NewManager(logger.New("DEBUG").App)
	svc := services.NewInstanceService(repo, waMgr, nil)
	msg := "Sem chamadas"
	in := instance.CreateInstanceInput{
		InstanceName: "flat",
		RejectCall:   boolPtr(true),
		MsgCall:      &msg,
		Webhook: &instance.InstanceWebhook{
			URL: "http://example",
		},
	}
	ctx := context.Background()
	inst, err := svc.Create(ctx, in)
	if err != nil {
		t.Fatalf("expected nil err got %v", err)
	}
	if !inst.Settings.RejectCall {
		t.Fatalf("expected rejectCall true")
	}
	if inst.Settings.MsgCall != msg {
		t.Fatalf("expected msgCall propagated")
	}
}

func boolPtr(v bool) *bool { return &v }
