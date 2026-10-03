# Shared private image feedback

Integ Feedback owns the API, image preparation, consent checks, private storage,
receipts, retention and operator export. Integ Tools is the first consumer.

## Integrate another project

Use its own registered publishable project key and add its exact production
origin to `ALLOWED_ORIGINS`. Projects are isolated by the key's project ID and
its stable `resource`. Never use another project's key.

```js
import { FeedbackClient, prepareFeedbackImage } from
  'https://discuss.integ.life/v1/feedback/client.js';

const client = new FeedbackClient({
  apiUrl: 'https://discuss.integ.life',
  projectKey: 'YOUR_REGISTERED_PUBLISHABLE_PROJECT_KEY',
});
// Preparation does not upload. Preview the prepared JPEG first.
const image = await prepareFeedbackImage(fileInput.files[0]);
preview.src = `data:image/jpeg;base64,${image.base64}`;
// Call after the user consents and presses Submit.
const receipt = await client.submitFeedback({
  resource: 'feature:camera',
  kind: 'issue',
  body: 'Describe the failure and include only diagnostics shown to the user.',
  images: [image], // Add up to three additional prepared photos.
  attachmentConsent: true,
});
// Show receipt.id to the user. Do not log image data.
```

The SDK source is `sdk/web` (`@integ-life/feedback`). Browser applications can
import the public ES module above without maintaining a copied implementation.
The `/v1/` path bypasses the legacy `/sdk/*` static CDN worker.

`POST /v1/feedback` accepts optional `image_base64` (one image) or `images_base64` (array), and `attachment_consent`.
`GET /v1/feedback/capabilities` requires the project key and exposes the limits.
Responses include `id`, `has_attachment` and `attachment_count`. Text-only clients remain compatible.
No report-list or image-download endpoint is public.

## Limits and privacy

- Up to four JPEG attachments per report, at most 512 KiB and 1280 pixels per edge.
- Preparation accepts JPEG, PNG or WebP up to 12 MiB, converts locally to JPEG,
  handles orientation, and flattens transparency on white.
- The backend decodes and re-encodes JPEGs, removing EXIF/location metadata,
  filenames and appended payloads. Claimed MIME types are not trusted.
- Explicit consent is mandatory. Comments cannot contain image attachments.
  Public receipts do not contain image bytes.
- A project can submit 30 images per minute, with a serialized 256 MiB storage
  quota per project. Feedback text remains in the existing queue.
- Images expire after 30 days. The service removes expired bytes on startup,
  every six hours and before image inserts. Export never returns expired images.
- Requests time out after 30 seconds. An optional client `signal` cancels requests
  on consumer unmount. Missing receipts are not success; previews stay for retry.

## Internal maintenance

`feedback-export` requires private database access, not a publishable project
key. It lists reports or exports one report into a new directory (mode 0700)
containing `report.json`, `image.jpg` and optional `image-2.jpg` through
`image-4.jpg` (mode 0600). Keep exports private.

On production, load the protected environment without printing credentials:

```sh
sudo sh -c 'set -a; . /etc/integ-feedback.env; set +a; exec /usr/local/bin/integ-feedback-export --project tools --limit 20'
sudo sh -c 'set -a; . /etc/integ-feedback.env; set +a; exec /usr/local/bin/integ-feedback-export --project tools --id REPORT_UUID --output /tmp/NEW_PRIVATE_EXPORT_DIRECTORY'
```

Run `003_feedback_attachments.sql` and then `004_feedback_multiple_images.sql`
once each before restarting the
new binary. Preserve the database and environment. Build locally and install the
Linux server and export binaries; verify the receipt and persisted JPEG using a
clearly labeled synthetic report.

```sh
npm --prefix sdk/web ci
node scripts/build-web-sdk.mjs
npm --prefix sdk/web test
go test ./...
GOOS=linux GOARCH=arm64 go build -o /tmp/integ-feedback ./cmd/server
GOOS=linux GOARCH=arm64 go build -o /tmp/integ-feedback-export ./cmd/feedback-export
```

`build-web-sdk.mjs` regenerates the embedded public JS from the single TypeScript
source. Commit the embedded artifact with SDK changes.

Multiple images are validated and saved atomically. Consent covers all previews;
changing the selection requires renewed consent in the consumer UI. Rate and
storage quotas count every image. Legacy `image` SDK calls remain supported.
Use a version query on the module URL when adopting batch uploads so returning
browsers do not reuse the previous single-image client.
