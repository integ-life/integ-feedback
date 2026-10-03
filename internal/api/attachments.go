package api

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"errors"
	"image/jpeg"
)

//go:embed assets/feedback.js
var feedbackJS []byte

func normalizeAttachment(encoded string, consent bool) ([]byte, error) {
	if encoded == "" {
		return nil, nil
	}
	if !consent {
		return nil, errors.New("Explicit attachment consent is required")
	}
	if len(encoded) > base64.StdEncoding.EncodedLen(512<<10) {
		return nil, errors.New("Image exceeds 512 KiB")
	}
	data, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(data) == 0 || len(data) > 512<<10 {
		return nil, errors.New("Invalid image encoding")
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 1280 || config.Height > 1280 {
		return nil, errors.New("A JPEG image with dimensions up to 1280 pixels is required")
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("Could not decode image")
	}
	// Decode and re-encode even browser-prepared images to remove EXIF and
	// appended payloads at the trust boundary. Only JPEG pixels are persisted.
	var out bytes.Buffer
	if err = jpeg.Encode(&out, img, &jpeg.Options{Quality: 85}); err != nil || out.Len() > 512<<10 {
		return nil, errors.New("Image cannot be stored within the 512 KiB limit")
	}
	return out.Bytes(), nil
}
