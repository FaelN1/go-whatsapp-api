package message

import "mime/multipart"

type QuotedMessage struct {
	Key     *MessageKey  `json:"key,omitempty"`
	Message *MessageBody `json:"message,omitempty"`
}

type SendTextInput struct {
	InstanceID       string         `json:"instanceId"`
	To               string         `json:"to,omitempty"`
	Number           string         `json:"number,omitempty"` // Compat Evolution API
	Text             string         `json:"text"`
	Delay            int            `json:"delay,omitempty"`
	LinkPreview      bool           `json:"linkPreview,omitempty"`
	MentionsEveryOne bool           `json:"mentionsEveryOne,omitempty"`
	Mentioned        []string       `json:"mentioned,omitempty"`
	Quoted           *QuotedMessage `json:"quoted,omitempty"`
}

type SendMediaInput struct {
	InstanceID       string         `json:"instanceId"`
	To               string         `json:"to,omitempty"`
	Number           string         `json:"number,omitempty"` // Compat Evolution API
	MediaType        string         `json:"mediatype"`        // image, video, document
	MimeType         string         `json:"mimetype,omitempty"`
	Caption          string         `json:"caption,omitempty"`
	Media            string         `json:"media"` // URL ou base64
	FileName         string         `json:"fileName,omitempty"`
	Delay            int            `json:"delay,omitempty"`
	LinkPreview      bool           `json:"linkPreview,omitempty"`
	MentionsEveryOne bool           `json:"mentionsEveryOne,omitempty"`
	Mentioned        []string       `json:"mentioned,omitempty"`
	Quoted           *QuotedMessage `json:"quoted,omitempty"`
	// Legacy multipart
	File       multipart.File        `json:"-"`
	FileHeader *multipart.FileHeader `json:"-"`
}

type AudioMessage struct {
	Audio string `json:"audio"` // URL or base64
}

type AudioOptions struct {
	Delay    int    `json:"delay,omitempty"`
	Presence string `json:"presence,omitempty"` // "recording" or "composing"
	Encoding bool   `json:"encoding,omitempty"` // Convert to PTT format
}

type SendAudioInput struct {
	InstanceID   string        `json:"instanceId"`
	Number       string        `json:"number"`
	AudioMessage AudioMessage  `json:"audioMessage"`
	Options      *AudioOptions `json:"options,omitempty"`
}

type StickerMessage struct {
	Image string `json:"image"` // URL or base64
}

type StickerOptions struct {
	Delay    int    `json:"delay,omitempty"`
	Presence string `json:"presence,omitempty"` // "composing" or "recording"
}

type SendStickerInput struct {
	InstanceID     string          `json:"instanceId"`
	Number         string          `json:"number"`
	StickerMessage StickerMessage  `json:"stickerMessage"`
	Options        *StickerOptions `json:"options,omitempty"`
}

