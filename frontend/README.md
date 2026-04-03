# Brave BPC Frontend

This package contains the React SPA for Brave BPC. It is built with Vite, TanStack Router, React Query, HeroUI, and TypeScript.

The frontend does not implement authentication or business logic on its own. It relies on the Go backend for:

- OAuth entry points and callback handling
- Session and cookie management
- Authenticated JSON APIs under `/api/*`
- Logout and session refresh endpoints

For the broader project overview and backend OAuth app setup, see the root [README](../README.md).

## Architecture

The frontend is a cookie-authenticated SPA:

- On app startup, `AuthProvider` calls `/session` with `credentials: "include"` to determine whether the user has an active session.
- Protected routes live under the `/_auth` route group. If a user is not authenticated, the route guard sends them back to `/` rather than starting OAuth automatically.
- The visible login action is an explicit link to `/login`, which is handled by the backend.
- Data fetching uses React Query and talks to backend endpoints such as `/api/blueprints` and `/api/requisition`.

In development, Vite proxies frontend requests to the backend so the browser can stay on the frontend origin while still using cookie auth.

## Backend Interaction

The frontend expects the backend to provide these routes:

- `/session` to return the current authenticated user or a non-success response when logged out
- `/logout` to clear the server-side session and redirect to `/`
- `/login`, `/login/char`, and `/login/scope` for OAuth-related flows
- `/api/*` for application data and mutations

Current frontend/backend interaction conventions:

- Authenticated fetches must use `credentials: "include"` so the session cookie is sent.
- API calls are written against relative paths such as `/api/blueprints` and `/api/requisition`.
- The frontend assumes the backend is the source of truth for auth state.
- The frontend does not auto-start OAuth from protected routes. Users are redirected to the home page, where the explicit login CTA can begin the `/login` flow.

## Development Setup

### Prerequisites

- Docker and Docker Compose for the recommended workflow
- Node.js and npm if you want to run the frontend outside Docker
- A backend configured for EVE OAuth, as described in the root [README](../README.md)

### Recommended Local Workflow

Start the backend and database first:

```sh
docker compose up -d --build backend
```

That gives you:

- MariaDB on `localhost:3308`
- Backend on `http://localhost:2727`

Then start the frontend either with Docker Compose:

```sh
docker compose up -d frontend
```

Or locally with Node.js:

```sh
cd frontend
npm ci --legacy-peer-deps
npm run dev
```

Default frontend URLs:

- `npm run dev` listens on `http://localhost:3000` and binds to the network with `--host`
- `npm run dev-local` listens on localhost only

### Frontend Configuration

The frontend currently uses these Vite-time environment variables:

- `VITE_BACKEND_ORIGIN`
  - Default: `http://localhost:2727`
  - Used by the Vite dev proxy to forward backend-bound requests in local development
  - In Docker Compose, the frontend service uses `http://backend:2727`
- `VITE_ALLOWED_HOSTS`
  - Optional comma-separated list of additional hosts to allow in the Vite dev server
  - Useful when testing through external domains or tunnels
  - The config already allows localhost, loopback, and common ngrok domains

The frontend does not read the backend OAuth secrets directly. Those live in `backend/.env` and are required if you need login to work during development. In practice, frontend login testing depends on the backend being configured with:

- `ESI_APP_ID`
- `ESI_APP_SECRET`
- `ESI_APP_REDIRECT`

If the backend callback URL is wrong for your current host or tunnel, the frontend login button will still render, but the OAuth flow will fail or return to the wrong place.

## Dev Proxy Behavior

When the Vite dev server is running, these frontend-visible paths are proxied to `VITE_BACKEND_ORIGIN`:

- `/api`
- `/session`
- `/logout`
- `/login`
- `/login/char`
- `/login/scope`

This proxy setup is what allows the frontend to use relative URLs and cookie auth in development without hardcoding backend origins in the browser code.

## Scripts

Available scripts from this package:

- `npm run dev`
  - Start the Vite dev server on port 3000 with network exposure
- `npm run dev-local`
  - Start the Vite dev server without `--host`
- `npm run build`
  - Run TypeScript build checks and produce a production bundle in `dist/`
- `npm run lint`
  - Run ESLint
- `npm run test`
  - Run Vitest once
- `npm run test:watch`
  - Run Vitest in watch mode
- `npm run test:e2e`
  - Run Playwright end-to-end tests
- `npm run test:e2e:headed`
  - Run Playwright in headed mode
- `npm run test:e2e:ui`
  - Run Playwright with the interactive UI
- `npm run preview`
  - Preview the built app locally

## Testing

Unit and component tests use Vitest and JSDOM:

```sh
npm run test
```

Browser tests use Playwright. The Playwright config starts the frontend on port 4173 with `npm run dev-local -- --port 4173 --strictPort`:

```sh
npm run test:e2e
```

If your tests depend on backend behavior beyond mocked routes, make sure the backend is already running.

## Production Deployment

Current production expectations:

- `npm run build` outputs static assets into `frontend/dist/`
- The frontend should be hosted behind the same origin as the backend, or behind a reverse proxy that makes the backend routes available on the same browser-visible origin
- The proxy or hosting layer must route these backend paths correctly:
  - `/api/*`
  - `/session`
  - `/logout`
  - `/login`
  - `/login/char`
  - `/login/scope`

Important constraints:

- Backend CORS support is dev-only and only allows `http://localhost:3000` when `ENVIRONMENT=dev`
- Production should not rely on general cross-origin browser access between frontend and backend
- The frontend currently assumes cookie/session auth, so same-origin or reverse-proxied routing is the safe deployment model

## Troubleshooting

### Backend requests fail in development

Check that the backend is running and that `VITE_BACKEND_ORIGIN` points at it. For local Node-based development, the default backend origin is `http://localhost:2727`.

### Login button works, but OAuth fails

This is usually a backend callback configuration issue. Verify the backend OAuth settings in `backend/.env`, especially `ESI_APP_REDIRECT`, and make sure the EVE developer application callback URL matches the actual host you are testing.

### External dev host is blocked by Vite

Add the host to `VITE_ALLOWED_HOSTS` if it is not already covered by the built-in allowlist. This is mainly relevant for external tunnels or custom test domains.

### Session-dependent API requests fail unexpectedly

Make sure the request includes `credentials: "include"` and that the backend is setting and accepting the session cookie on the same browser-visible origin.
