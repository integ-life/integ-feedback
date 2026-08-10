package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/integ-life/integ-feedback/internal/auth"
	"github.com/integ-life/integ-feedback/internal/store"
)

type fakeRepo struct {
	actor         store.Actor
	feedbackActor store.Actor
}

func (f *fakeRepo) ProjectForKey(_ context.Context, k string) (string, error) {
	if k != "pk_test" {
		return "", store.ErrForbidden
	}
	return "project-1", nil
}
func (f *fakeRepo) ListComments(context.Context, string, string, int, string) ([]store.Comment, string, error) {
	return []store.Comment{{ID: "c1", Body: "hello", Author: store.Author{Name: "Guest"}, CreatedAt: time.Now()}}, "", nil
}
func (f *fakeRepo) CreateComment(_ context.Context, p, r, parent, body string, a store.Actor) (store.Comment, error) {
	f.actor = a
	return store.Comment{ID: "c2", ProjectID: p, Resource: r, Body: body, Author: store.Author{Name: a.Name, Registered: a.Registered}}, nil
}
func (f *fakeRepo) DeleteComment(context.Context, string, string, store.Actor) error { return nil }
func (f *fakeRepo) CreateFeedback(_ context.Context, p, r, k, b string, a store.Actor) (store.Feedback, error) {
	f.feedbackActor = a
	return store.Feedback{ID: "f1", ProjectID: p, Resource: r, Kind: k, Body: b}, nil
}

func TestGuestCommentKeepsEmailPrivate(t *testing.T) {
	repo := &fakeRepo{}
	h := New(repo, auth.New(""), []string{"https://integ.life"}).Handler()
	req := httptest.NewRequest("POST", "/v1/comments", strings.NewReader(`{"resource":"/post/1","body":"Useful","guest_name":"Ada","guest_email":"ada@example.com"}`))
	req.Header.Set("X-Project-Key", "pk_test")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 201 {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if repo.actor.Email != "ada@example.com" {
		t.Fatal("email not passed to private store")
	}
	if strings.Contains(rr.Body.String(), "ada@example.com") {
		t.Fatal("private email leaked")
	}
	if !strings.Contains(rr.Body.String(), `"registered":false`) {
		t.Fatal("guest not marked")
	}
}

func TestInvalidKeyRejected(t *testing.T) {
	repo := &fakeRepo{}
	h := New(repo, auth.New(""), nil).Handler()
	req := httptest.NewRequest("GET", "/v1/comments?resource=x", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 401 {
		t.Fatalf("got %d", rr.Code)
	}
}

func TestBearerIdentityComesFromUserInfo(t *testing.T) {
	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer valid" {
			w.WriteHeader(401)
			return
		}
		io.WriteString(w, `{"sub":"user-7","name":"Lin","email":"lin@example.com"}`)
	}))
	defer idp.Close()
	repo := &fakeRepo{}
	h := New(repo, auth.New(idp.URL), nil).Handler()
	req := httptest.NewRequest("POST", "/v1/comments", strings.NewReader(`{"resource":"x","body":"hello","guest_name":"Fake"}`))
	req.Header.Set("X-Project-Key", "pk_test")
	req.Header.Set("Authorization", "Bearer valid")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 201 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	if !repo.actor.Registered || repo.actor.UserID != "user-7" || repo.actor.Name != "Lin" {
		t.Fatalf("actor=%+v", repo.actor)
	}
}

func TestCORSPreflight(t *testing.T) {
	h := New(&fakeRepo{}, auth.New(""), []string{"https://integ.life"}).Handler()
	req := httptest.NewRequest("OPTIONS", "/v1/comments", nil)
	req.Header.Set("Origin", "https://integ.life")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 204 || rr.Header().Get("Access-Control-Allow-Origin") != "https://integ.life" {
		t.Fatalf("bad preflight: %d %v", rr.Code, rr.Header())
	}
}

func TestBrowserComponentIsPublic(t *testing.T) {
	h := New(&fakeRepo{}, auth.New(""), nil).Handler()
	req := httptest.NewRequest("GET", "/sdk/v1/comments.js", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `customElements.define("integ-comments"`) {
		t.Fatalf("browser component unavailable: %d", rr.Code)
	}
}
