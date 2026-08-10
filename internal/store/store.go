package store

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
)

type Actor struct {
	UserID, Name, Email string
	Registered          bool
}

type Comment struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id,omitempty"`
	Resource  string    `json:"resource"`
	ParentID  string    `json:"parent_id,omitempty"`
	Body      string    `json:"body"`
	Author    Author    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Author struct {
	Name       string `json:"name"`
	UserID     string `json:"user_id,omitempty"`
	Registered bool   `json:"registered"`
}

type Feedback struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id,omitempty"`
	Resource  string    `json:"resource"`
	Kind      string    `json:"kind"`
	Body      string    `json:"body"`
	Status    string    `json:"status"`
	Author    Author    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
}

type Repository interface {
	ProjectForKey(context.Context, string) (string, error)
	ListComments(context.Context, string, string, int, string) ([]Comment, string, error)
	CreateComment(context.Context, string, string, string, string, Actor) (Comment, error)
	DeleteComment(context.Context, string, string, Actor) error
	CreateFeedback(context.Context, string, string, string, string, Actor) (Feedback, error)
}
