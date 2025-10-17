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

type SendAudioInput struct {
	InstanceID       string         `json:"instanceId"`
	To               string         `json:"to,omitempty"`
	Number           string         `json:"number,omitempty"`
	Audio            string         `json:"audio"`
	MimeType         string         `json:"mimetype,omitempty"`
	PTT              bool           `json:"ptt,omitempty"`
	Delay            int            `json:"delay,omitempty"`
	LinkPreview      bool           `json:"linkPreview,omitempty"`
	MentionsEveryOne bool           `json:"mentionsEveryOne,omitempty"`
	Mentioned        []string       `json:"mentioned,omitempty"`
	Quoted           *QuotedMessage `json:"quoted,omitempty"`
}

type SendStickerInput struct {
	InstanceID       string         `json:"instanceId"`
	To               string         `json:"to,omitempty"`
	Number           string         `json:"number,omitempty"`
	Sticker          string         `json:"sticker"`
	Delay            int            `json:"delay,omitempty"`
	LinkPreview      bool           `json:"linkPreview,omitempty"`
	MentionsEveryOne bool           `json:"mentionsEveryOne,omitempty"`
	Mentioned        []string       `json:"mentioned,omitempty"`
	Quoted           *QuotedMessage `json:"quoted,omitempty"`
}

type SendLocationInput struct {
	InstanceID       string         `json:"instanceId"`
	To               string         `json:"to,omitempty"`
	Number           string         `json:"number,omitempty"`
	Name             string         `json:"name"`
	Address          string         `json:"address"`
	Latitude         float64        `json:"latitude"`
	Longitude        float64        `json:"longitude"`
	Delay            int            `json:"delay,omitempty"`
	LinkPreview      bool           `json:"linkPreview,omitempty"`
	MentionsEveryOne bool           `json:"mentionsEveryOne,omitempty"`
	Mentioned        []string       `json:"mentioned,omitempty"`
	Quoted           *QuotedMessage `json:"quoted,omitempty"`
}

type ContactEntry struct {
	FullName     string `json:"fullName"`
	WUID         string `json:"wuid,omitempty"`
	PhoneNumber  string `json:"phoneNumber,omitempty"`
	Organization string `json:"organization,omitempty"`
	Email        string `json:"email,omitempty"`
	URL          string `json:"url,omitempty"`
}

type SendContactInput struct {
	InstanceID string         `json:"instanceId"`
	To         string         `json:"to,omitempty"`
	Number     string         `json:"number,omitempty"`
	Contact    []ContactEntry `json:"contact"`
	Delay      int            `json:"delay,omitempty"`
}

type SendReactionInput struct {
	InstanceID string     `json:"instanceId"`
	Key        MessageKey `json:"key"`
	Reaction   string     `json:"reaction"`
	Delay      int        `json:"delay,omitempty"`
}

type SendPollInput struct {
	InstanceID       string         `json:"instanceId"`
	To               string         `json:"to,omitempty"`
	Number           string         `json:"number,omitempty"`
	Name             string         `json:"name"`
	SelectableCount  int            `json:"selectableCount"`
	Values           []string       `json:"values"`
	Delay            int            `json:"delay,omitempty"`
	LinkPreview      bool           `json:"linkPreview,omitempty"`
	MentionsEveryOne bool           `json:"mentionsEveryOne,omitempty"`
	Mentioned        []string       `json:"mentioned,omitempty"`
	Quoted           *QuotedMessage `json:"quoted,omitempty"`
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

type SendStatusInput struct {
	InstanceID      string   `json:"instanceId"`
	Type            string   `json:"type"` // text, image, audio
	Content         string   `json:"content"`
	Caption         string   `json:"caption,omitempty"`
	BackgroundColor string   `json:"backgroundColor,omitempty"`
	Font            int      `json:"font,omitempty"` // 1-5
	AllContacts     bool     `json:"allContacts"`
	StatusJidList   []string `json:"statusJidList,omitempty"`
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
