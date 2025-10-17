package services

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
	"unicode"

	"github.com/faeln1/go-whatsapp-api/internal/domain/message"
	"github.com/faeln1/go-whatsapp-api/internal/platform/whatsapp"
	"github.com/faeln1/go-whatsapp-api/pkg/storage"
	"go.mau.fi/whatsmeow"
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

const maxMediaSizeBytes = 64 * 1024 * 1024 // 64MB limit per attachment

var errMediaTooLarge = errors.New("media payload exceeds 64MB limit")

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

	data, fileName, mimeType, err := s.extractMediaPayload(ctx, in)
	if err != nil {
		return out, err
	}
	if len(data) == 0 {
		return out, errors.New("media payload is empty")
	}

	mimeType = normalizeContentType(mimeType, data)
	kind, mediaType := inferMediaKind(in.MediaType, mimeType, fileName)
	caption := strings.TrimSpace(in.Caption)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	if kind == "document" {
		fileName = ensureFileName(fileName, mimeType)
	} else {
		fileName = sanitizeFileName(fileName)
	}

	uploadResp, err := sess.Client.Upload(ctx, data, mediaType)
	if err != nil {
		return out, err
	}
	if uploadResp.FileLength == 0 {
		uploadResp.FileLength = uint64(len(data))
	}

	msg, messageType := buildMediaMessage(uploadResp, kind, mimeType, fileName, caption)

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
		Message:          message.MessageBody{Conversation: caption},
		MessageType:      messageType,
		MessageTimestamp: time.Now().Unix(),
		InstanceID:       sess.ID,
		Source:           "unknown",
	}

	return out, nil
}

func (s *messageService) extractMediaPayload(ctx context.Context, in message.SendMediaInput) ([]byte, string, string, error) {
	mimeType := strings.TrimSpace(in.MimeType)
	if in.File != nil {
		defer in.File.Close()
		data, err := readAllLimited(in.File, maxMediaSizeBytes)
		if err != nil {
			return nil, "", "", err
		}
		name := strings.TrimSpace(in.FileName)
		if name == "" && in.FileHeader != nil {
			name = in.FileHeader.Filename
		}
		return data, name, mimeType, nil
	}

	media := strings.TrimSpace(in.Media)
	if media == "" {
		return nil, "", "", errors.New("media field is required")
	}
	if strings.HasPrefix(media, "http://") || strings.HasPrefix(media, "https://") {
		return s.downloadMedia(ctx, media, in.FileName, mimeType)
	}

	data, name, detectedMime, err := decodeBase64Media(media, in.FileName, mimeType)
	if err != nil {
		return nil, "", "", err
	}
	if mimeType == "" {
		mimeType = detectedMime
	}
	if strings.TrimSpace(name) == "" {
		name = in.FileName
	}
	return data, name, mimeType, nil
}

func (s *messageService) downloadMedia(ctx context.Context, mediaURL, fallbackName, currentMime string) ([]byte, string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to create media request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to fetch media: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, "", "", fmt.Errorf("media download failed with status %d", resp.StatusCode)
	}
	if resp.ContentLength > 0 && resp.ContentLength > int64(maxMediaSizeBytes) {
		return nil, "", "", errMediaTooLarge
	}
	data, err := readAllLimited(resp.Body, maxMediaSizeBytes)
	if err != nil {
		return nil, "", "", err
	}
	mimeType := currentMime
	if mimeType == "" {
		mimeType = strings.TrimSpace(resp.Header.Get("Content-Type"))
	}
	name := strings.TrimSpace(fallbackName)
	if name == "" {
		name = filenameFromDisposition(resp.Header.Get("Content-Disposition"))
	}
	if name == "" {
		if u, err := url.Parse(mediaURL); err == nil {
			candidate := path.Base(u.Path)
			if candidate != "" && candidate != "." && candidate != "/" {
				name = candidate
			}
		}
	}
	return data, name, mimeType, nil
}

func decodeBase64Media(raw, fallbackName, fallbackMime string) ([]byte, string, string, error) {
	data := raw
	name := strings.TrimSpace(fallbackName)
	mimeType := strings.TrimSpace(fallbackMime)
	if strings.HasPrefix(data, "data:") {
		idx := strings.Index(data, ",")
		if idx <= 0 {
			return nil, "", "", errors.New("invalid data URI")
		}
		head := data[5:idx]
		data = data[idx+1:]
		parts := strings.Split(head, ";")
		if len(parts) > 0 && strings.Contains(parts[0], "/") {
			if mimeType == "" {
				mimeType = parts[0]
			}
		}
		for _, part := range parts[1:] {
			if strings.EqualFold(part, "base64") {
				continue
			}
			if strings.HasPrefix(strings.ToLower(part), "name=") || strings.HasPrefix(strings.ToLower(part), "filename=") {
				val := part[strings.Index(part, "=")+1:]
				val = strings.Trim(val, "\"'")
				if decoded, err := url.QueryUnescape(val); err == nil {
					val = decoded
				}
				if val != "" {
					name = val
				}
			}
		}
	}
	data = strings.TrimSpace(data)
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(data)
	}
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to decode media payload: %w", err)
	}
	if len(decoded) > maxMediaSizeBytes {
		return nil, "", "", errMediaTooLarge
	}
	return decoded, name, mimeType, nil
}

