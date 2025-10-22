package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/faeln1/go-whatsapp-api/internal/app/repositories"
	"github.com/faeln1/go-whatsapp-api/internal/domain/instance"
	"github.com/faeln1/go-whatsapp-api/internal/platform/whatsapp"
	"github.com/faeln1/go-whatsapp-api/pkg/storage"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type MessageEventHandler struct {
	repo             repositories.InstanceRepository
	waMgr            *whatsapp.Manager
	storage          storage.Service
	dispatcher       WebhookDispatcher
	analyticsService AnalyticsService
	log              waLog.Logger
}

func NewMessageEventHandler(repo repositories.InstanceRepository, waMgr *whatsapp.Manager, store storage.Service, dispatcher WebhookDispatcher, analytics AnalyticsService, log waLog.Logger) *MessageEventHandler {
	return &MessageEventHandler{
		repo:             repo,
		waMgr:            waMgr,
		storage:          store,
		dispatcher:       dispatcher,
		analyticsService: analytics,
		log:              log,
	}
}

func (h *MessageEventHandler) HandleMessage(ctx context.Context, instanceName string, evt *events.Message) {
	if h == nil || evt == nil || h.dispatcher == nil || h.repo == nil || h.waMgr == nil {
		return
	}

	sess, ok := h.waMgr.Get(instanceName)
	if !ok || sess == nil || sess.Client == nil {
		return
	}

	inst, err := h.repo.GetByName(context.Background(), instanceName)
	if err != nil {
		if !errors.Is(err, repositories.ErrInstanceNotFound) && h.log != nil {
			h.log.Errorf("messages.upsert instance=%s repository error: %v", instanceName, err)
		}
		return
	}
	if inst == nil {
		return
	}
	if inst.Webhook.URL == "" && inst.WebhookURL == "" {
		return
	}

	evt.UnwrapRaw()
	if evt.Message == nil && evt.RawMessage == nil {
		return
	}
	if evt.Message == nil {
		evt.Message = evt.RawMessage
	}

	uploads := h.replaceMedia(ctx, inst, sess, evt)

	messageType := detectMessageType(evt.Message)

	// Se for reação, processar no analytics
	if messageType == "reactionMessage" {
		go h.ProcessReaction(ctx, instanceName, evt)
	}

	if h.log != nil {
		h.log.Debugf("messages.upsert instance=%s chat=%s id=%s type=%s", inst.Name, evt.Info.Chat, evt.Info.ID, messageType)
	}

	messageMap, err := protoToMap(evt.Message)
	if err != nil {
		if h.log != nil {
			h.log.Errorf("messages.upsert instance=%s marshal error: %v", instanceName, err)
		}
		return
	}

	contextMap := extractContextInfo(evt.Message)

	ts := evt.Info.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}

	status := strings.ToUpper(strings.TrimSpace(evt.Info.Type))
	if status == "" {
		status = "UNKNOWN"
	}
	source := strings.TrimSpace(evt.Info.Category)
	if source == "" {
		source = "unknown"
	}

	key := map[string]any{
		"remoteJid": evt.Info.Chat.String(),
		"fromMe":    evt.Info.IsFromMe,
		"id":        string(evt.Info.ID),
	}
	if !evt.Info.RecipientAlt.IsEmpty() {
		key["remoteJidAlt"] = evt.Info.RecipientAlt.String()
	}
	if !evt.Info.Sender.IsEmpty() {
		key["participant"] = evt.Info.Sender.String()
	}
	if !evt.Info.SenderAlt.IsEmpty() {
		key["participantAlt"] = evt.Info.SenderAlt.String()
	}
	if evt.Info.AddressingMode != "" {
		key["addressingMode"] = string(evt.Info.AddressingMode)
	}
	if !evt.Info.BroadcastListOwner.IsEmpty() {
		key["broadcastListOwner"] = evt.Info.BroadcastListOwner.String()
	}
	if evt.Info.DeviceSentMeta != nil {
		key["deviceSentMeta"] = map[string]any{
			"destinationJid": evt.Info.DeviceSentMeta.DestinationJID,
			"phash":          evt.Info.DeviceSentMeta.Phash,
		}
	}

	payload := map[string]any{
		"key":              key,
		"pushName":         evt.Info.PushName,
		"status":           status,
		"message":          messageMap,
		"messageType":      messageType,
		"messageTimestamp": ts.Unix(),
		"instanceId":       string(inst.ID),
		"source":           source,
		"isViewOnce":       evt.IsViewOnce,
		"isEdit":           evt.IsEdit,
	}

	if len(contextMap) > 0 {
		payload["contextInfo"] = contextMap
	}
	if len(uploads) > 0 {
		payload["media"] = uploads
	}

	dispatchCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	targetURL := strings.TrimSpace(inst.Webhook.URL)
	if targetURL == "" {
		targetURL = strings.TrimSpace(inst.WebhookURL)
	}
	delivered, err := h.dispatcher.Dispatch(dispatchCtx, inst, "messages.upsert", payload)
	if err != nil {
		if h.log != nil {
			h.log.Errorf("messages.upsert instance=%s dispatch error: %v", instanceName, err)
		}
	} else if delivered {
		if h.log != nil {
			h.log.Debugf("messages.upsert instance=%s dispatched to %s", inst.Name, targetURL)
		}
	} else if h.log != nil {
		h.log.Debugf("messages.upsert instance=%s webhook skipped (disabled or filtered)", inst.Name)
	}
}

