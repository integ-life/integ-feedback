package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"github.com/integ-life/integ-feedback/internal/auth"
	"image"
	"image/color"
	"image/jpeg"
	"net/http/httptest"
	"strings"
	"testing"
)

func imageBase64(width, height int) string {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.RGBA{R: 200, G: 50, B: 40, A: 255})
	var b bytes.Buffer
	jpeg.Encode(&b, img, &jpeg.Options{Quality: 90})
	// Appended non-image data must not survive normalization.
	b.WriteString("private trailing metadata")
	return base64.StdEncoding.EncodeToString(b.Bytes())
}

func TestAttachmentValidationAndPrivateReceipt(t *testing.T) {
	cases := []struct {
		name, encoded string
		consent       bool
		status        int
	}{
		{"accepted", imageBase64(20, 10), true, 201},
		{"text-only", "", false, 201},
		{"consent-required", imageBase64(20, 10), false, 400},
		{"not-image", base64.StdEncoding.EncodeToString([]byte("<svg>")), true, 400},
		{"malformed", "!!!!", true, 400},
		{"too-wide", imageBase64(1281, 1), true, 400},
		{"too-large", strings.Repeat("A", 700000), true, 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{}
			h := New(repo, auth.New(""), []string{"https://tools.integ.life"}).Handler()
			payload, _ := json.Marshal(map[string]any{"resource": "tool:component-scanner", "kind": "issue", "body": "Could not read bands", "image_base64": tc.encoded, "attachment_consent": tc.consent})
			req := httptest.NewRequest("POST", "/v1/feedback", bytes.NewReader(payload))
			req.Header.Set("X-Project-Key", "pk_test")
			req.Header.Set("Origin", "https://tools.integ.life")
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != tc.status {
				t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
			}
			if tc.name == "accepted" {
				if !strings.Contains(rr.Body.String(), `"has_attachment":true`) || strings.Contains(rr.Body.String(), tc.encoded) {
					t.Fatal("invalid private receipt")
				}
				if bytes.Contains(repo.attachment, []byte("private trailing metadata")) {
					t.Fatal("metadata persisted")
				}
				config, err := jpeg.DecodeConfig(bytes.NewReader(repo.attachment))
				if err != nil || config.Width != 20 || config.Height != 10 {
					t.Fatal("normalized image missing")
				}
			}
		})
	}
}

func TestSharedSDKPublicButReportsAndImagesNotPublic(t *testing.T) {
	h := New(&fakeRepo{}, auth.New(""), []string{"https://tools.integ.life"}).Handler()
	req := httptest.NewRequest("GET", "/v1/feedback/client.js", nil)
	req.Header.Set("Origin", "https://tools.integ.life")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "prepareFeedbackImage") || rr.Header().Get("Access-Control-Allow-Origin") != "https://tools.integ.life" {
		t.Fatal("shared client unavailable")
	}
	for _, path := range []string{"/v1/feedback", "/v1/feedback/f1/image", "/internal/feedback"} {
		req = httptest.NewRequest("GET", path, nil)
		req.Header.Set("X-Project-Key", "pk_test")
		rr = httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code == 200 {
			t.Fatal("private reports exposed via project key")
		}
	}
}

func TestImageReportRateLimit(t *testing.T) {
	s := New(&fakeRepo{}, auth.New(""), nil)
	for i := 0; i < 30; i++ {
		if !s.allowUpload("project-1") {
			t.Fatal("too early")
		}
	}
	if s.allowUpload("project-1") {
		t.Fatal("project upload cap missing")
	}
	if !s.allowUpload("project-2") {
		t.Fatal("project isolation missing")
	}
}
