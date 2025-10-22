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
	"regexp"
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

	var msg *waProto.Message
	var messageType string

	// Check if link preview is requested and text contains a URL
	if in.LinkPreview && containsURL(in.Text) {
		// Extract first URL from text
		url := extractFirstURL(in.Text)
		if url != "" {
			// Try to generate link preview using enhanced method
			preview, err := s.generateLinkPreviewEnhanced(ctx, sess, url)
			if err == nil && preview != nil {
				// Send as ExtendedTextMessage with preview
				extMsg := &waProto.ExtendedTextMessage{
					Text:        proto.String(in.Text),
					MatchedText: proto.String(url),
				}

				if preview.Title != nil {
					extMsg.Title = preview.Title
				}
				if preview.Description != nil {
					extMsg.Description = preview.Description
				}
				if len(preview.JPEGThumbnail) > 0 {
					extMsg.JPEGThumbnail = preview.JPEGThumbnail
				}
				if preview.ThumbnailDirectPath != nil {
					extMsg.ThumbnailDirectPath = preview.ThumbnailDirectPath
				}
				if len(preview.ThumbnailSHA256) > 0 {
					extMsg.ThumbnailSHA256 = preview.ThumbnailSHA256
				}
				if len(preview.ThumbnailEncSHA256) > 0 {
					extMsg.ThumbnailEncSHA256 = preview.ThumbnailEncSHA256
				}
				if len(preview.MediaKey) > 0 {
					extMsg.MediaKey = preview.MediaKey
				}
				if preview.MediaKeyTimestamp != nil {
					extMsg.MediaKeyTimestamp = preview.MediaKeyTimestamp
				}

				msg = &waProto.Message{
					ExtendedTextMessage: extMsg,
				}
				messageType = "extendedTextMessage"
			}
		}
	}

	// Fallback to simple conversation message if no preview generated
	if msg == nil {
		msg = &waProto.Message{Conversation: proto.String(in.Text)}
		messageType = "conversation"
	}

	// TODO: Implementar mentionsEveryOne, mentioned, quoted

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
		MessageType:      messageType,
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
	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return out, err
	}

	statusMsg := in.StatusMessage
	statusType := strings.ToLower(strings.TrimSpace(statusMsg.Type))
	if statusType == "" {
		return out, errors.New("status type is required")
	}

	// Status broadcast JID
	statusJID := types.NewJID("", types.BroadcastServer)

	var protoMsg *waProto.Message
	var messageType string

	switch statusType {
	case "text":
		if strings.TrimSpace(statusMsg.Content) == "" {
			return out, errors.New("content is required for text status")
		}

		// Build text status message
		extMsg := &waProto.ExtendedTextMessage{
			Text: proto.String(statusMsg.Content),
		}

		// Parse background color if provided
		if bgColor := strings.TrimSpace(statusMsg.BackgroundColor); bgColor != "" {
			// Remove # if present and convert hex to ARGB
			bgColor = strings.TrimPrefix(bgColor, "#")
			if len(bgColor) == 6 {
				// Convert RGB to ARGB (add FF for alpha)
				bgColor = "FF" + bgColor
			}
			// Parse hex color to uint32
			var argb uint32
			if _, err := fmt.Sscanf(bgColor, "%x", &argb); err == nil {
				extMsg.BackgroundArgb = proto.Uint32(argb)
			}
		}

		// Font can be set but we'll use default for now
		// Different WhatsApp versions may support different font types

		protoMsg = &waProto.Message{
			ExtendedTextMessage: extMsg,
		}
		messageType = "extendedTextMessage"

	case "image":
		data, _, mimeType, err := s.extractStatusMedia(ctx, statusMsg.Content)
		if err != nil {
			return out, fmt.Errorf("failed to extract image: %w", err)
		}

		mimeType = normalizeContentType(mimeType, data)
		if !strings.HasPrefix(mimeType, "image/") {
			mimeType = "image/jpeg"
		}

		uploadResp, err := sess.Client.Upload(ctx, data, whatsmeow.MediaImage)
		if err != nil {
			return out, fmt.Errorf("failed to upload image: %w", err)
		}

		img := &waProto.ImageMessage{
			URL:           proto.String(uploadResp.URL),
			DirectPath:    proto.String(uploadResp.DirectPath),
			MediaKey:      uploadResp.MediaKey,
			FileEncSHA256: uploadResp.FileEncSHA256,
			FileSHA256:    uploadResp.FileSHA256,
			FileLength:    proto.Uint64(uploadResp.FileLength),
			Mimetype:      proto.String(mimeType),
		}

		if caption := strings.TrimSpace(statusMsg.Caption); caption != "" {
			img.Caption = proto.String(caption)
		}

		protoMsg = &waProto.Message{ImageMessage: img}
		messageType = "imageMessage"

	case "video":
		data, _, mimeType, err := s.extractStatusMedia(ctx, statusMsg.Content)
		if err != nil {
			return out, fmt.Errorf("failed to extract video: %w", err)
		}

		mimeType = normalizeContentType(mimeType, data)
		if !strings.HasPrefix(mimeType, "video/") {
			mimeType = "video/mp4"
		}

		uploadResp, err := sess.Client.Upload(ctx, data, whatsmeow.MediaVideo)
		if err != nil {
			return out, fmt.Errorf("failed to upload video: %w", err)
		}

		vid := &waProto.VideoMessage{
			URL:           proto.String(uploadResp.URL),
			DirectPath:    proto.String(uploadResp.DirectPath),
			MediaKey:      uploadResp.MediaKey,
			FileEncSHA256: uploadResp.FileEncSHA256,
			FileSHA256:    uploadResp.FileSHA256,
			FileLength:    proto.Uint64(uploadResp.FileLength),
			Mimetype:      proto.String(mimeType),
		}

		if caption := strings.TrimSpace(statusMsg.Caption); caption != "" {
			vid.Caption = proto.String(caption)
		}

		protoMsg = &waProto.Message{VideoMessage: vid}
		messageType = "videoMessage"

	case "audio":
		data, _, mimeType, err := s.extractStatusMedia(ctx, statusMsg.Content)
		if err != nil {
			return out, fmt.Errorf("failed to extract audio: %w", err)
		}

		mimeType = normalizeContentType(mimeType, data)
		if !strings.HasPrefix(mimeType, "audio/") {
			mimeType = "audio/ogg; codecs=opus"
		}

		uploadResp, err := sess.Client.Upload(ctx, data, whatsmeow.MediaAudio)
		if err != nil {
			return out, fmt.Errorf("failed to upload audio: %w", err)
		}

		aud := &waProto.AudioMessage{
			URL:           proto.String(uploadResp.URL),
			DirectPath:    proto.String(uploadResp.DirectPath),
			MediaKey:      uploadResp.MediaKey,
			FileEncSHA256: uploadResp.FileEncSHA256,
			FileSHA256:    uploadResp.FileSHA256,
			FileLength:    proto.Uint64(uploadResp.FileLength),
			Mimetype:      proto.String(mimeType),
			PTT:           proto.Bool(false),
		}

		protoMsg = &waProto.Message{AudioMessage: aud}
		messageType = "audioMessage"

	default:
		return out, fmt.Errorf("unsupported status type: %s", statusType)
	}

	// Send to status broadcast
	resp, err := sess.Client.SendMessage(ctx, statusJID, protoMsg)
	if err != nil {
		return out, fmt.Errorf("failed to send status: %w", err)
	}

	pushName := "Você"
	if sess.Client != nil && sess.Client.Store != nil && sess.Client.Store.PushName != "" {
		pushName = sess.Client.Store.PushName
	}

	out = message.SendTextOutput{
		Key: message.MessageKey{
			RemoteJID: "status@broadcast",
			FromMe:    true,
			ID:        resp.ID,
		},
		PushName:         pushName,
		Status:           "PENDING",
		MessageType:      messageType,
		MessageTimestamp: resp.Timestamp.Unix(),
		InstanceID:       sess.ID,
		Source:           "unknown",
	}

	return out, nil
}

