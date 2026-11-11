# Currents

Currents is a calm, ad-free place to collect visual inspiration. It is open source and built on the [AT Protocol](https://atproto.com), so your saves and collections are stored in your AT Protocol repository.

Visit [currents.is](https://currents.is) to try it.

## What you can do

- Save images into collections and sections, or keep them unsorted on your profile.
- Explore images, search by text or color, and find similar images.
- Save images from other websites with the [browser extension](extension/README.md).
- Import boards from Pinterest.

Some search features are available with a supporter subscription. See the [supporter guide](docs/POLAR.md) for details.

## How it works

The Svelte frontend and browser extension talk to a Go AppView. Saves are written to the user's personal data server (PDS), then indexed asynchronously by TAP. The AppView uses PostgreSQL with pgvector for search and discovery. A Python inference service creates image and text embeddings in the same space.

| Directory | What's inside |
| --- | --- |
| [frontend/](frontend/) | Web and mobile app |
| [extension/](extension/) | Browser extension |
| [appview/](appview/) | API, OAuth, indexing and database |
| [inference/](inference/) | Image and text embeddings |
| [clustering/](clustering/) | Background embedding and clustering jobs |
| [lexicons/](lexicons/) | AT Protocol record and API schemas |

## Local development

You need Docker with Compose, Python 3.10+, and Node.js 22.17 or later. The inference server uses Apple Silicon when available and falls back to CPU. The frontend depends on a private Capawesome package: configure registry credentials as described in the [frontend guide](frontend/README.md#native-security-updates) before installing dependencies.

Install the frontend and inference dependencies:

```bash
cd frontend
npm install
cd ../inference
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
cd ..
```

Copy the root [.env.example](.env.example) to `.env` and adjust it for your development environment. Set `SESSION_SECRET` to a random value and configure the OAuth hostname and frontend URL. In `frontend/.env.development.local`, set `PUBLIC_APPVIEW_URL` and `PUBLIC_WEB_APPVIEW_URL` to your development AppView; the checked-in development settings point to the live service. Login requires OAuth settings that match the address you use. See the [AppView configuration and OAuth guide](appview/README.md#oauth-modes) for details.

Start the database, TAP, AppView and clustering service:

```bash
docker compose -f docker-compose.dev.yml up -d --build
```

The dev TAP stack follows the accounts in `TAP_DEV_HANDLES`; set that variable to a comma-separated list of handles to change them. The AppView listens on port 8080.

In separate terminals, start inference and the frontend:

```bash
cd inference
source venv/bin/activate
uvicorn main:app --reload
```

```bash
cd frontend
npm run dev
```

Inference listens on port 8000; Vite prints the frontend URL. The root `dev.sh` can start these services in tmux along with a Cloudflare tunnel. It expects tmux, cloudflared, a configured `currents-dev` tunnel, and the dependencies above.

## Documentation

See the [documentation index](docs/README.md) for architecture, features, operations and deployment guides. Component setup and commands are in the [AppView](appview/README.md), [inference](inference/README.md), [frontend](frontend/README.md) and [extension](extension/README.md) guides. Planned work is in [BACKLOG.md](BACKLOG.md).

## Contributing

Issues and pull requests are welcome. For a change to an existing subsystem, read its guide in the [documentation index](docs/README.md) first. Run the checks relevant to your change and describe how you tested it in your pull request.

## License

Currents is licensed under the [GNU Affero General Public License v3.0](LICENSE).
