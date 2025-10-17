package services

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"

	"github.com/faeln1/go-whatsapp-api/internal/domain/message"
	"github.com/faeln1/go-whatsapp-api/internal/platform/whatsapp"
	"github.com/faeln1/go-whatsapp-api/pkg/storage"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

type MessageService interface {
	SendText(ctx context.Context, in message.SendTextInput) (message.SendTextOutput, error)
	SendMedia(ctx context.Context, in message.SendMediaInput) (message.SendTextOutput, error)
	SendStatus(ctx context.Context, in message.SendStatusInput) (message.SendTextOutput, error)
	SendAudio(ctx context.Context, in message.SendAudioInput) (message.SendTextOutput, error)
	SendSticker(ctx context.Context, in message.SendStickerInput) (message.SendTextOutput, error)
	SendLocation(ctx context.Context, in message.SendLocationInput) (message.SendTextOutput, error)
	SendContact(ctx context.Context, in message.SendContactInput) (message.SendTextOutput, error)
	SendReaction(ctx context.Context, in message.SendReactionInput) (message.SendTextOutput, error)
	SendPoll(ctx context.Context, in message.SendPollInput) (message.SendTextOutput, error)
	SendList(ctx context.Context, in message.SendListInput) (message.SendTextOutput, error)
	SendButtons(ctx context.Context, in message.SendButtonInput) (message.SendTextOutput, error)
}

type messageService struct {
	waMgr   *whatsapp.Manager
	storage storage.Service
}

func NewMessageService(waMgr *whatsapp.Manager, storage storage.Service) MessageService {
	return &messageService{waMgr: waMgr, storage: storage}
}

func (s *messageService) SendText(ctx context.Context, in message.SendTextInput) (message.SendTextOutput, error) {
	out := message.SendTextOutput{}
	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return out, err
	}
	jid, err := resolveDestination(in.To, in.Number)
	if err != nil {
		return out, err
	}

	if in.Delay > 0 {
		time.Sleep(time.Duration(in.Delay) * time.Millisecond)
	}

	msg := &waProto.Message{Conversation: proto.String(in.Text)}

	// TODO: Implementar linkPreview, mentionsEveryOne, mentioned, quoted

	msgID, err := sess.Client.SendMessage(ctx, jid, msg)
	if err != nil {
		return out, err
	}

	pushName := "Você"
	if sess.Client != nil && sess.Client.Store != nil && sess.Client.Store.PushName != "" {
		pushName = sess.Client.Store.PushName
	}

	out = message.SendTextOutput{
		Key: message.MessageKey{
			RemoteJID: jid.String(),
			FromMe:    true,
			ID:        msgID.ID,
		},
		PushName:         pushName,
		Status:           "PENDING",
		Message:          message.MessageBody{Conversation: in.Text},
		MessageType:      "conversation",
		MessageTimestamp: time.Now().Unix(),
		InstanceID:       sess.ID,
		Source:           "unknown",
	}

	return out, nil
}

func (s *messageService) SendMedia(ctx context.Context, in message.SendMediaInput) (message.SendTextOutput, error) {
	out := message.SendTextOutput{}
	if _, err := s.readySession(in.InstanceID); err != nil {
		return out, err
	}
	if _, err := resolveDestination(in.To, in.Number); err != nil {
		return out, err
	}

	if in.Delay > 0 {
		time.Sleep(time.Duration(in.Delay) * time.Millisecond)
	}

	// TODO: Implementar upload de mídia via URL ou base64
	// TODO: Suportar quoted, mentioned, etc

	return out, errors.New("media sending not implemented yet")
}

func (s *messageService) SendStatus(ctx context.Context, in message.SendStatusInput) (message.SendTextOutput, error) {
	out := message.SendTextOutput{}
	if _, err := s.readySession(in.InstanceID); err != nil {
		return out, err
	}

	// TODO: Implementar envio de status
	// Requer construção específica para broadcast list do status
	// StatusBroadcast JID: status@broadcast
	_ = in

	return out, errors.New("status sending not implemented yet")
}

func (s *messageService) SendAudio(ctx context.Context, in message.SendAudioInput) (message.SendTextOutput, error) {
	out := message.SendTextOutput{}
	if _, err := s.readySession(in.InstanceID); err != nil {
		return out, err
	}
	if _, err := resolveDestination(in.To, in.Number); err != nil {
		return out, err
	}

	if in.Delay > 0 {
		time.Sleep(time.Duration(in.Delay) * time.Millisecond)
	}

	// TODO: Implementar envio de áudio (upload, quoted, mentions)

	return out, errors.New("audio sending not implemented yet")
}

func (s *messageService) SendSticker(ctx context.Context, in message.SendStickerInput) (message.SendTextOutput, error) {
	out := message.SendTextOutput{}
	if _, err := s.readySession(in.InstanceID); err != nil {
		return out, err
	}
	if _, err := resolveDestination(in.To, in.Number); err != nil {
		return out, err
	}

	if in.Delay > 0 {
		time.Sleep(time.Duration(in.Delay) * time.Millisecond)
	}

	// TODO: Implementar envio de figurinha (upload, quoted, mentions)

	return out, errors.New("sticker sending not implemented yet")
}

