package storage

import (
	"context"
	"io"
)

type UploadInput struct {
	Key         string
	ContentType string
	Body        io.Reader
	Size        int64
}

type Service interface {
	PutObject(ctx context.Context, in UploadInput) (string, error)
	DeleteObject(ctx context.Context, key string) error
}