func (h *MessageEventHandler) replaceMedia(ctx context.Context, inst *instance.Instance, sess *whatsapp.Session, evt *events.Message) []map[string]string {
	if h.storage == nil || sess == nil || sess.Client == nil || evt.Message == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var uploads []map[string]string
	msg := evt.Message

	if image := msg.GetImageMessage(); image != nil {
		if data, err := sess.Client.Download(ctx, image); err != nil {
			if h.log != nil {
				h.log.Warnf("messages.upsert instance=%s image download failed: %v", inst.Name, err)
			}
		} else if len(data) > 0 {
			if url, ct, err := h.putMedia(ctx, inst, evt, data, image.GetMimetype(), ".jpg", ""); err != nil {
				if h.log != nil {
					h.log.Errorf("messages.upsert instance=%s image upload failed: %v", inst.Name, err)
				}
			} else if url != "" {
				image.URL = proto.String(url)
				uploads = append(uploads, map[string]string{"type": "image", "url": url, "mimeType": ct})
			}
		}
	}

	if audio := msg.GetAudioMessage(); audio != nil {
		if data, err := sess.Client.Download(ctx, audio); err != nil {
			if h.log != nil {
				h.log.Warnf("messages.upsert instance=%s audio download failed: %v", inst.Name, err)
			}
		} else if len(data) > 0 {
			if url, ct, err := h.putMedia(ctx, inst, evt, data, audio.GetMimetype(), ".ogg", ""); err != nil {
				if h.log != nil {
					h.log.Errorf("messages.upsert instance=%s audio upload failed: %v", inst.Name, err)
				}
			} else if url != "" {
				audio.URL = proto.String(url)
				uploads = append(uploads, map[string]string{"type": "audio", "url": url, "mimeType": ct})
			}
		}
	}

	if video := msg.GetVideoMessage(); video != nil {
		if data, err := sess.Client.Download(ctx, video); err != nil {
			if h.log != nil {
				h.log.Warnf("messages.upsert instance=%s video download failed: %v", inst.Name, err)
			}
		} else if len(data) > 0 {
			if url, ct, err := h.putMedia(ctx, inst, evt, data, video.GetMimetype(), ".mp4", ""); err != nil {
				if h.log != nil {
					h.log.Errorf("messages.upsert instance=%s video upload failed: %v", inst.Name, err)
				}
			} else if url != "" {
				video.URL = proto.String(url)
				uploads = append(uploads, map[string]string{"type": "video", "url": url, "mimeType": ct})
			}
		}
	}

	if doc := msg.GetDocumentMessage(); doc != nil {
		if data, err := sess.Client.Download(ctx, doc); err != nil {
			if h.log != nil {
				h.log.Warnf("messages.upsert instance=%s document download failed: %v", inst.Name, err)
			}
		} else if len(data) > 0 {
			fileName := doc.GetFileName()
			if url, ct, err := h.putMedia(ctx, inst, evt, data, doc.GetMimetype(), ".bin", fileName); err != nil {
				if h.log != nil {
					h.log.Errorf("messages.upsert instance=%s document upload failed: %v", inst.Name, err)
				}
			} else if url != "" {
				doc.URL = proto.String(url)
				uploads = append(uploads, map[string]string{"type": "document", "url": url, "mimeType": ct})
			}
		}
	}

	if sticker := msg.GetStickerMessage(); sticker != nil {
		if data, err := sess.Client.Download(ctx, sticker); err != nil {
			if h.log != nil {
				h.log.Warnf("messages.upsert instance=%s sticker download failed: %v", inst.Name, err)
			}
		} else if len(data) > 0 {
			if url, ct, err := h.putMedia(ctx, inst, evt, data, sticker.GetMimetype(), ".webp", ""); err != nil {
				if h.log != nil {
					h.log.Errorf("messages.upsert instance=%s sticker upload failed: %v", inst.Name, err)
				}
			} else if url != "" {
				sticker.URL = proto.String(url)
				uploads = append(uploads, map[string]string{"type": "sticker", "url": url, "mimeType": ct})
			}
		}
	}

	return uploads
}