func readAllLimited(r io.Reader, limit int) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, errMediaTooLarge
	}
	return data, nil
}

func filenameFromDisposition(header string) string {
	if strings.TrimSpace(header) == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(header)
	if err != nil {
		return ""
	}
	if v, ok := params["filename*"]; ok {
		if strings.HasPrefix(strings.ToLower(v), "utf-8''") {
			if decoded, err := url.QueryUnescape(v[7:]); err == nil {
				return decoded
			}
		}
		return v
	}
	if v, ok := params["filename"]; ok {
		return v
	}
	return ""
}

func inferMediaKind(explicit, mimeType, fileName string) (string, whatsmeow.MediaType) {
	if kind := strings.ToLower(strings.TrimSpace(explicit)); kind != "" {
		switch kind {
		case "image":
			return "image", whatsmeow.MediaImage
		case "video":
			return "video", whatsmeow.MediaVideo
		case "audio":
			return "audio", whatsmeow.MediaAudio
		case "document", "file", "doc", "pdf":
			return "document", whatsmeow.MediaDocument
		}
	}
	lowerMime := strings.ToLower(strings.TrimSpace(mimeType))
	switch {
	case strings.HasPrefix(lowerMime, "image/"):
		return "image", whatsmeow.MediaImage
	case strings.HasPrefix(lowerMime, "video/"):
		return "video", whatsmeow.MediaVideo
	case strings.HasPrefix(lowerMime, "audio/"):
		return "audio", whatsmeow.MediaAudio
	case strings.HasSuffix(strings.ToLower(fileName), ".gif"):
		return "image", whatsmeow.MediaImage
	default:
		return "document", whatsmeow.MediaDocument
	}
}

func ensureFileName(name, mimeType string) string {
	clean := sanitizeFileName(name)
	if clean != "" && strings.Contains(clean, ".") {
		return clean
	}
	if clean == "" {
		clean = "document"
	}
	ext := ""
	if mimeType != "" {
		if exts, _ := mime.ExtensionsByType(mimeType); len(exts) > 0 {
			ext = exts[0]
		}
	}
	if ext == "" {
		ext = ".bin"
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return sanitizeFileName(clean + ext)
}

func buildMediaMessage(upload whatsmeow.UploadResponse, kind, mimeType, fileName, caption string) (*waProto.Message, string) {
	fileLength := upload.FileLength
	switch kind {
	case "image":
		img := &waProto.ImageMessage{
			URL:           proto.String(upload.URL),
			DirectPath:    proto.String(upload.DirectPath),
			MediaKey:      upload.MediaKey,
			FileEncSHA256: upload.FileEncSHA256,
			FileSHA256:    upload.FileSHA256,
			FileLength:    proto.Uint64(fileLength),
			Mimetype:      proto.String(mimeType),
		}
		if caption != "" {
			img.Caption = proto.String(caption)
		}
		return &waProto.Message{ImageMessage: img}, "imageMessage"
	case "video":
		vid := &waProto.VideoMessage{
			URL:           proto.String(upload.URL),
			DirectPath:    proto.String(upload.DirectPath),
			MediaKey:      upload.MediaKey,
			FileEncSHA256: upload.FileEncSHA256,
			FileSHA256:    upload.FileSHA256,
			FileLength:    proto.Uint64(fileLength),
			Mimetype:      proto.String(mimeType),
		}
		if caption != "" {
			vid.Caption = proto.String(caption)
		}
		return &waProto.Message{VideoMessage: vid}, "videoMessage"
	case "audio":
		aud := &waProto.AudioMessage{
			URL:           proto.String(upload.URL),
			DirectPath:    proto.String(upload.DirectPath),
			MediaKey:      upload.MediaKey,
			FileEncSHA256: upload.FileEncSHA256,
			FileSHA256:    upload.FileSHA256,
			FileLength:    proto.Uint64(fileLength),
			Mimetype:      proto.String(mimeType),
		}
		return &waProto.Message{AudioMessage: aud}, "audioMessage"
	default:
		doc := &waProto.DocumentMessage{
			URL:           proto.String(upload.URL),
			DirectPath:    proto.String(upload.DirectPath),
			MediaKey:      upload.MediaKey,
			FileEncSHA256: upload.FileEncSHA256,
			FileSHA256:    upload.FileSHA256,
			FileLength:    proto.Uint64(fileLength),
			Mimetype:      proto.String(mimeType),
			FileName:      proto.String(fileName),
		}
		if fileName != "" {
			doc.Title = proto.String(fileName)
		}
		if caption != "" {
			doc.Caption = proto.String(caption)
		}
		return &waProto.Message{DocumentMessage: doc}, "documentMessage"
	}
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
