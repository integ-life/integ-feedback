package api

import (
	_ "embed"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"github.com/integ-life/integ-feedback/internal/auth"
	"github.com/integ-life/integ-feedback/internal/store"
)

//go:embed assets/comments.js
var commentsJS []byte

type Server struct {
	repo    store.Repository
	auth    *auth.Resolver
	allowed map[string]bool
}

func New(repo store.Repository, resolver *auth.Resolver, origins []string) *Server {
	m := map[string]bool{}
	for _, o := range origins {
		m[strings.TrimSpace(o)] = true
	}
	return &Server{repo: repo, auth: resolver, allowed: m}
}

func (s *Server) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { write(w, 200, map[string]string{"status": "ok"}) })
	m.HandleFunc("GET /sdk/v1/comments.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		_, _ = w.Write(commentsJS)
	})
	m.HandleFunc("GET /v1/comments", s.listComments)
	m.HandleFunc("POST /v1/comments", s.createComment)
	m.HandleFunc("DELETE /v1/comments/{id}", s.deleteComment)
	m.HandleFunc("POST /v1/feedback", s.createFeedback)
	return s.middleware(m)
}
func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		o := r.Header.Get("Origin")
		if o != "" && s.allowed[o] {
			w.Header().Set("Access-Control-Allow-Origin", o)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Project-Key")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")
		}
		if r.Method == http.MethodOptions {
			if !s.allowed[o] {
				problem(w, 403, "origin_not_allowed", "Origin is not allowed")
			} else {
				w.WriteHeader(204)
			}
			return
		}
		if r.URL.Path != "/healthz" && !strings.HasPrefix(r.URL.Path, "/sdk/") {
			p, err := s.repo.ProjectForKey(r.Context(), r.Header.Get("X-Project-Key"))
			if err != nil {
				problem(w, 401, "invalid_project_key", "A valid project key is required")
				return
			}
			r.Header.Set("X-Project-ID", p)
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) listComments(w http.ResponseWriter, r *http.Request) {
	resource := r.URL.Query().Get("resource")
	if !validResource(resource) {
		problem(w, 400, "invalid_resource", "resource must be 1-500 characters")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, next, err := s.repo.ListComments(r.Context(), r.Header.Get("X-Project-ID"), resource, limit, r.URL.Query().Get("after"))
	if err != nil {
		problem(w, 400, "invalid_request", err.Error())
		return
	}
	write(w, 200, map[string]any{"items": items, "next_cursor": next})
}

type createCommentRequest struct {
	Resource   string `json:"resource"`
	ParentID   string `json:"parent_id"`
	Body       string `json:"body"`
	GuestName  string `json:"guest_name"`
	GuestEmail string `json:"guest_email"`
}

func (s *Server) createComment(w http.ResponseWriter, r *http.Request) {
	var in createCommentRequest
	if !decode(w, r, &in) || !validResource(in.Resource) || len(strings.TrimSpace(in.Body)) < 1 || len(in.Body) > 10000 {
		problem(w, 400, "invalid_comment", "resource and body (1-10000 chars) are required")
		return
	}
	a, err := s.actor(r, in.GuestName, in.GuestEmail)
	if err != nil {
		problem(w, 401, "invalid_identity", err.Error())
		return
	}
	c, err := s.repo.CreateComment(r.Context(), r.Header.Get("X-Project-ID"), in.Resource, in.ParentID, strings.TrimSpace(in.Body), a)
	if errors.Is(err, store.ErrNotFound) {
		problem(w, 400, "invalid_parent", "Parent comment must exist in the same thread")
		return
	}
	if err != nil {
		problem(w, 500, "store_error", "Could not save comment")
		return
	}
	write(w, 201, c)
}

func (s *Server) deleteComment(w http.ResponseWriter, r *http.Request) {
	a, err := s.actor(r, "", "")
	if err != nil || !a.Registered {
		problem(w, 401, "authentication_required", "Sign in to delete a comment")
		return
	}
	err = s.repo.DeleteComment(r.Context(), r.Header.Get("X-Project-ID"), r.PathValue("id"), a)
	if errors.Is(err, store.ErrNotFound) {
		problem(w, 404, "not_found", "Comment not found or not owned by user")
		return
	}
	if err != nil {
		problem(w, 500, "store_error", "Could not delete comment")
		return
	}
	w.WriteHeader(204)
}

type feedbackRequest struct {
	Resource   string `json:"resource"`
	Kind       string `json:"kind"`
	Body       string `json:"body"`
	GuestName  string `json:"guest_name"`
	GuestEmail string `json:"guest_email"`
}

func (s *Server) createFeedback(w http.ResponseWriter, r *http.Request) {
	var in feedbackRequest
	if !decode(w, r, &in) || !validResource(in.Resource) || !map[string]bool{"idea": true, "issue": true, "question": true, "other": true}[in.Kind] || len(strings.TrimSpace(in.Body)) < 1 || len(in.Body) > 10000 {
		problem(w, 400, "invalid_feedback", "resource, kind and body are required")
		return
	}
	a, err := s.actor(r, in.GuestName, in.GuestEmail)
	if err != nil {
		problem(w, 401, "invalid_identity", err.Error())
		return
	}
	f, err := s.repo.CreateFeedback(r.Context(), r.Header.Get("X-Project-ID"), in.Resource, in.Kind, strings.TrimSpace(in.Body), a)
	if err != nil {
		problem(w, 500, "store_error", "Could not save feedback")
		return
	}
	write(w, 201, f)
}

func (s *Server) actor(r *http.Request, name, email string) (store.Actor, error) {
	if len(name) > 100 || len(email) > 320 {
		return store.Actor{}, errors.New("guest identity is too long")
	}
	if email != "" {
		if _, err := mail.ParseAddress(email); err != nil {
			return store.Actor{}, errors.New("guest email is invalid")
		}
	}
	return s.auth.Resolve(r.Context(), r.Header.Get("Authorization"), store.Actor{Name: strings.TrimSpace(name), Email: strings.TrimSpace(email)})
}
func validResource(v string) bool { return len(v) > 0 && len(v) <= 500 }
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	return d.Decode(v) == nil
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, status int, code, message string) {
	write(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
