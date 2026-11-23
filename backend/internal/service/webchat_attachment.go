package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os/exec"
	"path"
	"strings"
	"time"
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

const webchatDOCXMIME = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

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
		data, err := base64.StdEncoding.Strict().DecodeString(upload.Data)
		if err != nil || len(data) == 0 || len(data) > remaining {
			return nil, ErrWebchatInvalidAttachment
		}
		remaining -= len(data)
		a := WebchatAttachment{Name: upload.Name, Size: len(data), Data: data}
		ext := strings.ToLower(path.Ext(a.Name))
		switch ext {
		case ".png", ".jpg", ".jpeg", ".webp":
			a.MIMEType, err = validateWebchatImage(data, ext)
		case ".docx":
			a.MIMEType = webchatDOCXMIME
			a.Text, err = extractWebchatDOCX(data)
		case ".pdf":
			a.MIMEType = "application/pdf"
			a.Text, err = extractWebchatPDF(ctx, data)
		case ".txt", ".md", ".markdown", ".csv", ".tsv", ".json", ".log", ".yaml", ".yml", ".xml", ".html", ".css", ".js", ".ts", ".py", ".go", ".sh":
			a.MIMEType = "text/plain"
			a.Text, err = webchatDocumentText(data)
		default:
			return nil, ErrWebchatInvalidAttachment
		}
		if err != nil {
			return nil, err
		}
		// Browsers may omit a type, or return vendor-specific types for text files.
		// An explicit image type must agree with the detected bytes and extension.
		claimed := strings.ToLower(strings.TrimSpace(strings.SplitN(upload.MIMEType, ";", 2)[0]))
		if strings.HasPrefix(claimed, "image/") && claimed != a.MIMEType {
			return nil, ErrWebchatInvalidAttachment
		}
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

func webchatDocumentText(data []byte) (string, error) {
	if len(data) > WebchatMaxExtractedFileBytes || !utf8.Valid(data) {
		return "", ErrWebchatInvalidAttachment
	}
	s := strings.TrimSpace(strings.TrimPrefix(string(data), "\ufeff"))
	if s == "" || strings.IndexFunc(s, func(r rune) bool {
		return unicode.IsControl(r) && r != '\t' && r != '\r' && r != '\n'
	}) >= 0 {
		return "", ErrWebchatInvalidAttachment
	}
	return s, nil
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

func extractWebchatDOCX(data []byte) (string, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || len(archive.File) > 512 {
		return "", ErrWebchatInvalidAttachment
	}
	var contentTypes, doc *zip.File
	var expanded uint64
	for _, f := range archive.File {
		if f.UncompressedSize64 > 32<<20 || f.Flags&1 != 0 {
			return "", ErrWebchatInvalidAttachment
		}
		expanded += f.UncompressedSize64
		if expanded > 32<<20 {
			return "", ErrWebchatInvalidAttachment
		}
		switch f.Name {
		case "[Content_Types].xml":
			if contentTypes != nil {
				return "", ErrWebchatInvalidAttachment
			}
			contentTypes = f
		case "word/document.xml":
			if doc != nil {
				return "", ErrWebchatInvalidAttachment
			}
			doc = f
		}
	}
	content, err := readWebchatZIPPart(contentTypes, 128<<10)
	if err != nil {
		return "", err
	}
	var manifest struct {
		XMLName xml.Name `xml:"http://schemas.openxmlformats.org/package/2006/content-types Types"`
		Entries []struct {
			Path string `xml:"PartName,attr"`
			Type string `xml:"ContentType,attr"`
		} `xml:"Override"`
	}
	if xml.Unmarshal(content, &manifest) != nil {
		return "", ErrWebchatInvalidAttachment
	}
	for _, entry := range manifest.Entries {
		if entry.Path == "/word/document.xml" && entry.Type == "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml" {
			body, err := readWebchatZIPPart(doc, 2<<20)
			if err != nil {
				return "", err
			}
			return readWebchatWordText(body)
		}
	}
	return "", ErrWebchatInvalidAttachment
}

func readWebchatZIPPart(f *zip.File, limit int64) ([]byte, error) {
	if f == nil || f.UncompressedSize64 > uint64(limit) {
		return nil, ErrWebchatInvalidAttachment
	}
	r, err := f.Open()
	if err != nil {
		return nil, ErrWebchatInvalidAttachment
	}
	defer func() { _ = r.Close() }()
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, ErrWebchatInvalidAttachment
	}
	return data, nil
}

