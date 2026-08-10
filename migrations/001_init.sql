CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE projects (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug text NOT NULL UNIQUE,
  name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE project_keys (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  key_hash text NOT NULL UNIQUE,
  label text NOT NULL DEFAULT 'default',
  created_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz
);

CREATE TABLE comments (
  id uuid PRIMARY KEY,
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  resource text NOT NULL,
  parent_id uuid REFERENCES comments(id) ON DELETE CASCADE,
  body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 10000),
  user_id text,
  author_name text NOT NULL DEFAULT '',
  author_email text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE INDEX comments_thread_idx ON comments(project_id,resource,created_at,id) WHERE deleted_at IS NULL;
CREATE INDEX comments_parent_idx ON comments(parent_id) WHERE deleted_at IS NULL;

CREATE TYPE feedback_kind AS ENUM ('idea','issue','question','other');
CREATE TYPE feedback_status AS ENUM ('new','reviewing','planned','resolved','closed');
CREATE TABLE feedback (
  id uuid PRIMARY KEY,
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  resource text NOT NULL,
  kind feedback_kind NOT NULL,
  status feedback_status NOT NULL DEFAULT 'new',
  body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 10000),
  user_id text,
  author_name text NOT NULL DEFAULT '',
  author_email text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX feedback_queue_idx ON feedback(project_id,status,created_at DESC);

-- Bootstrap example (replace the key before production):
-- INSERT INTO projects(slug,name) VALUES ('integ-life','Integ Life');
-- INSERT INTO project_keys(project_id,key_hash) SELECT id,encode(digest('pk_dev_change_me','sha256'),'hex') FROM projects WHERE slug='integ-life';