func (s *messageService) extractStatusMedia(ctx context.Context, content string) ([]byte, string, string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, "", "", errors.New("content is required for media status")
	}

	// Check if it's a URL
	if strings.HasPrefix(content, "http://") || strings.HasPrefix(content, "https://") {
		return s.downloadMedia(ctx, content, "", "")
	}

	// Otherwise, treat as base64
	data, fileName, mimeType, err := decodeBase64Media(content, "", "")
	if err != nil {
		return nil, "", "", err
	}

	return data, fileName, mimeType, nil
}

func (s *messageService) SendAudio(ctx context.Context, in message.SendAudioInput) (message.SendTextOutput, error) {
	out := message.SendTextOutput{}
	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return out, err
	}

	if strings.TrimSpace(in.Number) == "" {
		return out, errors.New("number is required")
	}
	if strings.TrimSpace(in.AudioMessage.Audio) == "" {
		return out, errors.New("audio is required")
	}

	dest, err := parseDestinationJID(in.Number)
	if err != nil {
		return out, err
	}

	delay := 0
	ptt := false
	if in.Options != nil {
		if in.Options.Delay > 0 {
			delay = in.Options.Delay
		}
		// If encoding is true, send as PTT (voice message)
		ptt = in.Options.Encoding
	}

	if delay > 0 {
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	// Extract audio data (URL or base64)
	data, _, mimeType, err := s.extractStatusMedia(ctx, in.AudioMessage.Audio)
	if err != nil {
		return out, fmt.Errorf("failed to extract audio: %w", err)
	}

	// Normalize MIME type
	mimeType = normalizeContentType(mimeType, data)
	if !strings.HasPrefix(mimeType, "audio/") {
		// Default to audio/ogg for PTT, audio/mp4 for regular audio
		if ptt {
			mimeType = "audio/ogg; codecs=opus"
		} else {
			mimeType = "audio/mp4"
		}
	}

	// Upload audio
	uploadResp, err := sess.Client.Upload(ctx, data, whatsmeow.MediaAudio)
	if err != nil {
		return out, fmt.Errorf("failed to upload audio: %w", err)
	}

	if uploadResp.FileLength == 0 {
		uploadResp.FileLength = uint64(len(data))
	}

	// Build audio message
	audioMsg := &waProto.AudioMessage{
		URL:           proto.String(uploadResp.URL),
		DirectPath:    proto.String(uploadResp.DirectPath),
		MediaKey:      uploadResp.MediaKey,
		FileEncSHA256: uploadResp.FileEncSHA256,
		FileSHA256:    uploadResp.FileSHA256,
		FileLength:    proto.Uint64(uploadResp.FileLength),
		Mimetype:      proto.String(mimeType),
		PTT:           proto.Bool(ptt),
	}

	protoMsg := &waProto.Message{
		AudioMessage: audioMsg,
	}

	// Send message
	resp, err := sess.Client.SendMessage(ctx, dest, protoMsg)
	if err != nil {
		return out, fmt.Errorf("failed to send audio: %w", err)
	}

	pushName := "Você"
	if sess.Client != nil && sess.Client.Store != nil && sess.Client.Store.PushName != "" {
		pushName = sess.Client.Store.PushName
	}

	out = message.SendTextOutput{
		Key: message.MessageKey{
			RemoteJID: dest.String(),
			FromMe:    true,
			ID:        resp.ID,
		},
		PushName:         pushName,
		Status:           "PENDING",
		MessageType:      "audioMessage",
		MessageTimestamp: resp.Timestamp.Unix(),
		InstanceID:       sess.ID,
		Source:           "unknown",
	}

	return out, nil
}

