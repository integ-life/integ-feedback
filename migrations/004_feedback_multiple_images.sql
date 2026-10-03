BEGIN;
ALTER TABLE feedback ADD COLUMN IF NOT EXISTS extra_attachments bytea[] NOT NULL DEFAULT ARRAY[]::bytea[];
ALTER TABLE feedback ADD CONSTRAINT feedback_extra_attachment_limit CHECK (cardinality(extra_attachments) <= 3);
ALTER TABLE feedback ADD CONSTRAINT feedback_extra_attachment_consent CHECK (cardinality(extra_attachments) = 0 OR attachment IS NOT NULL);
COMMIT;
