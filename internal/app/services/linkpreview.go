package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/faeln1/go-whatsapp-api/internal/platform/whatsapp"
	"google.golang.org/protobuf/proto"
)

// SimpleLinkPreview holds basic preview data without peer operation
type SimpleLinkPreview struct {
	URL         string
	Title       string
	Description string
	ImageURL    string
	ImageData   []byte
}

// fetchSimpleLinkPreview fetches basic link preview data from HTML meta tags
func fetchSimpleLinkPreview(ctx context.Context, targetURL string) (*SimpleLinkPreview, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "WhatsApp/2.23.20")

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}

	// Read up to 512KB of HTML
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, err
	}

	body := string(bodyBytes)
	preview := &SimpleLinkPreview{URL: targetURL}

	// Parse Open Graph / Twitter Card meta tags
	preview.Title = extractMetaTag(body, "og:title", "twitter:title")
	preview.Description = extractMetaTag(body, "og:description", "twitter:description", "description")
	preview.ImageURL = extractMetaTag(body, "og:image", "twitter:image")

	// Fallback to title tag
	if preview.Title == "" {
		if start := strings.Index(body, "<title>"); start != -1 {
			if end := strings.Index(body[start:], "</title>"); end != -1 {
				preview.Title = strings.TrimSpace(body[start+7 : start+end])
			}
		}
	}

	// Download image if available
	if preview.ImageURL != "" {
		if imgData, err := downloadImageForPreview(ctx, preview.ImageURL); err == nil {
			preview.ImageData = imgData
		}
	}

	return preview, nil
}

func extractMetaTag(html string, names ...string) string {
	for _, name := range names {
		// Try property="name"
		if val := findMetaContent(html, `property="`+name+`"`); val != "" {
			return val
		}
		// Try name="name"
		if val := findMetaContent(html, `name="`+name+`"`); val != "" {
			return val
		}
	}
	return ""
}

func findMetaContent(html, pattern string) string {
	idx := strings.Index(html, pattern)
	if idx == -1 {
		return ""
	}

	// Find content attribute after pattern
	contentIdx := strings.Index(html[idx:], `content="`)
	if contentIdx == -1 {
		return ""
	}

	start := idx + contentIdx + 9
	if start >= len(html) {
		return ""
	}

	end := strings.Index(html[start:], `"`)
	if end == -1 {
		return ""
	}

	return strings.TrimSpace(html[start : start+end])
}

func downloadImageForPreview(ctx context.Context, imageURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}

	// Limit to 5MB
	data, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return nil, err
	}

	// Basic validation - check if it's actually an image
	if len(data) < 4 {
		return nil, errors.New("invalid image data")
	}

	// Check for common image signatures
	isImage := bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}) || // JPEG
		bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47}) || // PNG
		bytes.HasPrefix(data, []byte{0x47, 0x49, 0x46}) || // GIF
		bytes.HasPrefix(data, []byte{0x52, 0x49, 0x46, 0x46}) // WEBP

	if !isImage {
		return nil, errors.New("not an image")
	}

	return data, nil
}

// Enhanced generateLinkPreview that uses simple HTML parsing as fallback
func (s *messageService) generateLinkPreviewEnhanced(ctx context.Context, sess *whatsapp.Session, targetURL string) (*LinkPreviewData, error) {
	// Try simple HTML-based preview first
	simple, err := fetchSimpleLinkPreview(ctx, targetURL)
	if err != nil {
		return nil, err
	}

	preview := &LinkPreviewData{
		CanonicalURL: targetURL,
	}

	if simple.Title != "" {
		preview.Title = proto.String(simple.Title)
	}

	if simple.Description != "" {
		preview.Description = proto.String(simple.Description)
	}

	// If we have image data, use it as thumbnail
	if len(simple.ImageData) > 0 {
		preview.JPEGThumbnail = simple.ImageData
	}

	return preview, nil
}

func sanitizeFileNamePreview(name string) string {
	if strings.TrimSpace(name) == "" {
		return ""
	}
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' || r == '\x00' {
			return '_'
		}
		return r
	}, name)
	return strings.TrimSpace(name)
}