func (s *messageService) SendSticker(ctx context.Context, in message.SendStickerInput) (message.SendTextOutput, error) {
	out := message.SendTextOutput{}
	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return out, err
	}

	if strings.TrimSpace(in.Number) == "" {
		return out, errors.New("number is required")
	}
	if strings.TrimSpace(in.StickerMessage.Image) == "" {
		return out, errors.New("sticker image is required")
	}

	dest, err := parseDestinationJID(in.Number)
	if err != nil {
		return out, err
	}

	delay := 0
	if in.Options != nil && in.Options.Delay > 0 {
		delay = in.Options.Delay
	}

	if delay > 0 {
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	// Extract sticker image data (URL or base64)
	data, _, mimeType, err := s.extractStatusMedia(ctx, in.StickerMessage.Image)
	if err != nil {
		return out, fmt.Errorf("failed to extract sticker image: %w", err)
	}

	// Normalize MIME type - stickers should be image/webp
	mimeType = normalizeContentType(mimeType, data)
	if !strings.HasPrefix(mimeType, "image/") {
		mimeType = "image/webp"
	}

	// Upload sticker
	uploadResp, err := sess.Client.Upload(ctx, data, whatsmeow.MediaImage)
	if err != nil {
		return out, fmt.Errorf("failed to upload sticker: %w", err)
	}

	if uploadResp.FileLength == 0 {
		uploadResp.FileLength = uint64(len(data))
	}

	// Build sticker message
	stickerMsg := &waProto.StickerMessage{
		URL:           proto.String(uploadResp.URL),
		DirectPath:    proto.String(uploadResp.DirectPath),
		MediaKey:      uploadResp.MediaKey,
		FileEncSHA256: uploadResp.FileEncSHA256,
		FileSHA256:    uploadResp.FileSHA256,
		FileLength:    proto.Uint64(uploadResp.FileLength),
		Mimetype:      proto.String(mimeType),
	}

	protoMsg := &waProto.Message{
		StickerMessage: stickerMsg,
	}

	// Send message
	resp, err := sess.Client.SendMessage(ctx, dest, protoMsg)
	if err != nil {
		return out, fmt.Errorf("failed to send sticker: %w", err)
	}

	pushName := "Você"
	if sess.Client != nil && sess.Client.Store != nil && sess.Client.Store.PushName != "" {
		pushName = sess.Client.Store.PushName
	}

	out = message.SendTextOutput{
		Key: message.MessageKey{
			RemoteJID: dest.String(),
			FromMe:    true,
			ID:        resp.ID,
		},
		PushName:         pushName,
		Status:           "PENDING",
		MessageType:      "stickerMessage",
		MessageTimestamp: resp.Timestamp.Unix(),
		InstanceID:       sess.ID,
		Source:           "unknown",
	}

	return out, nil
}

