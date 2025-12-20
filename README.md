## Brave Blueprint Programme

From https://wiki.bravecollective.com/public/alliance/industry/bpcprogram

- Brave Alliance only
- 10 BPCs per request
  - Requests for multiple of the same BPC is at the discretion of the BPC manager.
- Only the BPCs on the list can be requested.
- Please only have one request active at a time
  - **Per person NOT per character**
- Capital hull BPCs are provided at one per month per person due to scarcity (and because if you need them, you're generally wealthy enough to be able to get them from contracts)
- The program operates only from "E3OI-U - Librarian"

Anything that is unreasonable will be rejected. And this is a sliding scale, arbitrary and utterly at the discretion of the BPC Managers. If they blink at the request, they’ll reject it.

## Development

Create a development app at [developers.eveonline.com](https://developers.eveonline.com) with the callback URL `http://localhost:2727/login` and following scopes:
- esi-assets.read_corporation_assets.v1
- esi-corporations.read_blueprints.v1
- esi-industry.read_corporation_jobs.v1

Then create `backend/.env` with 
``` sh
ESI_APP_ID=<appid>
ESI_APP_SECRET=<secret>
ESI_APP_REDIRECT=http://localhost:2727/login
```

The backend container can now be built and run using
``` sh
docker compose up -d --build backend
```

Access to npm can be acquired via a node container linked in the `/app` directory
``` sh
docker run --rm -it --volume ./frontend:/app node:23-alpine sh
```

Running the frontend container provides a hotloading webserver at `localhost:3000` for react/frontend development.
``` sh
docker compose up -d frontend
```

## Production

### Docker (recommended)
- Use [docker-compose.prod.yaml](docker-compose.prod.yaml). It serves the SPA via nginx and reverse-proxies `/api/*`, `/session`, and `/logout` to the backend container so cookie auth works on a single origin.
- Production publishes `80:80` and `443:443` (Vite `:3000` is dev-only).
- Provide secrets via environment variables (compose interpolation), e.g. in a deploy-time env file:
  - `ESI_APP_ID`, `ESI_APP_SECRET`, `ESI_APP_REDIRECT`
  - `DB_USER`, `DB_PASS`, `DB_NAME`, `DB_ROOT_PASSWORD`

- TLS certs: mount `./deploy/nginx/certs/fullchain.pem` and `./deploy/nginx/certs/privkey.pem` (see the `frontend` service in the compose file).

Run:
```sh
docker compose -f docker-compose.prod.yaml up -d --build
```

### Docker + standalone nginx (host-installed)
- If you prefer to run nginx directly on the host (system package / certbot / etc), use [docker-compose.prod.host-nginx.yaml](docker-compose.prod.host-nginx.yaml).
- This runs DB + backend only, binding backend to `127.0.0.1:2727` for the host nginx to proxy to.

Run:
```sh
docker compose -f docker-compose.prod.host-nginx.yaml up -d --build
```

Then:
- Build the SPA and place it on disk:
  - Option A (dockerized build):
    ```sh
    docker compose -f docker-compose.prod.host-nginx.yaml run --rm frontend-build
    ```
    Outputs to `./deploy/frontend-dist`.
  - Option B (host build): `cd frontend && npm ci && npm run build`
- Configure host nginx to serve the SPA directory and proxy backend routes. Example: [deploy/nginx/brave-bpc.conf](deploy/nginx/brave-bpc.conf)

### Bare metal
- Backend:
  - Build: `cd backend && go build -o ../app`
  - Run the binary with env vars set (or an env file consumed by your process manager): `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASS`, `DB_NAME`, `ENVIRONMENT=prod`, plus the `ESI_*` OAuth vars.
  - An example systemd unit is in [deploy/systemd/brave-bpc-backend.service](deploy/systemd/brave-bpc-backend.service).
- Frontend:
  - Build: `cd frontend && npm ci && npm run build`
  - Serve `frontend/dist` via nginx (or equivalent) and proxy the backend routes.
  - An example nginx site config is in [deploy/nginx/brave-bpc.conf](deploy/nginx/brave-bpc.conf).

### Known Bugs
- Issue when adding more scopes to an existing character/token

### TODO
- [ ] Backend
  - [x] Oauth for ESI
    - [x] Corp Blueprint roles to fetch blueprints store in token table
  - [x] Pull Blueprint data via ESI
- [ ] Pages
  - [ ] Unauthenticated / Login Page
  - [ ] Unauthorized Character
  - [ ] Authorized Character
    - [ ] User Settings P2
    - [ ] User Requests
    - [ ] Home Page
  - [ ] Authorized Manager Page
    - [ ] Request Queue
    - [ ] Request History P2
    - [ ] Research Queue
  - [ ] Authorized Admin Page
    - [ ] System Settings

