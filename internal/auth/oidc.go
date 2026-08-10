package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/integ-life/integ-feedback/internal/store"
)

type Resolver struct {
	UserInfoURL string
	Client      *http.Client
}

func New(userInfoURL string) *Resolver {
	return &Resolver{UserInfoURL: strings.TrimRight(userInfoURL, "/"), Client: &http.Client{Timeout: 5 * time.Second}}
}

func (r *Resolver) Resolve(ctx context.Context, header string, guest store.Actor) (store.Actor, error) {
	if header == "" {
		guest.Registered = false
		if guest.Name == "" {
			guest.Name = "Guest"
		}
		return guest, nil
	}
	if !strings.HasPrefix(header, "Bearer ") || r.UserInfoURL == "" {
		return store.Actor{}, errors.New("invalid authorization")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, r.UserInfoURL, nil)
	req.Header.Set("Authorization", header)
	res, err := r.Client.Do(req)
	if err != nil {
		return store.Actor{}, errors.New("identity provider unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return store.Actor{}, errors.New("invalid access token")
	}
	var u struct {
		Sub, Name, Email  string
		PreferredUsername string `json:"preferred_username"`
	}
	if json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&u) != nil || u.Sub == "" {
		return store.Actor{}, errors.New("invalid userinfo")
	}
	if u.Name == "" {
		u.Name = u.PreferredUsername
	}
	if u.Name == "" {
		u.Name = u.Email
	}
	if u.Name == "" {
		u.Name = "User"
	}
	return store.Actor{UserID: u.Sub, Name: u.Name, Email: u.Email, Registered: true}, nil
}
