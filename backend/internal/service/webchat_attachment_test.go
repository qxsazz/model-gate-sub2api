package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func webchatUpload(name string, raw []byte) WebchatUpload {
	return WebchatUpload{Name: name, Data: base64.StdEncoding.EncodeToString(raw)}
}

func TestWebchatAttachmentImages(t *testing.T) {
	imageData := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var p, j bytes.Buffer
	require.NoError(t, png.Encode(&p, imageData))
	require.NoError(t, jpeg.Encode(&j, imageData, nil))
	const webp = "UklGRjgAAABXRUJQVlA4ICwAAACQAQCdASoBAAEAAgA0JaACdLoAA5gA/vmTb/+QH/+QH/+QH/8gP+IXeyAwAA=="
	pngUpload := webchatUpload("a.PNG", p.Bytes())
	pngUpload.MIMEType = "application/octet-stream"
	files, err := ParseWebchatAttachments(context.Background(), []WebchatUpload{pngUpload, webchatUpload("b.jpeg", j.Bytes()), {Name: "c.webp", MIMEType: "image/webp", Data: webp}})
	require.NoError(t, err)
	require.Equal(t, []string{"image/png", "image/jpeg", "image/webp"}, []string{files[0].MIMEType, files[1].MIMEType, files[2].MIMEType})
	require.Empty(t, files[2].Text)
	for _, f := range []WebchatUpload{webchatUpload("wrong.jpg", p.Bytes()), webchatUpload("truncated.png", p.Bytes()[:len(p.Bytes())/2]), {Name: "wrong.webp", MIMEType: "image/jpeg", Data: webp}} {
		_, err := ParseWebchatAttachments(context.Background(), []WebchatUpload{f})
		require.ErrorIs(t, err, ErrWebchatInvalidAttachment)
	}
}

func TestWebchatAttachmentInvalidInput(t *testing.T) {
	big := strings.Repeat("A", base64.StdEncoding.EncodedLen(WebchatMaxTurnAttachmentBytes)+4)
	const webp = "UklGRjgAAABXRUJQVlA4ICwAAACQAQCdASoBAAEAAgA0JaACdLoAA5gA/vmTb/+QH/+QH/+QH/8gP+IXeyAwAA=="
	for name, uploads := range map[string][]WebchatUpload{
		"missing image":       {{Name: "blank.png"}},
		"invalid base64":      {{Name: "x.png", Data: "?"}},
		"external URL":        {{Name: "x.png", Data: "https://test.invalid/x.png"}},
		"invalid path":        {{Name: "../image.webp", Data: webp}},
		"PDF":                 {webchatUpload("doc.pdf", []byte("%PDF-1.4"))},
		"Word document":       {webchatUpload("doc.docx", []byte("PK"))},
		"plain text":          {webchatUpload("file.txt", []byte("hello"))},
		"Markdown":            {webchatUpload("notes.md", []byte("#Title"))},
		"code":                {webchatUpload("data.json", []byte("{}"))},
		"SVG":                 {webchatUpload("pic.svg", []byte("<svg/>"))},
		"non-image extension": {{Name: "doc.pdf", MIMEType: "image/webp", Data: webp}},
		"non-image MIME":      {{Name: "pic.webp", MIMEType: "application/pdf", Data: webp}},
		"non-image bytes":     {webchatUpload("pic.jpeg", []byte("%PDF-1.4"))},
		"oversize":            {{Name: "x.png", Data: big}},
		"too many items":      make([]WebchatUpload, WebchatMaxAttachments+1),
	} {
		t.Run(name, func(t *testing.T) {
			files, err := ParseWebchatAttachments(context.Background(), uploads)
			require.ErrorIs(t, err, ErrWebchatInvalidAttachment)
			require.Nil(t, files)
		})
	}
}
