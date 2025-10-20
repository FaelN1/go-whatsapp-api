package eventlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	waLog "go.mau.fi/whatsmeow/util/log"
)

var invalidSegment = regexp.MustCompile(`[^A-Za-z0-9_.-]`)

// Writer persiste eventos brutos vindos do WhatsApp em disco.
type Writer struct {
	baseDir string
	log     waLog.Logger
}

// NewWriter cria uma instância pronta para gravar eventos no diretório informado.
func NewWriter(baseDir string, log waLog.Logger) *Writer {
	base := strings.TrimSpace(baseDir)
	if base == "" {
		return nil
	}
	return &Writer{baseDir: filepath.Clean(base), log: log}
}

// Enabled informa se a gravação de eventos está ativa.
func (w *Writer) Enabled() bool {
	return w != nil && w.baseDir != ""
}

// Write armazena o evento recebido em um arquivo JSON dentro da hierarquia
// baseDir/<tipo>/<instancia>/timestamp-uuid.json.
func (w *Writer) Write(instance string, evt any) error {
	if !w.Enabled() || evt == nil {
		return nil
	}

	eventType := detectEventType(evt)
	segmentType := sanitizeSegment(eventType)
	segmentInstance := sanitizeSegment(instance)

	dir := filepath.Join(w.baseDir, segmentType, segmentInstance)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	ts := time.Now().UTC()
	fileName := fmt.Sprintf("%s-%s.json", ts.Format("20060102T150405Z"), uuid.NewString())
	path := filepath.Join(dir, fileName)

	record := map[string]any{
		"event_type":  eventType,
		"instance":    instance,
		"received_at": ts.Format(time.RFC3339Nano),
		"payload":     marshalPayload(evt),
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		fallback := map[string]any{
			"event_type":    eventType,
			"instance":      instance,
			"received_at":   ts.Format(time.RFC3339Nano),
			"marshal_error": err.Error(),
		}
		if raw := fmt.Sprintf("%+v", evt); raw != "" {
			fallback["payload_text"] = raw
		}
		data, err = json.MarshalIndent(fallback, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal fallback: %w", err)
		}
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func detectEventType(evt any) string {
	if evt == nil {
		return "Unknown"
	}
	t := fmt.Sprintf("%T", evt)
	if t == "" {
		return "Unknown"
	}
	if idx := strings.LastIndex(t, "."); idx >= 0 && idx < len(t)-1 {
		return t[idx+1:]
	}
	return t
}

func sanitizeSegment(raw string) string {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		return "unknown"
	}
	sanitized := invalidSegment.ReplaceAllString(candidate, "_")
	sanitized = strings.Trim(sanitized, "._-")
	if sanitized == "" {
		return "unknown"
	}
	return sanitized
}

func marshalPayload(evt any) any {
	if evt == nil {
		return nil
	}

	raw, err := json.Marshal(evt)
	if err != nil {
		return map[string]any{
			"marshal_error": err.Error(),
			"payload_text":  fmt.Sprintf("%+v", evt),
		}
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return map[string]any{
			"unmarshal_error": err.Error(),
			"payload_text":    fmt.Sprintf("%+v", evt),
		}
	}
	return payload
}
