-- Private JPEG bytes belong to feedback, never to public comments or URLs.
ALTER TABLE feedback ADD COLUMN IF NOT EXISTS attachment bytea;
ALTER TABLE feedback ADD COLUMN IF NOT EXISTS attachment_consent_at timestamptz;
ALTER TABLE feedback ADD CONSTRAINT feedback_attachment_size CHECK (attachment IS NULL OR octet_length(attachment) BETWEEN 1 AND 524288);
ALTER TABLE feedback ADD CONSTRAINT feedback_attachment_consent CHECK ((attachment IS NULL) = (attachment_consent_at IS NULL));
