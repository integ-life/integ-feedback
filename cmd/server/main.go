package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/integ-life/integ-feedback/internal/api"
	"github.com/integ-life/integ-feedback/internal/auth"
	"github.com/integ-life/integ-feedback/internal/store"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	db := mustEnv("DATABASE_URL")
	repo, err := store.Open(ctx, db)
	if err != nil {
		log.Fatal(err)
	}
	defer repo.Close()
	origins := split(os.Getenv("ALLOWED_ORIGINS"))
	resolver := auth.New(os.Getenv("OIDC_USERINFO_URL"))
	srv := &http.Server{Addr: env("HTTP_ADDR", ":8080"), Handler: api.New(repo, resolver, origins).Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(c)
	}()
	log.Printf("integ-feedback listening on %s", srv.Addr)
	if err = srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("%s is required", k)
	}
	return v
}
func split(v string) []string {
	var out []string
	for _, x := range strings.Split(v, ",") {
		if strings.TrimSpace(x) != "" {
			out = append(out, strings.TrimSpace(x))
		}
	}
	return out
}