func (s *messageService) SendLocation(ctx context.Context, in message.SendLocationInput) (message.SendTextOutput, error) {
	out := message.SendTextOutput{}
	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return out, err
	}

	if strings.TrimSpace(in.Number) == "" {
		return out, errors.New("number is required")
	}

	dest, err := parseDestinationJID(in.Number)
	if err != nil {
		return out, err
	}

	delay := 0
	if in.Options != nil && in.Options.Delay > 0 {
		delay = in.Options.Delay
	}

	if delay > 0 {
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	// Build location message
	locationMsg := &waProto.LocationMessage{
		DegreesLatitude:  proto.Float64(in.LocationMessage.Latitude),
		DegreesLongitude: proto.Float64(in.LocationMessage.Longitude),
	}

	if name := strings.TrimSpace(in.LocationMessage.Name); name != "" {
		locationMsg.Name = proto.String(name)
	}

	if address := strings.TrimSpace(in.LocationMessage.Address); address != "" {
		locationMsg.Address = proto.String(address)
	}

	protoMsg := &waProto.Message{
		LocationMessage: locationMsg,
	}

	// Send message
	resp, err := sess.Client.SendMessage(ctx, dest, protoMsg)
	if err != nil {
		return out, fmt.Errorf("failed to send location: %w", err)
	}

	pushName := "Você"
	if sess.Client != nil && sess.Client.Store != nil && sess.Client.Store.PushName != "" {
		pushName = sess.Client.Store.PushName
	}

	out = message.SendTextOutput{
		Key: message.MessageKey{
			RemoteJID: dest.String(),
			FromMe:    true,
			ID:        resp.ID,
		},
		PushName:         pushName,
		Status:           "PENDING",
		MessageType:      "locationMessage",
		MessageTimestamp: resp.Timestamp.Unix(),
		InstanceID:       sess.ID,
		Source:           "unknown",
	}

	return out, nil
}

