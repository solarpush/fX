# fX - Factur-X / EN16931 Generation Pipeline

A complete open-source solution (CLI, API Server & Web UI) to generate, validate, and extract electronic invoices compliant with the Factur-X (ZUGFeRD 2.4 / EN16931) standard.

## Capabilities

- PDF/A-3 Generator: Compilation of highly customizable templates using Typst.
- CII XML Builder: Strict creation of the `factur-x.xml` file compliant with the EN16931 standard.
- API Server: Easy integration via JSON HTTP requests.
- Web Builder UI (Angular): Modern interface to create invoices, manage templates, and preview live.
- CLI Tool: For integration into CI/CD pipelines or system scripts.
- Strict Validation: Verification of business rules. Only the EN16931 and EXTENDED profiles are supported.
- High Performance: Highly concurrent Go architecture. Capable of generating ~165 full Factur-X PDFs per second under load with sub-600ms response times for 100 concurrent requests, avoiding the memory overhead of headless browsers.
- Supported Document Types: Handles standard invoices (380), credit notes (381), corrected invoices (384), prepaid invoices (386), self-billed invoices (389), and proforma/information invoices (751).

## Quick Local Setup (Docker Compose)

The entire project (Go Server, Angular Web UI, and Typst) is packaged to run directly with Docker Compose. This is all you need for local development and testing:

```bash
git clone https://github.com/solarpush/fX.git
cd fX
docker-compose up -d
```

- Web Builder UI: Available at `http://localhost:4200`
- API HTTP Backend: Available at `http://localhost:8080/api/v1`

### API Usage Example

**POST** `/api/v1/generate`

```json
{
  "invoice": {
    "profile": "EN16931",
    "invoice": {
      "number": "INV-2026-001",
      "issue_date": "2026-01-15T00:00:00Z",
      "currency": "EUR"
    },
    "seller": {
      "name": "Tech Corp",
      "address": { "street": "123 Rue", "postal_code": "75000", "city": "Paris", "country": "FR" },
      "vat_id": "FR12345678901"
    },
    "buyer": { ... },
    "lines": [ ... ]
  },
  "options": {
    "templateId": "test55.typ"
  }
}
```

## Cloud Run Deployment (with Cloud Storage)

For a serverless deployment on Google Cloud Platform, you can deploy the Docker container to Cloud Run and connect it to a Cloud Storage bucket for persistence.

1. Push the Docker image to Artifact Registry:

```bash
docker build -t gcr.io/YOUR_PROJECT_ID/fx-server .
docker push gcr.io/YOUR_PROJECT_ID/fx-server
```

2. Deploy to Cloud Run:

```bash
gcloud run deploy fx-server \
  --image gcr.io/YOUR_PROJECT_ID/fx-server \
  --platform managed \
  --allow-unauthenticated \
  --env-vars-file .env.production
```

_(Make sure the Cloud Run service account has Storage Object Admin permissions on the bucket)_

## VPS Deployment (Docker Compose + Reverse Proxy)

For a traditional VPS deployment (e.g., Ubuntu/Debian), you can run the application using `docker-compose` behind a reverse proxy (like Nginx or Traefik) to handle SSL/TLS and proper networking.

1. Ensure Docker and Docker Compose are installed on your VPS.
2. Clone the repository and configure your `.env` file (set strong authentication secrets if exposing the API directly).
3. Use a standard `docker-compose.yml` that binds to a specific bridge network, avoiding exposing ports directly to the public internet:

```yaml
version: "3.8"
services:
  fx-server:
    build: .
    environment:
      - PORT=8080
      - HOST=0.0.0.0
      - STORAGE_TYPE=local
      - STORAGE_LOCAL_PATH=/storage
    volumes:
      - ./storage:/storage
      - ./templates-custom:/templates-custom
    restart: unless-stopped
    networks:
      - proxy_network

networks:
  proxy_network:
    external: true
```

4. Configure your Reverse Proxy (e.g., Nginx) to forward traffic to the `fx-server` container on port 8080, and secure it with Let's Encrypt.

```nginx
server {
    listen 80;
    server_name fx.yourdomain.com;

    location / {
        proxy_pass http://fx-server:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Environment Variables

The application can be configured via environment variables. Here are the most important ones:

### Server & Security

- `PORT` (default: 8080): The port the API listens on.
- `AUTH_ENABLED` (default: false): Enable API key and password authentication.
- `AUTH_API_KEY`: The API key required to access the backend if auth is enabled.
- `AUTH_PASSWORD`: The password for the Web UI if auth is enabled.
- `AUTH_JWT_SECRET`: Secret key used for signing JWTs in the Web UI.

### Storage

- `STORAGE_TYPE` (default: local): Where to store generated PDFs. Options: `local`, `s3`, `gcs`, `azure`.
- `STORAGE_LOCAL_PATH` (default: ./storage): Path for local storage.

**For S3/MinIO/GCS (when STORAGE_TYPE=s3 or gcs):**

- `S3_ENDPOINT`: URL of the storage endpoint (e.g., `https://storage.googleapis.com`).
- `S3_BUCKET`: Name of your bucket.
- `S3_ACCESS_KEY` & `S3_SECRET_KEY`: Your storage credentials.

### Typst & Templates

- `TEMPLATES_PATH`: Directory containing your custom `.typ` templates.
- `TYPST_FONT_PATHS`: Optional directory containing custom fonts for Typst.
- `TYPST_ROOT`: Virtual root directory for Typst (useful for resolving absolute image paths in templates).

### AI Generation (Optional)

- `AI_PROVIDER` (default: openai): AI provider. Options: `openai`, `ollama`.
- `AI_BASE_URL`: API URL for the provider (e.g., `https://api.deepseek.com` or local Ollama).
- `AI_MODEL`: The LLM model to use (e.g., `gemini-3.5-flash`, `deepseek-coder`).
- `AI_API_KEY`: Your API key for the AI provider.

## Standards & Compliance

- Factur-X 1.08
- ZUGFeRD 2.4
- EN16931 (European e-invoicing standard)
- PDF/A-3 (ISO 19005-3)
- UN/CEFACT CII D22B (Cross Industry Invoice)

## License

fX is licensed under the **GNU AGPL-3.0 License**.

For commercial use, embedding in proprietary software, or if you cannot comply with the AGPL-3.0 terms (e.g., you do not wish to open-source your entire application), please contact us to purchase a commercial license.
