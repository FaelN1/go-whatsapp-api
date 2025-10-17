package minio

import (
	"context"
	"fmt"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/faeln1/go-whatsapp-api/pkg/storage"
)

type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	Region    string
	UseSSL    bool
	PublicURL string
}

type Client struct {
	core      *minio.Client
	bucket    string
	publicURL string
}

func New(ctx context.Context, cfg Config) (*Client, error) {
	core, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, err
	}

	if err := ensureBucket(ctx, core, cfg.Bucket, cfg.Region); err != nil {
		return nil, err
	}

	return &Client{core: core, bucket: cfg.Bucket, publicURL: cfg.PublicURL}, nil
}

func ensureBucket(ctx context.Context, client *minio.Client, bucket, region string) error {
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	opts := minio.MakeBucketOptions{Region: region}
	return client.MakeBucket(ctx, bucket, opts)
}

func (c *Client) PutObject(ctx context.Context, in storage.UploadInput) (string, error) {
	_, err := c.core.PutObject(ctx, c.bucket, in.Key, in.Body, in.Size, minio.PutObjectOptions{ContentType: in.ContentType})
	if err != nil {
		return "", err
	}
	return c.objectURL(in.Key), nil
}

func (c *Client) DeleteObject(ctx context.Context, key string) error {
	return c.core.RemoveObject(ctx, c.bucket, key, minio.RemoveObjectOptions{})
}

func (c *Client) objectURL(key string) string {
	key = strings.TrimLeft(key, "/")
	if c.publicURL != "" {
		return fmt.Sprintf("%s/%s", strings.TrimRight(c.publicURL, "/"), key)
	}

	endpoint := c.core.EndpointURL()
	if endpoint != nil {
		return fmt.Sprintf("%s/%s/%s", strings.TrimRight(endpoint.String(), "/"), c.bucket, key)
	}

	return fmt.Sprintf("/%s/%s", c.bucket, key)
}

var _ storage.Service = (*Client)(nil)
