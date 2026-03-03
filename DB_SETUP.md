# Octo Web Platform — Database Setup Guide

This guide walks you through setting up the **Supabase** database for the Octo web dashboard.

---

## 1. Create a Supabase Project

1. Go to [supabase.com](https://supabase.com) and sign in (or create an account).
2. Click **New Project**.
3. Fill in:
   - **Name**: `octo-platform` (or any name)
   - **Database Password**: Choose a strong password (save it)
   - **Region**: Pick one close to your users
4. Click **Create new project** and wait for provisioning (~2 min).

---

## 2. Get Your API Keys

Once the project is ready:

1. Go to **Settings → API** in the Supabase dashboard.
2. Note down these values:

| Key                  | Where to find                                             | Used by                        |
| -------------------- | --------------------------------------------------------- | ------------------------------ |
| **Project URL**      | `Settings → API → Project URL`                            | Both frontend & backend        |
| **anon/public key**  | `Settings → API → Project API keys → anon public`         | Frontend (.env)                |
| **service_role key** | `Settings → API → Project API keys → service_role secret` | Backend only (⚠️ keep secret!) |

---

## 3. Run the Database Migration

1. Go to **SQL Editor** in the Supabase dashboard.
2. Click **New Query**.
3. Copy the entire contents of [`web/supabase/migrations/001_initial_schema.sql`](./web/supabase/migrations/001_initial_schema.sql).
4. Paste it into the SQL editor and click **Run**.

This creates:

- `projects` table — stores project configs and blueprints
- `activity_log` table — tracks user actions
- Row Level Security (RLS) policies
- Auto-update timestamp triggers
- User info auto-population triggers

---

## 4. Enable GitHub Authentication

1. Go to **Authentication → Providers** in Supabase.
2. Find **GitHub** and enable it.
3. You'll need a GitHub OAuth App:
   - Go to [GitHub Developer Settings → OAuth Apps → New](https://github.com/settings/developers)
   - **Application name**: `Octo Platform`
   - **Homepage URL**: `http://localhost:5173` (dev) or your production URL
   - **Authorization callback URL**: `https://<your-project-ref>.supabase.co/auth/v1/callback`
4. Copy the **Client ID** and **Client Secret** from GitHub.
5. Paste them into the Supabase GitHub provider settings.
6. Click **Save**.

---

## 5. Configure Environment Variables

### Frontend (`web/.env`)

Create `web/.env` from the example:

```bash
cp web/.env.example web/.env
```

Fill in:

```env
VITE_SUPABASE_URL=https://<your-project-ref>.supabase.co
VITE_SUPABASE_ANON_KEY=eyJhbGciOi...your-anon-key
VITE_API_URL=
```

### Backend (shell environment)

Set these before running `octo serve`:

```bash
export SUPABASE_URL=https://<your-project-ref>.supabase.co
export SUPABASE_ANON_KEY=eyJhbGciOi...your-anon-key
export SUPABASE_SERVICE_KEY=eyJhbGciOi...your-service-role-key
export GITHUB_TOKEN=ghp_...optional-for-github-api
export OCTO_PORT=8080
```

Or create a `.env` file in the project root:

```bash
# .env (project root — for the Go server)
SUPABASE_URL=https://your-project.supabase.co
SUPABASE_ANON_KEY=your-anon-key
SUPABASE_SERVICE_KEY=your-service-role-key
GITHUB_TOKEN=your-github-pat
OCTO_PORT=8080
```

---

## 6. Verify the Setup

### Check tables exist

Go to **Table Editor** in Supabase — you should see:

- `projects` (with columns: id, name, repo_url, description, language, is_public, stars, user_id, user_name, avatar_url, blueprint, created_at, updated_at)
- `activity_log` (with columns: id, user_id, project_id, action, metadata, created_at)

### Check RLS policies

Go to **Authentication → Policies** and verify:

- `projects` has 5 policies (SELECT own, INSERT own, UPDATE own, DELETE own, SELECT public)
- `activity_log` has 2 policies (SELECT own, INSERT own)

### Test authentication

1. Start the web dashboard: `cd web && npm run dev`
2. Click "Sign in with GitHub"
3. You should be redirected to GitHub and back to `/dashboard`

### Test the API

```bash
# Health check
curl http://localhost:8080/api/health

# Analyze a repo (requires auth)
curl -X POST http://localhost:8080/api/analyze/github \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_SUPABASE_TOKEN" \
  -d '{"repo_url": "https://github.com/expressjs/express.git"}'
```

---

## 7. Database Schema Reference

### `projects` Table

| Column        | Type        | Description                                  |
| ------------- | ----------- | -------------------------------------------- |
| `id`          | UUID        | Primary key (auto-generated)                 |
| `name`        | TEXT        | Project name                                 |
| `repo_url`    | TEXT        | GitHub repository URL                        |
| `description` | TEXT        | Project description                          |
| `language`    | TEXT        | Detected language (Node, Python, Go, etc.)   |
| `is_public`   | BOOLEAN     | Whether the project is publicly discoverable |
| `stars`       | INTEGER     | Community stars/votes                        |
| `user_id`     | UUID        | Owner's auth user ID                         |
| `user_name`   | TEXT        | Auto-populated from GitHub profile           |
| `avatar_url`  | TEXT        | Auto-populated from GitHub avatar            |
| `blueprint`   | JSONB       | The .octo.yaml content as JSON               |
| `created_at`  | TIMESTAMPTZ | Creation timestamp                           |
| `updated_at`  | TIMESTAMPTZ | Last update (auto-maintained)                |

### `blueprint` JSONB Structure

```json
{
  "name": "my-project",
  "language": "Node",
  "version": "20.x",
  "run": "pnpm run dev",
  "setup": "pnpm install",
  "setup_required": true,
  "package_manager": "pnpm",
  "is_monorepo": false,
  "env_vars": [
    { "name": "DATABASE_URL", "required": true },
    { "name": "API_KEY", "required": false }
  ],
  "thermal": {
    "concurrency": 4,
    "batch_size": 3,
    "cool_down_ms": 500,
    "mode": "auto"
  }
}
```

---

## Troubleshooting

| Issue                       | Fix                                                               |
| --------------------------- | ----------------------------------------------------------------- |
| "Invalid API key"           | Double-check `SUPABASE_ANON_KEY` — it should start with `eyJ`     |
| GitHub OAuth redirect fails | Verify callback URL: `https://<ref>.supabase.co/auth/v1/callback` |
| RLS blocks all queries      | Ensure you're passing the `Authorization: Bearer <token>` header  |
| Projects not showing        | Check RLS policies are created (step 6)                           |
| Service key errors          | The `service_role` key bypasses RLS — use only on the backend     |

---

## Production Deployment

For production, additionally:

1. **Set Site URL** in Supabase: `Authentication → URL Configuration → Site URL` → your production domain
2. **Add redirect URLs**: Add your production callback URL to the allowed list
3. **Enable email confirmations** if desired
4. **Set up database backups** via Supabase Pro plan
5. **Use environment-specific keys** — never expose `service_role` key to the frontend