func (h *MessageEventHandler) putMedia(ctx context.Context, inst *instance.Instance, evt *events.Message, data []byte, mimeType, fallbackExt, fileName string) (string, string, error) {
	if len(data) == 0 {
		return "", "", nil
	}

	ct := normalizeContentType(mimeType, data)
	ext := ""
	if fileName != "" {
		if idx := strings.LastIndex(fileName, "."); idx != -1 {
			ext = fileName[idx:]
		}
	}
	if ext == "" && ct != "" {
		if exts, _ := mime.ExtensionsByType(ct); len(exts) > 0 {
			ext = exts[0]
		}
	}
	if ext == "" {
		ext = fallbackExt
	}
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	cleanInst := sanitizeSegment(inst.Name)
	cleanMsg := sanitizeSegment(string(evt.Info.ID))
	if cleanMsg == "" {
		cleanMsg = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	ts := evt.Info.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	folder := ts.UTC().Format("2006/01/02")

	sanitizedName := sanitizeFileName(fileName)
	if sanitizedName == "" {
		sanitizedName = cleanMsg + ext
	} else if ext != "" && !strings.HasSuffix(strings.ToLower(sanitizedName), strings.ToLower(ext)) {
		sanitizedName += ext
	}

	key := fmt.Sprintf("instances/%s/messages/%s/%s/%s", cleanInst, folder, cleanMsg, sanitizedName)

	url, err := h.storage.PutObject(ctx, storage.UploadInput{
		Key:         key,
		ContentType: ct,
		Body:        bytes.NewReader(data),
		Size:        int64(len(data)),
	})
	return url, ct, err
}

func protoToMap(msg proto.Message) (map[string]any, error) {
	if msg == nil {
		return nil, nil
	}

	opts := protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: false}
	payload, err := opts.Marshal(msg)
	if err != nil {
		return nil, err
	}

	var out map[string]any
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func extractContextInfo(msg *waProto.Message) map[string]any {
	if msg == nil || msg.MessageContextInfo == nil {
		return nil
	}
	ctxMap, err := protoToMap(msg.MessageContextInfo)
	if err != nil {
		return nil
	}
	return ctxMap
}

func detectMessageType(msg *waProto.Message) string {
	if msg == nil {
		return "unknown"
	}
	switch {
	case msg.GetConversation() != "":
		return "conversation"
	case msg.GetExtendedTextMessage() != nil:
		return "extendedTextMessage"
	case msg.GetImageMessage() != nil:
		return "imageMessage"
	case msg.GetVideoMessage() != nil:
		return "videoMessage"
	case msg.GetAudioMessage() != nil:
		return "audioMessage"
	case msg.GetDocumentMessage() != nil:
		return "documentMessage"
	case msg.GetStickerMessage() != nil:
		return "stickerMessage"
	case msg.GetContactMessage() != nil:
		return "contactMessage"
	case msg.GetLocationMessage() != nil:
		return "locationMessage"
	case msg.GetLiveLocationMessage() != nil:
		return "liveLocationMessage"
	case msg.GetReactionMessage() != nil:
		return "reactionMessage"
	default:
		return "unknown"
	}
}

func normalizeContentType(raw string, data []byte) string {
	v := strings.TrimSpace(raw)
	if idx := strings.Index(v, ";"); idx != -1 {
		v = strings.TrimSpace(v[:idx])
	}
	if v == "" {
		v = http.DetectContentType(data)
	}
	return v
}

func sanitizeSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, " ", "_")
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, value)
}

