# Project instructions

## Verification

- Run `go test ./...` for backend or deployment changes.
- Run `npm --prefix sdk/web test` for Web SDK changes.
- Cross-compile the server for Linux before production deployment.

## Machine context

- Read the repo-root `.agent-machine.env` before making deployment or runtime assumptions.
- `.agent-machine.env` contains untracked machine-local facts and must never be committed or copied into logs, pull requests, or chat.
- `.agent-machine.env.example` is the tracked schema and safe example only; do not treat its values as live facts.
- Interpret `INTEG_LIFE_MACHINE_KIND`, `INTEG_LIFE_BACKEND_MODE`, `INTEG_LIFE_BACKEND_BASE_URL`, `INTEG_LIFE_BACKEND_ROLE`, `INTEG_LIFE_HOST_TAG`, and `INTEG_LIFE_NOTES` when present.
- This repository's production topology is documented in `deploy/production/` and the central `second-brain/context/knowledge/repo-deployment-map.md` index.
- Confirm runtime claims with live service, port, database, DNS, or HTTP checks; configuration alone is not runtime proof.
- Generic deploy requests default to a non-production target. Deploying to `discuss.integ.life` requires an explicit production request.

## Production safety

- Build the Go binary locally and upload it; do not compile on the production host.
- Preserve `/etc/integ-feedback.env`, the `integ-feedback-postgres-data` Docker volume, and unrelated Caddy fragments.
- Never print or commit the production database password or environment file.