func (s *messageService) SendContact(ctx context.Context, in message.SendContactInput) (message.SendTextOutput, error) {
	out := message.SendTextOutput{}
	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return out, err
	}

	if strings.TrimSpace(in.Number) == "" {
		return out, errors.New("number is required")
	}
	if len(in.ContactMessage) == 0 {
		return out, errors.New("contactMessage is required and must contain at least one contact")
	}

	dest, err := parseDestinationJID(in.Number)
	if err != nil {
		return out, err
	}

	delay := 0
	if in.Options != nil && in.Options.Delay > 0 {
		delay = in.Options.Delay
	}
	if delay > 0 {
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	// Build vCard for each contact
	vcards := make([]*waProto.ContactMessage, 0, len(in.ContactMessage))
	for _, contact := range in.ContactMessage {
		vcard := buildVCard(contact)
		displayName := strings.TrimSpace(contact.FullName)
		if displayName == "" {
			displayName = strings.TrimSpace(contact.PhoneNumber)
		}

		vcards = append(vcards, &waProto.ContactMessage{
			DisplayName: proto.String(displayName),
			Vcard:       proto.String(vcard),
		})
	}

	var protoMsg *waProto.Message
	if len(vcards) == 1 {
		protoMsg = &waProto.Message{
			ContactMessage: vcards[0],
		}
	} else {
		protoMsg = &waProto.Message{
			ContactsArrayMessage: &waProto.ContactsArrayMessage{
				DisplayName: proto.String(fmt.Sprintf("%d contacts", len(vcards))),
				Contacts:    vcards,
			},
		}
	}

	resp, err := sess.Client.SendMessage(ctx, dest, protoMsg)
	if err != nil {
		return out, fmt.Errorf("failed to send contact: %w", err)
	}

	out.Key = message.MessageKey{
		RemoteJID: resp.ID,
		FromMe:    true,
		ID:        resp.ID,
	}
	out.Status = "PENDING"
	out.MessageTimestamp = resp.Timestamp.Unix()
	out.InstanceID = in.InstanceID

	return out, nil
}

func buildVCard(contact message.ContactEntry) string {
	var builder strings.Builder
	builder.WriteString("BEGIN:VCARD\n")
	builder.WriteString("VERSION:3.0\n")

	fullName := strings.TrimSpace(contact.FullName)
	if fullName != "" {
		builder.WriteString(fmt.Sprintf("FN:%s\n", fullName))
		builder.WriteString(fmt.Sprintf("N:%s\n", fullName))
	}

	if wuid := strings.TrimSpace(contact.WUID); wuid != "" {
		builder.WriteString(fmt.Sprintf("item1.X-ABLabel:WhatsApp\n"))
		builder.WriteString(fmt.Sprintf("item1.IMPP:x-apple:wuid=%s\n", wuid))
	}

	if phone := strings.TrimSpace(contact.PhoneNumber); phone != "" {
		// Ensure phone starts with +
		if !strings.HasPrefix(phone, "+") {
			phone = "+" + phone
		}
		builder.WriteString(fmt.Sprintf("TEL;type=CELL;waid=%s:+%s\n",
			strings.TrimPrefix(phone, "+"),
			strings.TrimPrefix(phone, "+")))
	}

	if org := strings.TrimSpace(contact.Organization); org != "" {
		builder.WriteString(fmt.Sprintf("ORG:%s;\n", org))
	}

	if email := strings.TrimSpace(contact.Email); email != "" {
		builder.WriteString(fmt.Sprintf("EMAIL:%s\n", email))
	}

	if url := strings.TrimSpace(contact.URL); url != "" {
		builder.WriteString(fmt.Sprintf("URL:%s\n", url))
	}

	builder.WriteString("END:VCARD")
	return builder.String()
}

func (s *messageService) SendReaction(ctx context.Context, in message.SendReactionInput) (message.SendTextOutput, error) {
	out := message.SendTextOutput{}
	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return out, err
	}

	// Validate message key
	if strings.TrimSpace(in.ReactionMessage.Key.RemoteJID) == "" {
		return out, errors.New("remoteJid is required in reactionMessage.key")
	}
	if strings.TrimSpace(in.ReactionMessage.Key.ID) == "" {
		return out, errors.New("id is required in reactionMessage.key")
	}

	// Parse destination JID
	destJID, err := parseDestinationJID(in.ReactionMessage.Key.RemoteJID)
	if err != nil {
		return out, fmt.Errorf("invalid remoteJid: %w", err)
	}

	// Build reaction message
	// Empty string removes the reaction, otherwise adds/updates it
	reaction := strings.TrimSpace(in.ReactionMessage.Reaction)

	reactionMsg := &waProto.ReactionMessage{
		Key: &waProto.MessageKey{
			RemoteJID: proto.String(destJID.String()),
			FromMe:    proto.Bool(in.ReactionMessage.Key.FromMe),
			ID:        proto.String(in.ReactionMessage.Key.ID),
		},
		Text:              proto.String(reaction),
		SenderTimestampMS: proto.Int64(time.Now().UnixMilli()),
	}

	protoMsg := &waProto.Message{
		ReactionMessage: reactionMsg,
	}

	// Send reaction
	resp, err := sess.Client.SendMessage(ctx, destJID, protoMsg)
	if err != nil {
		return out, fmt.Errorf("failed to send reaction: %w", err)
	}

	pushName := "Você"
	if sess.Client != nil && sess.Client.Store != nil && sess.Client.Store.PushName != "" {
		pushName = sess.Client.Store.PushName
	}

	out = message.SendTextOutput{
		Key: message.MessageKey{
			RemoteJID: destJID.String(),
			FromMe:    true,
			ID:        resp.ID,
		},
		PushName:         pushName,
		Status:           "PENDING",
		MessageType:      "reactionMessage",
		MessageTimestamp: resp.Timestamp.Unix(),
		InstanceID:       sess.ID,
		Source:           "unknown",
	}

	return out, nil
}