func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, " ", "_")
	clean := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '_'
	}, name)
	clean = strings.Trim(clean, "._")
	if clean == "" {
		return ""
	}
	return clean
}

// HandleReceipt processa confirmações de leitura/visualização
func (h *MessageEventHandler) HandleReceipt(ctx context.Context, instanceName string, evt *events.Receipt) {
	if h == nil || evt == nil || h.analyticsService == nil {
		return
	}

	var (
		sess       *whatsapp.Session
		viewerName string
	)
	if h.waMgr != nil && instanceName != "" {
		if candidate, ok := h.waMgr.Get(instanceName); ok {
			sess = candidate
		}
	}

	viewerJID := evt.Sender

	if sess != nil && sess.Client != nil && sess.Client.Store != nil && !viewerJID.IsEmpty() {
		contact, err := sess.Client.Store.Contacts.GetContact(ctx, viewerJID.ToNonAD())
		if err == nil {
			switch {
			case contact.FullName != "":
				viewerName = contact.FullName
			case contact.PushName != "":
				viewerName = contact.PushName
			case contact.BusinessName != "":
				viewerName = contact.BusinessName
			case contact.FirstName != "":
				viewerName = contact.FirstName
			}
		}
	}

	if viewerName == "" {
		viewerName = strings.TrimSpace(viewerJID.User)
	}

	viewedAt := evt.Timestamp
	if viewedAt.IsZero() {
		viewedAt = time.Now().UTC()
	}

	// Processar cada mensagem confirmada
	for _, msgID := range evt.MessageIDs {
		// Registrar visualização no analytics
		if err := h.analyticsService.RecordMessageView(ctx, msgID, viewerJID.String(), viewerName, viewedAt); err != nil {
			if h.log != nil {
				h.log.Warnf("Failed to record message view for %s: %v", msgID, err)
			}
		}
	}
}

// ProcessReaction processa reações recebidas em mensagens
func (h *MessageEventHandler) ProcessReaction(ctx context.Context, instanceName string, evt *events.Message) {
	if h == nil || evt == nil || evt.Message == nil || h.analyticsService == nil {
		return
	}

	reaction := evt.Message.GetReactionMessage()
	if reaction == nil {
		return
	}

	// Extrair informações da reação
	key := reaction.GetKey()
	if key == nil {
		return
	}

	messageID := key.GetId()
	if messageID == "" {
		return
	}

	reactorJID := evt.Info.Sender.String()
	if reactorJID == "" {
		reactorJID = evt.Info.Chat.String()
	}

	reactorName := evt.Info.PushName
	reactionText := reaction.GetText()

	// Registrar reação no analytics
	if err := h.analyticsService.RecordMessageReaction(ctx, messageID, reactorJID, reactorName, reactionText); err != nil {
		if h.log != nil {
			h.log.Warnf("Failed to record message reaction for %s: %v", messageID, err)
		}
	}
}

var _ MessageEventListener = (*MessageEventHandler)(nil)
var _ ReceiptEventListener = (*MessageEventHandler)(nil)