func (s *messageService) SendLocation(ctx context.Context, in message.SendLocationInput) (message.SendTextOutput, error) {
	out := message.SendTextOutput{}
	if _, err := s.readySession(in.InstanceID); err != nil {
		return out, err
	}
	if _, err := resolveDestination(in.To, in.Number); err != nil {
		return out, err
	}

	if in.Delay > 0 {
		time.Sleep(time.Duration(in.Delay) * time.Millisecond)
	}

	// TODO: Implementar envio de localização (quoted, mentions)

	return out, errors.New("location sending not implemented yet")
}

func (s *messageService) SendContact(ctx context.Context, in message.SendContactInput) (message.SendTextOutput, error) {
	out := message.SendTextOutput{}
	if _, err := s.readySession(in.InstanceID); err != nil {
		return out, err
	}
	if _, err := resolveDestination(in.To, in.Number); err != nil {
		return out, err
	}
	if len(in.Contact) == 0 {
		return out, errors.New("invalid contact payload")
	}

	if in.Delay > 0 {
		time.Sleep(time.Duration(in.Delay) * time.Millisecond)
	}

	// TODO: Implementar envio de contatos (vCard)

	return out, errors.New("contact sending not implemented yet")
}

func (s *messageService) SendReaction(ctx context.Context, in message.SendReactionInput) (message.SendTextOutput, error) {
	out := message.SendTextOutput{}
	if _, err := s.readySession(in.InstanceID); err != nil {
		return out, err
	}
	if strings.TrimSpace(in.Reaction) == "" {
		return out, errors.New("invalid reaction")
	}
	if _, err := parseDestinationJID(in.Key.RemoteJID); err != nil {
		return out, err
	}
	if strings.TrimSpace(in.Key.ID) == "" {
		return out, errors.New("invalid reaction")
	}

	if in.Delay > 0 {
		time.Sleep(time.Duration(in.Delay) * time.Millisecond)
	}

	// TODO: Implementar envio de reação

	return out, errors.New("reaction sending not implemented yet")
}

func (s *messageService) SendPoll(ctx context.Context, in message.SendPollInput) (message.SendTextOutput, error) {
	out := message.SendTextOutput{}
	if _, err := s.readySession(in.InstanceID); err != nil {
		return out, err
	}
	if _, err := resolveDestination(in.To, in.Number); err != nil {
		return out, err
	}
	if strings.TrimSpace(in.Name) == "" || len(in.Values) == 0 {
		return out, errors.New("invalid poll payload")
	}
	if in.SelectableCount <= 0 {
		return out, errors.New("invalid poll payload")
	}

	if in.Delay > 0 {
		time.Sleep(time.Duration(in.Delay) * time.Millisecond)
	}

	// TODO: Implementar envio de enquetes

	return out, errors.New("poll sending not implemented yet")
}

func (s *messageService) SendList(ctx context.Context, in message.SendListInput) (message.SendTextOutput, error) {
	out := message.SendTextOutput{}
	if _, err := s.readySession(in.InstanceID); err != nil {
		return out, err
	}
	if _, err := resolveDestination(in.To, in.Number); err != nil {
		return out, err
	}
	if strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Description) == "" {
		return out, errors.New("invalid list payload")
	}
	if len(in.Values) == 0 {
		return out, errors.New("invalid list payload")
	}

	if in.Delay > 0 {
		time.Sleep(time.Duration(in.Delay) * time.Millisecond)
	}

	// TODO: Implementar envio de listas interativas

	return out, errors.New("list sending not implemented yet")
}

func (s *messageService) SendButtons(ctx context.Context, in message.SendButtonInput) (message.SendTextOutput, error) {
	out := message.SendTextOutput{}
	if _, err := s.readySession(in.InstanceID); err != nil {
		return out, err
	}
	if _, err := resolveDestination(in.To, in.Number); err != nil {
		return out, err
	}
	if strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Description) == "" {
		return out, errors.New("invalid buttons payload")
	}
	if len(in.Buttons) == 0 {
		return out, errors.New("invalid buttons payload")
	}

	if in.Delay > 0 {
		time.Sleep(time.Duration(in.Delay) * time.Millisecond)
	}

	// TODO: Implementar envio de botões interativos

	return out, errors.New("buttons sending not implemented yet")
}

func (s *messageService) readySession(instanceID string) (*whatsapp.Session, error) {
	cleaned := strings.TrimSpace(instanceID)
	if cleaned == "" {
		return nil, errors.New("invalid instance id")
	}
	sess, ok := s.waMgr.Get(cleaned)
	if !ok {
		return nil, errors.New("instance not found")
	}
	if sess.Client == nil {
		return nil, errors.New("instance client not ready")
	}
	if !sess.Client.IsConnected() {
		return nil, errors.New("instance not connected")
	}
	return sess, nil
}

func resolveDestination(to, number string) (types.JID, error) {
	destination := strings.TrimSpace(number)
	if destination == "" {
		destination = strings.TrimSpace(to)
	}
	if destination == "" {
		return types.JID{}, errors.New("invalid recipient")
	}
	return parseDestinationJID(destination)
}

func parseDestinationJID(raw string) (types.JID, error) {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return types.JID{}, errors.New("invalid recipient")
	}
	if strings.Contains(cleaned, "@") {
		jid, err := types.ParseJID(cleaned)
		if err != nil {
			return types.JID{}, err
		}
		return jid, nil
	}

	digits := strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}
		return -1
	}, cleaned)
	if digits == "" {
		return types.JID{}, errors.New("invalid recipient")
	}
	return types.NewJID(digits, types.DefaultUserServer), nil
}
