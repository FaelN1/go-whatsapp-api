package whatsapp

import "errors"

var (
	ErrAlreadyExists     = errors.New("whatsapp session already exists")
	ErrNotFound          = errors.New("whatsapp session not found")
	ErrClientUnavailable = errors.New("whatsapp client not available")
)