func readWebchatWordText(data []byte) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var text bytes.Buffer
	var namespace string
	var depth, textDepth int
	for {
		token, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", ErrWebchatInvalidAttachment
		}
		switch v := token.(type) {
		case xml.Directive:
			return "", ErrWebchatInvalidAttachment
		case xml.StartElement:
			depth++
			if depth == 1 {
				if namespace != "" || v.Name.Local != "document" || (v.Name.Space != "http://schemas.openxmlformats.org/wordprocessingml/2006/main" && v.Name.Space != "http://purl.oclc.org/ooxml/wordprocessingml/main") {
					return "", ErrWebchatInvalidAttachment
				}
				namespace = v.Name.Space
			}
			if depth > 128 {
				return "", ErrWebchatInvalidAttachment
			}
			if v.Name.Space == namespace {
				switch v.Name.Local {
				case "t":
					textDepth = depth
				case "br", "cr":
					_ = text.WriteByte('\n')
				case "tab":
					_ = text.WriteByte('\t')
				}
			}
		case xml.CharData:
			if textDepth == depth && depth > 0 {
				_, _ = text.Write(v)
			}
		case xml.EndElement:
			if textDepth == depth {
				textDepth = 0
			}
			if v.Name.Space == namespace && v.Name.Local == "p" {
				_ = text.WriteByte('\n')
			}
			depth--
		}
		if text.Len() > WebchatMaxExtractedFileBytes {
			return "", ErrWebchatInvalidAttachment
		}
	}
	return webchatDocumentText(text.Bytes())
}

type webchatPDFOutput struct{ buf bytes.Buffer }

func (w *webchatPDFOutput) Write(p []byte) (int, error) {
	if len(p) > WebchatMaxExtractedFileBytes-w.buf.Len() {
		return 0, ErrWebchatInvalidAttachment
	}
	return w.buf.Write(p)
}

func extractWebchatPDF(ctx context.Context, data []byte) (string, error) {
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return "", ErrWebchatInvalidAttachment
	}
	jail, err := exec.LookPath("nsjail")
	if err != nil {
		return "", ErrWebchatAttachmentBusy
	}
	pdf, err := exec.LookPath("pdftotext")
	if err != nil {
		return "", ErrWebchatAttachmentBusy
	}
	deadline, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// Docker already supplies namespaces. A nested seccomp filter blocks networking
	// without elevated privileges; nsjail enforces the child's CPU, memory and fd limits.
	cmd := exec.CommandContext(deadline, jail, "-Mo", "--quiet", "--disable_clone_newnet", "--disable_clone_newuser", "--disable_clone_newns", "--disable_clone_newpid", "--disable_clone_newipc", "--disable_clone_newuts", "--disable_clone_newcgroup", "--disable_proc", "--rlimit_as", "160", "--rlimit_cpu", "3", "--rlimit_nofile", "32", "--rlimit_fsize", "0", "--time_limit", "4", "--seccomp_string", "POLICY pdf { ERRNO(1) { socket, socketpair, connect } } USE pdf DEFAULT ALLOW", "--", pdf, "-enc", "UTF-8", "-nopgbrk", "-", "-")
	cmd.Env = []string{"LANG=C", "HOME=/nonexistent"}
	cmd.Dir = "/"
	cmd.Stdin = bytes.NewReader(data)
	var output webchatPDFOutput
	cmd.Stdout = &output
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", ErrWebchatInvalidAttachment
	}
	return webchatDocumentText(output.buf.Bytes())
}
