#!/bin/sh
set -eu

SOURCE_DIR=${1:-/tmp/integ-feedback-release}
ENV_FILE=/etc/integ-feedback.env
DB_CONTAINER=integ-feedback-postgres
DB_VOLUME=integ-feedback-postgres-data

test -x "$SOURCE_DIR/integ-feedback"
test -d "$SOURCE_DIR/migrations"

if ! id integ-feedback >/dev/null 2>&1; then
  useradd --system --home /nonexistent --shell /usr/sbin/nologin integ-feedback
fi

install -m 0755 "$SOURCE_DIR/integ-feedback" /usr/local/bin/integ-feedback
install -m 0644 "$SOURCE_DIR/integ-feedback.service" /etc/systemd/system/integ-feedback.service
install -m 0644 "$SOURCE_DIR/discuss.caddy" /etc/caddy/sites-enabled/discuss.caddy

if [ ! -f "$ENV_FILE" ]; then
  db_password=$(openssl rand -hex 32)
  env_temp=$(mktemp)
  chmod 0600 "$env_temp"
  {
    printf 'DATABASE_URL=postgres://integ_feedback:%s@127.0.0.1:5433/integ_feedback?sslmode=disable\n' "$db_password"
    printf 'HTTP_ADDR=127.0.0.1:8385\n'
    printf 'OIDC_USERINFO_URL=https://auth.integ.life/userinfo\n'
    printf 'ALLOWED_ORIGINS=https://integ.life,https://www.integ.life,https://games.integ.life,https://tools.integ.life\n'
  } >"$env_temp"
  install -o root -g root -m 0600 "$env_temp" "$ENV_FILE"
  rm "$env_temp"
else
  db_password=$(sed -n 's#^DATABASE_URL=postgres://integ_feedback:\([^@]*\)@.*#\1#p' "$ENV_FILE")
fi
test -n "$db_password"

docker volume create "$DB_VOLUME" >/dev/null
if ! docker container inspect "$DB_CONTAINER" >/dev/null 2>&1; then
  docker run -d \
    --name "$DB_CONTAINER" \
    --restart unless-stopped \
    -e POSTGRES_USER=integ_feedback \
    -e POSTGRES_PASSWORD="$db_password" \
    -e POSTGRES_DB=integ_feedback \
    -p 127.0.0.1:5433:5432 \
    -v "$DB_VOLUME:/var/lib/postgresql/data" \
    postgres:17-alpine >/dev/null
fi

attempt=0
until docker exec "$DB_CONTAINER" pg_isready -U integ_feedback -d integ_feedback >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  [ "$attempt" -lt 30 ] || { docker logs --tail 80 "$DB_CONTAINER"; exit 1; }
  sleep 2
done

docker exec -i "$DB_CONTAINER" psql -v ON_ERROR_STOP=1 -U integ_feedback -d integ_feedback <<'SQL'
CREATE TABLE IF NOT EXISTS schema_migrations (
  name text PRIMARY KEY,
  applied_at timestamptz NOT NULL DEFAULT now()
);
SQL

for migration in "$SOURCE_DIR"/migrations/*.sql; do
  name=$(basename "$migration")
  applied=$(docker exec "$DB_CONTAINER" psql -At -U integ_feedback -d integ_feedback -c "SELECT 1 FROM schema_migrations WHERE name='$name'")
  [ "$applied" = "1" ] && continue
  {
    printf 'BEGIN;\n'
    cat "$migration"
    printf "\nINSERT INTO schema_migrations(name) VALUES ('%s');\nCOMMIT;\n" "$name"
  } | docker exec -i "$DB_CONTAINER" psql -v ON_ERROR_STOP=1 -U integ_feedback -d integ_feedback
done

systemctl daemon-reload
systemctl enable integ-feedback.service >/dev/null
systemctl restart integ-feedback.service
caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
systemctl reload caddy.service

attempt=0
until curl --fail --silent http://127.0.0.1:8385/healthz >/dev/null; do
  attempt=$((attempt + 1))
  [ "$attempt" -lt 20 ] || { journalctl -u integ-feedback.service -n 80 --no-pager; exit 1; }
  sleep 1
done

systemctl --no-pager --full status integ-feedback.service | sed -n '1,12p'