type LocationMessage struct {
	Name      string  `json:"name"`
	Address   string  `json:"address"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type LocationOptions struct {
	Delay    int    `json:"delay,omitempty"`
	Presence string `json:"presence,omitempty"` // "composing" or "recording"
}

type SendLocationInput struct {
	InstanceID      string           `json:"instanceId"`
	Number          string           `json:"number"`
	LocationMessage LocationMessage  `json:"locationMessage"`
	Options         *LocationOptions `json:"options,omitempty"`
}

type ContactEntry struct {
	FullName     string `json:"fullName"`
	WUID         string `json:"wuid,omitempty"`
	PhoneNumber  string `json:"phoneNumber,omitempty"`
	Organization string `json:"organization,omitempty"`
	Email        string `json:"email,omitempty"`
	URL          string `json:"url,omitempty"`
}

type SendContactOptions struct {
	Delay    int    `json:"delay,omitempty"`
	Presence string `json:"presence,omitempty"`
}

type SendContactInput struct {
	InstanceID     string              `json:"instanceId"`
	Number         string              `json:"number"`
	ContactMessage []ContactEntry      `json:"contactMessage"`
	Options        *SendContactOptions `json:"options,omitempty"`
}

type ReactionMessage struct {
	Key      MessageKey `json:"key"`
	Reaction string     `json:"reaction"` // Emoji or empty string to remove reaction
}

type SendReactionInput struct {
	InstanceID      string          `json:"instanceId"`
	ReactionMessage ReactionMessage `json:"reactionMessage"`
}

type PollMessage struct {
	Name            string   `json:"name"`
	SelectableCount int      `json:"selectableCount"`
	Values          []string `json:"values"`
}

type PollOptions struct {
	Delay    int    `json:"delay,omitempty"`
	Presence string `json:"presence,omitempty"`
}

type SendPollInput struct {
	InstanceID  string       `json:"instanceId"`
	Number      string       `json:"number"`
	PollMessage PollMessage  `json:"pollMessage"`
	Options     *PollOptions `json:"options,omitempty"`
}

type ListRow struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	RowID       string `json:"rowId"`
}

type ListSection struct {
	Title string    `json:"title"`
	Rows  []ListRow `json:"rows"`
}

type SendListInput struct {
	InstanceID       string         `json:"instanceId"`
	To               string         `json:"to,omitempty"`
	Number           string         `json:"number,omitempty"`
	Title            string         `json:"title"`
	Description      string         `json:"description"`
	ButtonText       string         `json:"buttonText"`
	FooterText       string         `json:"footerText"`
	Values           []ListSection  `json:"values"`
	Delay            int            `json:"delay,omitempty"`
	LinkPreview      bool           `json:"linkPreview,omitempty"`
	MentionsEveryOne bool           `json:"mentionsEveryOne,omitempty"`
	Mentioned        []string       `json:"mentioned,omitempty"`
	Quoted           *QuotedMessage `json:"quoted,omitempty"`
}

type ButtonOption struct {
	Title       string `json:"title"`
	DisplayText string `json:"displayText"`
	ID          string `json:"id"`
}

type SendButtonInput struct {
	InstanceID       string         `json:"instanceId"`
	To               string         `json:"to,omitempty"`
	Number           string         `json:"number,omitempty"`
	Title            string         `json:"title"`
	Description      string         `json:"description"`
	Footer           string         `json:"footer"`
	Buttons          []ButtonOption `json:"buttons"`
	Delay            int            `json:"delay,omitempty"`
	LinkPreview      bool           `json:"linkPreview,omitempty"`
	MentionsEveryOne bool           `json:"mentionsEveryOne,omitempty"`
	Mentioned        []string       `json:"mentioned,omitempty"`
	Quoted           *QuotedMessage `json:"quoted,omitempty"`
}

type StatusMessage struct {
	Type            string   `json:"type"`                      // text, image, video, audio
	Content         string   `json:"content"`                   // URL or base64 for media, text for text status
	Caption         string   `json:"caption,omitempty"`         // Caption for media status
	BackgroundColor string   `json:"backgroundColor,omitempty"` // Hex color for text status (e.g., "#FF5733")
	Font            int      `json:"font,omitempty"`            // Font number 0-5 for text status
	AllContacts     bool     `json:"allContacts"`               // Send to all contacts
	StatusJidList   []string `json:"statusJidList,omitempty"`   // Specific JIDs to send status to
}

type SendStatusInput struct {
	InstanceID    string        `json:"instanceId"`
	StatusMessage StatusMessage `json:"statusMessage"`
}

type MessageKey struct {
	RemoteJID string `json:"remoteJid"`
	FromMe    bool   `json:"fromMe"`
	ID        string `json:"id"`
}

type MessageBody struct {
	Conversation string `json:"conversation,omitempty"`
}

type SendTextOutput struct {
	Key              MessageKey  `json:"key"`
	PushName         string      `json:"pushName"`
	Status           string      `json:"status"`
	Message          MessageBody `json:"message"`
	MessageType      string      `json:"messageType"`
	MessageTimestamp int64       `json:"messageTimestamp"`
	InstanceID       string      `json:"instanceId"`
	Source           string      `json:"source"`
}
