package services

import (
	"context"
	"fmt"

	"github.com/faeln1/go-whatsapp-api/internal/platform/whatsapp"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waWeb"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

type MessageEventListener interface {
	HandleMessage(ctx context.Context, instanceName string, evt *events.Message)
}

type ReceiptEventListener interface {
	HandleReceipt(ctx context.Context, instanceName string, evt *events.Receipt)
}

type SessionBootstrap struct {
	StoreFactory  *whatsapp.StoreFactory
	Manager       *whatsapp.Manager
	Log           waLog.Logger
	Events        MessageEventListener
	ReceiptEvents ReceiptEventListener
	GroupEvents   CommunityEventListener
}

func NewSessionBootstrap(f *whatsapp.StoreFactory, m *whatsapp.Manager, log waLog.Logger, events MessageEventListener) *SessionBootstrap {
	return &SessionBootstrap{StoreFactory: f, Manager: m, Log: log, Events: events}
}

// InitNewSession cria (ou carrega) device store e gera QR channel se necessário.
func (b *SessionBootstrap) InitNewSession(ctx context.Context, instanceName string) (qr <-chan whatsmeow.QRChannelItem, alreadyLogged bool, err error) {
	container, err := b.StoreFactory.NewDeviceStore(ctx, instanceName)
	if err != nil {
		return nil, false, err
	}
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, false, err
	}
	client := whatsmeow.NewClient(device, b.Log.Sub("Client"))

	if b.Events != nil || b.ReceiptEvents != nil || b.GroupEvents != nil {
		client.AddEventHandler(func(evt any) {
			switch e := evt.(type) {
			case *events.Message:
				if b.Events == nil {
					return
				}
				dup := cloneMessageEvent(e)
				if dup == nil {
					return
				}
				go b.Events.HandleMessage(context.Background(), instanceName, dup)

			case *events.Receipt:
				if b.ReceiptEvents != nil {
					go b.ReceiptEvents.HandleReceipt(context.Background(), instanceName, e)
				}

			case *events.GroupInfo:
				if b.GroupEvents == nil {
					return
				}
				dup := cloneGroupInfoEvent(e)
				if dup == nil {
					return
				}
				go b.GroupEvents.HandleGroupInfo(context.Background(), instanceName, dup)
			}
		})
	} else {
		client.AddEventHandler(func(evt any) {})
	}

	var qrChan <-chan whatsmeow.QRChannelItem
	if device.ID == nil { // novo login
		// IMPORTANTE: Pegar o QR Channel ANTES de conectar
		qrChan, err = client.GetQRChannel(context.Background())
		if err != nil {
			return nil, false, fmt.Errorf("failed to get QR channel: %w", err)
		}
		// ENTÃO conectar para que o QR seja gerado
		if err = client.Connect(); err != nil {
			return nil, false, fmt.Errorf("connect failed: %w", err)
		}
	} else {
		// Já logado, apenas conectar
		if err = client.Connect(); err != nil {
			return nil, false, fmt.Errorf("connect failed: %w", err)
		}
	}

	b.Manager.AttachClient(instanceName, device, client, qrChan)
	if sess, ok := b.Manager.Get(instanceName); ok {
		b.Manager.StartEventLoop(sess)
	} else {
		b.Manager.StartEventLoop(&whatsapp.Session{Name: instanceName, Client: client})
	}
	return qrChan, device.ID != nil, nil
}

func cloneMessageEvent(evt *events.Message) *events.Message {
	if evt == nil {
		return nil
	}
	dup := *evt
	if evt.Message != nil {
		if msg, ok := proto.Clone(evt.Message).(*waE2E.Message); ok {
			dup.Message = msg
		}
	}
	if evt.RawMessage != nil {
		if msg, ok := proto.Clone(evt.RawMessage).(*waE2E.Message); ok {
			dup.RawMessage = msg
		}
	}
	if evt.SourceWebMsg != nil {
		if raw, ok := proto.Clone(evt.SourceWebMsg).(*waWeb.WebMessageInfo); ok {
			dup.SourceWebMsg = raw
		}
	}
	return &dup
}

func cloneGroupInfoEvent(evt *events.GroupInfo) *events.GroupInfo {
	if evt == nil {
		return nil
	}
	dup := *evt
	if len(evt.Join) > 0 {
		dup.Join = append([]types.JID(nil), evt.Join...)
	}
	if len(evt.Leave) > 0 {
		dup.Leave = append([]types.JID(nil), evt.Leave...)
	}
	return &dup
}
