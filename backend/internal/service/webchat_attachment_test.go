package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func webchatUpload(name string, raw []byte) WebchatUpload {
	return WebchatUpload{Name: name, Data: base64.StdEncoding.EncodeToString(raw)}
}

func webchatTestDOCX(t *testing.T, document string) []byte {
	t.Helper()
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	const types = `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`
	for name, data := range map[string]string{"[Content_Types].xml": types, "word/document.xml": document} {
		w, err := z.Create(name)
		require.NoError(t, err)
		_, err = io.WriteString(w, data)
		require.NoError(t, err)
	}
	require.NoError(t, z.Close())
	return buf.Bytes()
}

func TestWebchatAttachmentDocuments(t *testing.T) {
	const xml = `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>你好</w:t><w:tab/><w:t>ModelGate</w:t></w:r></w:p><w:p><w:r><w:t>next</w:t></w:r></w:p></w:body></w:document>`
	doc := webchatTestDOCX(t, xml)
	utf8 := []byte("\ufeff# Markdown\n节点")
	files, err := ParseWebchatAttachments(context.Background(), []WebchatUpload{webchatUpload("文档.docx", doc), webchatUpload("文本.md", utf8)})
	require.NoError(t, err)
	require.Len(t, files, 2)
	require.Equal(t, "你好\tModelGate\nnext", files[0].Text)
	require.Equal(t, webchatDOCXMIME, files[0].MIMEType)
	require.Equal(t, doc, files[0].Data)
	require.Equal(t, len(doc), files[0].Size)
	require.Equal(t, "# Markdown\n节点", files[1].Text)
	require.Equal(t, "text/plain", files[1].MIMEType)
	require.Equal(t, utf8, files[1].Data)
}

func TestWebchatAttachmentImages(t *testing.T) {
	imageData := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var p, j bytes.Buffer
	require.NoError(t, png.Encode(&p, imageData))
	require.NoError(t, jpeg.Encode(&j, imageData, nil))
	const webp = "UklGRjgAAABXRUJQVlA4ICwAAACQAQCdASoBAAEAAgA0JaACdLoAA5gA/vmTb/+QH/+QH/+QH/8gP+IXeyAwAA=="
	files, err := ParseWebchatAttachments(context.Background(), []WebchatUpload{webchatUpload("a.PNG", p.Bytes()), webchatUpload("b.jpeg", j.Bytes()), {Name: "c.webp", MIMEType: "image/webp", Data: webp}})
	require.NoError(t, err)
	require.Equal(t, []string{"image/png", "image/jpeg", "image/webp"}, []string{files[0].MIMEType, files[1].MIMEType, files[2].MIMEType})
	require.Empty(t, files[2].Text)
	for _, f := range []WebchatUpload{webchatUpload("wrong.jpg", p.Bytes()), webchatUpload("truncated.png", p.Bytes()[:len(p.Bytes())/2]), {Name: "wrong.webp", MIMEType: "image/jpeg", Data: webp}} {
		_, err := ParseWebchatAttachments(context.Background(), []WebchatUpload{f})
		require.ErrorIs(t, err, ErrWebchatInvalidAttachment)
	}
}

func TestWebchatAttachmentInvalidInput(t *testing.T) {
	const doc = `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:t>%s</w:t></w:document>`
	largeDoc := webchatTestDOCX(t, fmt.Sprintf(doc, strings.Repeat("a", WebchatMaxExtractedFileBytes+1)))
	oversizedXML := webchatTestDOCX(t, fmt.Sprintf(doc, strings.Repeat("a", 2<<20)))
	big := strings.Repeat("A", base64.StdEncoding.EncodedLen(WebchatMaxTurnAttachmentBytes)+4)
	for name, uploads := range map[string][]WebchatUpload{
		"missing bytes": {{Name: "blank.txt"}}, "invalid base64": {{Name: "x.pdf", Data: "?"}},
		"URL": {{Name: "x.png", Data: "https://test.invalid/x.png"}}, "invalid path": {webchatUpload("../test.txt", []byte("hello"))},
		"binary as txt": {webchatUpload("data.txt", []byte{'a', 0, 'z'})}, "oversize text": {webchatUpload("x.log", bytes.Repeat([]byte{'x'}, WebchatMaxExtractedFileBytes+1))},
		"SVG": {webchatUpload("pic.svg", []byte("<svg/>"))}, "plain text pretending to be a PDF": {webchatUpload("x.pdf", []byte("text"))},
		"ZIP pretending to be Word": {webchatUpload("x.docx", []byte("PK"))}, "excess Word text": {webchatUpload("x.docx", largeDoc)},
		"compressed Word expansion": {webchatUpload("x.docx", oversizedXML)}, "input larger than four MiB": {{Name: "x.png", Data: big}},
		"too many items": make([]WebchatUpload, WebchatMaxAttachments+1),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseWebchatAttachments(context.Background(), uploads)
			require.ErrorIs(t, err, ErrWebchatInvalidAttachment)
		})
	}
}

func TestWebchatPDFExtractionOutputLimit(t *testing.T) {
	var output webchatPDFOutput
	// Avoid a source-side WriterTo, so io.Copy also exercises its ReaderFrom fast path.
	reader := struct{ io.Reader }{strings.NewReader(strings.Repeat("x", WebchatMaxExtractedFileBytes+1))}
	_, err := io.Copy(&output, reader)
	require.ErrorIs(t, err, ErrWebchatInvalidAttachment)
	require.LessOrEqual(t, output.buf.Len(), WebchatMaxExtractedFileBytes)
}

func TestWebchatPDFExtraction(t *testing.T) {
	for _, tool := range []string{"nsjail", "pdftotext"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is provided by the runtime image", tool)
		}
	}
	var pdf bytes.Buffer
	_, _ = pdf.WriteString("%PDF-1.4\n")
	stream := "BT /F1 18 Tf 40 100 Td (ModelGate document) Tj ET"
	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R] /Count 1 >>", "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 200] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>", "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>", fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream)}
	var offsets []int
	for i, obj := range objects {
		offsets = append(offsets, pdf.Len())
		_, _ = fmt.Fprintf(&pdf, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := pdf.Len()
	_, _ = fmt.Fprintf(&pdf, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		_, _ = fmt.Fprintf(&pdf, "%010d 00000 n \n", offset)
	}
	_, _ = fmt.Fprintf(&pdf, "trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xref)
	files, err := ParseWebchatAttachments(context.Background(), []WebchatUpload{webchatUpload("doc.pdf", pdf.Bytes())})
	require.NoError(t, err)
	require.Equal(t, "ModelGate document", files[0].Text)
	require.Equal(t, pdf.Bytes(), files[0].Data)
}
