# TaskFlow API

REST API for TaskFlow, a task management application. Built with **NestJS 11
(TypeScript), TypeORM and PostgreSQL 16**.

Companion frontend (Next.js): **task-manager-web** — deploy it separately and
point its `NEXT_PUBLIC_API_URL` at this API.

## Features

- Task CRUD with title, description, status, priority and due date
- List endpoint with **status filtering, title search, sorting (due date /
  priority / created date) and pagination — all combinable in one query**
- JWT signup/login, bcrypt-hashed passwords, all task routes protected
- Per-user ownership: users only ever see and modify their own tasks
- Admin role (via `ADMIN_EMAILS`) that can view — but not modify — all users' tasks
- File attachments (5 MB cap, whitelisted image/document types)
- Per-task activity log of creations, edits and attachment changes
- Real-time task events over Server-Sent Events
- Input validation with per-field messages and one consistent error envelope
- SQL schema migration runs automatically at startup

## Quick start (Docker)

```bash
docker compose up --build
```

API on http://localhost:8080 — health check at `/healthz`.

## Running locally

Requires Node 22+ and PostgreSQL (or use the compose file for just the DB:
`docker compose up -d db`).

```bash
npm install

DATABASE_URL="postgres://taskuser:taskpass@localhost:5432/taskdb?sslmode=disable" \
JWT_SECRET="dev-secret-change-me" \
ADMIN_EMAILS="admin@example.com" \
npm run start:dev
```

All variables are documented in [.env.example](.env.example); `DATABASE_URL`
and `JWT_SECRET` are required, everything else has defaults.

## Tests

```bash
npm test
```

25 jest tests covering DTO validation, password hashing, JWT round-trips,
ownership isolation, admin scope rules, partial updates and the activity
history. CI ([.github/workflows/ci.yml](.github/workflows/ci.yml)) runs
eslint, the test suite and the build on every push.

## API reference

All endpoints are prefixed with `/api`. Protected routes expect
`Authorization: Bearer <token>`.

| Method | Path | Description |
| --- | --- | --- |
| POST | `/auth/signup` | Create an account → `201 {token, user}` |
| POST | `/auth/login` | Log in → `200 {token, user}` |
| GET | `/auth/me` | Current user |
| POST | `/tasks` | Create a task |
| GET | `/tasks` | List tasks — `status`, `search`, `sortBy` (`created_at`\|`due_date`\|`priority`), `order` (`asc`\|`desc`), `page`, `limit`, `scope=all` (admin) |
| GET | `/tasks/:id` | Fetch one task |
| PATCH | `/tasks/:id` | Partial update (any subset of fields) |
| DELETE | `/tasks/:id` | Delete a task → `204` |
| GET | `/tasks/:id/activity` | Per-task change history |
| GET/POST | `/tasks/:id/attachments` | List / upload (multipart `file`) attachments |
| GET | `/attachments/:id/download` | Download an attachment |
| DELETE | `/attachments/:id` | Remove an attachment |
| GET | `/events?token=…` | Server-Sent Events stream of task changes |

Errors always use one shape:

```json
{ "error": { "code": "validation_error", "message": "Invalid input", "fields": { "title": "title is required" } } }
```

## Deploying to Render

This repo ships a [render.yaml](render.yaml) blueprint that provisions the
API (Docker) plus a free managed PostgreSQL database.

1. Push this repository to GitHub.
2. In Render: **New → Blueprint**, select the repo, accept the plan.
3. After the frontend is live on Vercel, set the `CORS_ORIGIN` env var on the
   `taskflow-api` service to the Vercel URL (e.g. `https://taskflow.vercel.app`).
4. Optionally set `ADMIN_EMAILS` to the address you want to be admin.

The service health-checks `/healthz`; migrations run automatically on boot.

**Free-tier notes:** the instance sleeps after idle periods (first request
takes ~30s to wake) and its disk is ephemeral, so uploaded attachments do not
survive restarts. A paid disk or S3-style object storage fixes that for real
deployments.

## Assumptions and trade-offs

- **JWT in localStorage (frontend)** — keeps the API stateless; mitigated by
  short-ish TTLs and strict CORS.
- **Admin is read-only** on other users' tasks; the API returns 403 on writes.
- **Non-owners get 404, not 403** — other users' task IDs cannot be probed.
- **SSE over WebSockets** — one-directional updates are all the UI needs; the
  token travels as a query parameter because `EventSource` cannot set headers.
- **Due dates stored as midnight UTC** so dates never drift across timezones.
- **Hand-written SQL migration** instead of `synchronize` — explicit and
  reviewable; runs automatically at startup.
- **In-memory SSE hub** — events do not fan out across replicas; a Redis/NATS
  bus would be the production answer.
