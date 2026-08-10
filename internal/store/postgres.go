package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct{ pool *pgxpool.Pool }

func Open(ctx context.Context, url string) (*Postgres, error) {
	p, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err = p.Ping(ctx); err != nil {
		p.Close()
		return nil, err
	}
	return &Postgres{pool: p}, nil
}
func (p *Postgres) Close() { p.pool.Close() }

func (p *Postgres) ProjectForKey(ctx context.Context, key string) (string, error) {
	var id string
	err := p.pool.QueryRow(ctx, `SELECT project_id::text FROM project_keys WHERE key_hash = encode(digest($1,'sha256'),'hex') AND revoked_at IS NULL`, key).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrForbidden
	}
	return id, err
}

func (p *Postgres) ListComments(ctx context.Context, project, resource string, limit int, after string) ([]Comment, string, error) {
	if limit < 1 || limit > 100 {
		limit = 30
	}
	var cursor time.Time
	if after == "" {
		cursor = time.Unix(1, 0)
	} else if t, err := time.Parse(time.RFC3339Nano, after); err == nil {
		cursor = t
	} else {
		return nil, "", errors.New("invalid cursor")
	}
	rows, err := p.pool.Query(ctx, `SELECT id::text, COALESCE(parent_id::text,''), body, COALESCE(user_id,''), COALESCE(NULLIF(author_name,''),'Guest'), user_id IS NOT NULL, created_at, updated_at FROM comments WHERE project_id=$1 AND resource=$2 AND deleted_at IS NULL AND created_at>$3 ORDER BY created_at,id LIMIT $4`, project, resource, cursor, limit+1)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	out := make([]Comment, 0, limit)
	for rows.Next() {
		var c Comment
		c.ProjectID = project
		c.Resource = resource
		if err = rows.Scan(&c.ID, &c.ParentID, &c.Body, &c.Author.UserID, &c.Author.Name, &c.Author.Registered, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, "", err
		}
		out = append(out, c)
	}
	next := ""
	if len(out) > limit {
		next = out[limit-1].CreatedAt.Format(time.RFC3339Nano)
		out = out[:limit]
	}
	return out, next, rows.Err()
}

func (p *Postgres) CreateComment(ctx context.Context, project, resource, parent, body string, a Actor) (Comment, error) {
	c := Comment{ID: uuid.NewString(), ProjectID: project, Resource: resource, ParentID: parent, Body: body, Author: Author{Name: a.Name, UserID: a.UserID, Registered: a.Registered}}
	var uid any
	if a.Registered {
		uid = a.UserID
	}
	var pid any
	if parent != "" {
		pid = parent
	}
	err := p.pool.QueryRow(ctx, `INSERT INTO comments(id,project_id,resource,parent_id,body,user_id,author_name,author_email)
		SELECT $1,$2,$3,$4,$5,$6,$7,NULLIF($8,'')
		WHERE $4::uuid IS NULL OR EXISTS (SELECT 1 FROM comments WHERE id=$4 AND project_id=$2 AND resource=$3 AND deleted_at IS NULL)
		RETURNING created_at,updated_at`, c.ID, project, resource, pid, body, uid, a.Name, a.Email).Scan(&c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Comment{}, ErrNotFound
	}
	return c, err
}

func (p *Postgres) DeleteComment(ctx context.Context, project, id string, a Actor) error {
	tag, err := p.pool.Exec(ctx, `UPDATE comments SET deleted_at=now(), body='[deleted]', author_email=NULL WHERE project_id=$1 AND id=$2 AND deleted_at IS NULL AND user_id=$3`, project, id, a.UserID)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (p *Postgres) CreateFeedback(ctx context.Context, project, resource, kind, body string, a Actor) (Feedback, error) {
	f := Feedback{ID: uuid.NewString(), ProjectID: project, Resource: resource, Kind: kind, Body: body, Status: "new", Author: Author{Name: a.Name, UserID: a.UserID, Registered: a.Registered}}
	var uid any
	if a.Registered {
		uid = a.UserID
	}
	err := p.pool.QueryRow(ctx, `INSERT INTO feedback(id,project_id,resource,kind,body,user_id,author_name,author_email) VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,'')) RETURNING created_at`, f.ID, project, resource, kind, body, uid, a.Name, a.Email).Scan(&f.CreatedAt)
	return f, err
}
