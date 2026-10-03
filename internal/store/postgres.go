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

// Feedback text remains in the queue; only expired private image bytes expire.
func (p *Postgres) PruneAttachments(ctx context.Context) error {
	_, err := p.pool.Exec(ctx, `UPDATE feedback SET attachment=NULL,extra_attachments=ARRAY[]::bytea[],attachment_consent_at=NULL WHERE (attachment IS NOT NULL OR cardinality(extra_attachments)>0) AND created_at<now()-interval '30 days'`)
	return err
}

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

func (p *Postgres) CreateFeedback(ctx context.Context, project, resource, kind, body string, a Actor, attachments [][]byte) (Feedback, error) {
	f := Feedback{ID: uuid.NewString(), ProjectID: project, Resource: resource, Kind: kind, Body: body, Status: "new", Author: Author{Name: a.Name, UserID: a.UserID, Registered: a.Registered}}
	var uid any
	if a.Registered {
		uid = a.UserID
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return f, err
	}
	defer tx.Rollback(ctx)
	var consent any
	var attachment []byte
	extra := make([][]byte, 0)
	if len(attachments) > 0 {
		attachment = attachments[0]
		extra = attachments[1:]
		// Serialize the project quota check with the insert, including concurrent uploads.
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, project); err != nil {
			return f, err
		}
		if _, err = tx.Exec(ctx, `UPDATE feedback SET attachment=NULL,extra_attachments=ARRAY[]::bytea[],attachment_consent_at=NULL WHERE project_id=$1 AND (attachment IS NOT NULL OR cardinality(extra_attachments)>0) AND created_at<now()-interval '30 days'`, project); err != nil {
			return f, err
		}
		var used int64
		if err = tx.QueryRow(ctx, `SELECT COALESCE(sum(COALESCE(octet_length(attachment),0)+COALESCE((SELECT sum(octet_length(bytes)) FROM unnest(extra_attachments) AS item(bytes)),0)),0) FROM feedback WHERE project_id=$1`, project).Scan(&used); err != nil {
			return f, err
		}
		incoming := int64(0)
		for _, image := range attachments {
			incoming += int64(len(image))
		}
		if used+incoming > 256<<20 {
			return f, ErrAttachmentQuota
		}
		consent = time.Now().UTC()
		f.HasAttachment = true
		f.AttachmentCount = len(attachments)
	}
	err = tx.QueryRow(ctx, `INSERT INTO feedback(id,project_id,resource,kind,body,user_id,author_name,author_email,attachment,attachment_consent_at,extra_attachments) VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),$9,$10,$11) RETURNING created_at`, f.ID, project, resource, kind, body, uid, a.Name, a.Email, attachment, consent, extra).Scan(&f.CreatedAt)
	if err == nil {
		err = tx.Commit(ctx)
	}
	return f, err
}
