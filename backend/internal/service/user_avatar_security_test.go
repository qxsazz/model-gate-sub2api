//go:build unit

package service

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"image"
	"testing"

	"github.com/stretchr/testify/require"
)

// A real PNG header is enough to describe enormous decoder allocations; no
// dangerous image.Decode or large pixel buffer is needed to test the guard.
func avatarPNGHeader(width, height uint32) []byte {
	var png bytes.Buffer
	png.WriteString("\x89PNG\r\n\x1a\n")
	chunk := append([]byte("IHDR"), make([]byte, 13)...)
	binary.BigEndian.PutUint32(chunk[4:8], width)
	binary.BigEndian.PutUint32(chunk[8:12], height)
	chunk[12], chunk[13] = 8, 6 // 8-bit RGBA
	_ = binary.Write(&png, binary.BigEndian, uint32(13))
	png.Write(chunk)
	_ = binary.Write(&png, binary.BigEndian, crc32.ChecksumIEEE(chunk))
	return png.Bytes()
}

func TestAvatarRejectsOversizedPixelBuffersBeforeDecode(t *testing.T) {
	for _, dims := range [][2]uint32{{50000, 50000}, {4097, 1}, {4096, 2048}} {
		header := avatarPNGHeader(dims[0], dims[1])
		cfg, _, err := image.DecodeConfig(bytes.NewReader(header))
		require.NoError(t, err)
		require.Equal(t, int(dims[0]), cfg.Width)
		// Padding places the tiny header on the actual avatar compression path.
		payload := append(header, make([]byte, targetAvatarBytes)...)
		require.Less(t, len(payload), maxInlineAvatarBytes)
		_, err = normalizeInlineUserAvatarInput("data:image/png;base64," + base64.StdEncoding.EncodeToString(payload))
		require.ErrorIs(t, err, ErrAvatarDimensionsTooLarge)
	}
}

func TestAvatarSmallInvalidImageStillReturnsValidationError(t *testing.T) {
	_, _, err := compressInlineAvatar([]byte("invalid image"))
	require.ErrorIs(t, err, ErrAvatarInvalid)
}
