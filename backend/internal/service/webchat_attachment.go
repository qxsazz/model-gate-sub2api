package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	_ "golang.org/x/image/webp"
)

// WebchatUpload contains only inline bytes. The parser never resolves external paths or URLs.
type WebchatUpload struct {
	Name     string `json:"name"`
	MIMEType string `json:"mime_type"`
	Data     string `json:"data"`
}

var (
	ErrWebchatInvalidAttachment = errors.New("invalid or unsupported chat attachment")
	ErrWebchatAttachmentBusy    = errors.New("chat attachment processor unavailable")
	webchatParserSlots          = make(chan struct{}, 2)
)

func ParseWebchatAttachments(ctx context.Context, uploads []WebchatUpload) ([]WebchatAttachment, error) {
	if len(uploads) == 0 {
		return nil, nil
	}
	if len(uploads) > WebchatMaxAttachments {
		return nil, ErrWebchatInvalidAttachment
	}
	select {
	case webchatParserSlots <- struct{}{}:
		defer func() { <-webchatParserSlots }()
	default:
		return nil, ErrWebchatAttachmentBusy
	}
	remaining := WebchatMaxTurnAttachmentBytes
	result := make([]WebchatAttachment, 0, len(uploads))
	for _, upload := range uploads {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !validWebchatFilename(upload.Name) || upload.Data == "" || len(upload.Data) > base64.StdEncoding.EncodedLen(remaining) || strings.ContainsAny(upload.Data, "\n\r") {
			return nil, ErrWebchatInvalidAttachment
		}
		ext := strings.ToLower(path.Ext(upload.Name))
		switch ext {
		case ".png", ".jpg", ".jpeg", ".webp":
		default:
			return nil, ErrWebchatInvalidAttachment
		}
		data, err := base64.StdEncoding.Strict().DecodeString(upload.Data)
		if err != nil || len(data) == 0 || len(data) > remaining {
			return nil, ErrWebchatInvalidAttachment
		}
		remaining -= len(data)
		detected, err := validateWebchatImage(data, ext)
		if err != nil {
			return nil, err
		}
		// A browser may supply no MIME type or send generic binary data.
		claimed := strings.ToLower(strings.TrimSpace(strings.SplitN(upload.MIMEType, ";", 2)[0]))
		if claimed != "" && claimed != "application/octet-stream" && claimed != detected {
			return nil, ErrWebchatInvalidAttachment
		}
		a := WebchatAttachment{Name: upload.Name, Size: len(data), MIMEType: detected, Data: data}
		result = append(result, a)
	}
	return result, nil
}

func validWebchatFilename(s string) bool {
	if s == "" || len(s) > 160 || !utf8.ValidString(s) || strings.TrimSpace(s) != s || strings.HasPrefix(s, ".") || strings.ContainsAny(s, `/\:`) {
		return false
	}
	return strings.IndexFunc(s, func(r rune) bool { return unicode.IsControl(r) || unicode.In(r, unicode.Cf) }) < 0
}

func validateWebchatImage(data []byte, ext string) (string, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if ext == ".jpg" {
		ext = ".jpeg"
	}
	if err != nil || format != ext[1:] || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 8192 || cfg.Height > 8192 || cfg.Width*cfg.Height > 16_000_000 {
		return "", ErrWebchatInvalidAttachment
	}
	// Inspect dimensions before allocating pixels, then reject truncated/corrupt image data.
	if _, _, err = image.Decode(bytes.NewReader(data)); err != nil {
		return "", ErrWebchatInvalidAttachment
	}
	return "image/" + format, nil
}