func (s *messageService) SendPoll(ctx context.Context, in message.SendPollInput) (message.SendTextOutput, error) {
	out := message.SendTextOutput{}
	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return out, err
	}

	if strings.TrimSpace(in.Number) == "" {
		return out, errors.New("number is required")
	}
	if strings.TrimSpace(in.PollMessage.Name) == "" {
		return out, errors.New("poll name is required")
	}
	if len(in.PollMessage.Values) == 0 {
		return out, errors.New("poll must have at least one option")
	}
	if in.PollMessage.SelectableCount <= 0 {
		return out, errors.New("selectableCount must be greater than 0")
	}
	if in.PollMessage.SelectableCount > len(in.PollMessage.Values) {
		return out, errors.New("selectableCount cannot be greater than number of options")
	}

	dest, err := parseDestinationJID(in.Number)
	if err != nil {
		return out, err
	}

	delay := 0
	if in.Options != nil && in.Options.Delay > 0 {
		delay = in.Options.Delay
	}

	if delay > 0 {
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	// Build poll options
	pollOptions := make([]*waProto.PollCreationMessage_Option, 0, len(in.PollMessage.Values))
	for _, value := range in.PollMessage.Values {
		pollOptions = append(pollOptions, &waProto.PollCreationMessage_Option{
			OptionName: proto.String(value),
		})
	}

	// Build poll creation message
	pollMsg := &waProto.PollCreationMessage{
		Name:                   proto.String(in.PollMessage.Name),
		Options:                pollOptions,
		SelectableOptionsCount: proto.Uint32(uint32(in.PollMessage.SelectableCount)),
	}

	protoMsg := &waProto.Message{
		PollCreationMessage: pollMsg,
	}

	// Send message
	resp, err := sess.Client.SendMessage(ctx, dest, protoMsg)
	if err != nil {
		return out, fmt.Errorf("failed to send poll: %w", err)
	}

	pushName := "Você"
	if sess.Client != nil && sess.Client.Store != nil && sess.Client.Store.PushName != "" {
		pushName = sess.Client.Store.PushName
	}

	out = message.SendTextOutput{
		Key: message.MessageKey{
			RemoteJID: dest.String(),
			FromMe:    true,
			ID:        resp.ID,
		},
		PushName:         pushName,
		Status:           "PENDING",
		MessageType:      "pollCreationMessage",
		MessageTimestamp: resp.Timestamp.Unix(),
		InstanceID:       sess.ID,
		Source:           "unknown",
	}

	return out, nil
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

// URL detection and extraction helpers
var urlRegex = regexp.MustCompile(`https?://[^\s]+`)

func containsURL(text string) bool {
	return urlRegex.MatchString(text)
}

func extractFirstURL(text string) string {
	matches := urlRegex.FindStringSubmatch(text)
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

// LinkPreviewData holds the preview metadata
type LinkPreviewData struct {
	CanonicalURL        string
	Title               *string
	Description         *string
	JPEGThumbnail       []byte
	ThumbnailDirectPath *string
	ThumbnailSHA256     []byte
	ThumbnailEncSHA256  []byte
	MediaKey            []byte
	MediaKeyTimestamp   *int64
}
