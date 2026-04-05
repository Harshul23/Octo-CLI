# Octo Web Platform — Neon Database Setup Guide

This guide walks you through setting up **Neon PostgreSQL** for the Octo web dashboard.

---

## 1. Prerequisites

✅ You've already created a project called **'Octo'** in Neon
✅ You have the database connection string

---

## 2. Run the Database Migration

1. Go to your Neon Console: https://console.neon.tech
2. Select your **Octo** project
3. Click **SQL Editor** in the sidebar
4. Copy the entire contents of `neon_migration.sql` from the project root
5. Paste it into the SQL editor and click **Run** (or press `Cmd+Enter`)

This creates:
- `users` table — stores GitHub user profiles and tokens
- `projects` table — stores project configs and blueprints
- `activity_log` table — tracks user actions
- `project_env_vars` table — stores environment variables for projects
- `sessions` table — manages user authentication sessions
- Auto-update timestamp triggers
- User info auto-population triggers

**Verify:** After running, you should see 5 tables in the **Tables** section.

---

## 3. Get Your Neon Database URL

1. In the Neon Console, go to your **Octo** project
2. Click **Dashboard** in the sidebar
3. Copy the **Connection String** (it should look like):
   ```
   postgresql://[user]:[password]@[host]/[database]?sslmode=require
   ```
4. Keep this handy — you'll need it for environment variables

---

## 4. Create a GitHub OAuth App

Octo uses GitHub for authentication.

### Step 1: Create the OAuth App

1. Go to GitHub: https://github.com/settings/developers
2. Click **OAuth Apps** → **New OAuth App**
3. Fill in:
   - **Application name**: `Octo Platform` (or any name you prefer)
   - **Homepage URL**: `http://localhost:5173` (for development)
   - **Authorization callback URL**: `http://localhost:8080/api/auth/github/callback`
4. Click **Register application**

### Step 2: Generate a Client Secret

1. After creating the app, you'll see the **Client ID** — copy it
2. Click **Generate a new client secret**
3. Copy the **Client Secret** immediately (you won't see it again!)

### Step 3: Save Your Credentials

You now have:
- ✅ **Client ID** (looks like: `Iv1.a1b2c3d4e5f6g7h8`)
- ✅ **Client Secret** (looks like: `1234567890abcdef1234567890abcdef12345678`)

---

## 5. Configure Environment Variables

### Backend (.env in project root)

Create a `.env` file in the **project root** (not in `web/`):

```bash
# Neon Database
DATABASE_URL=postgresql://[user]:[password]@[host]/[database]?sslmode=require

# GitHub OAuth
GITHUB_CLIENT_ID=Iv1.your_client_id_here
GITHUB_CLIENT_SECRET=your_client_secret_here
GITHUB_CALLBACK_URL=http://localhost:8080/api/auth/github/callback

# GitHub API (optional - for enhanced GitHub features)
GITHUB_TOKEN=ghp_your_personal_access_token_here

# Server Config
OCTO_PORT=8080
OCTO_ALLOWED_ORIGINS=http://localhost:5173,http://localhost:3000

# Session Secret (generate a random string)
SESSION_SECRET=your_random_secret_here_min_32_chars
```

**Generate a SESSION_SECRET:**
```bash
openssl rand -base64 32
```

### Frontend (web/.env)

Update your `web/.env` file:

```env
# API URL (leave empty for same-origin in dev, or set for production)
VITE_API_URL=http://localhost:8080

# GitHub OAuth (same as backend)
VITE_GITHUB_CLIENT_ID=Iv1.your_client_id_here
```

**Note:** Remove the old `VITE_NEON_AUTH_URL` — we're handling auth via the backend now.

---

## 6. Update Backend Code (I'll do this for you)

The backend needs to be updated to:
- ✅ Connect to Neon PostgreSQL (instead of Supabase)
- ✅ Implement GitHub OAuth flow
- ✅ Manage user sessions

I'll create the necessary code changes to support Neon + GitHub OAuth.

---

## 7. Start the Servers

### Terminal 1: Start Backend
```bash
# Load .env and start the server
source .env  # Or: export $(cat .env | xargs)
go run cmd/*.go serve
```

### Terminal 2: Start Frontend
```bash
cd web
npm run dev
```

**Expected output:**
```
Backend:  🐙 Octo API server starting on http://localhost:8080
Frontend: ➜  Local:   http://localhost:5173/
```

---

## 8. Verify the Setup

### Test 1: Health Check
```bash
curl http://localhost:8080/api/health
# Expected: {"status":"ok","version":"0.1.0","time":"..."}
```

### Test 2: Database Connection
Open the backend logs — you should NOT see any database connection errors.

### Test 3: Sign In with GitHub
1. Open http://localhost:5173 in your browser
2. Click **Sign in with GitHub**
3. Authorize the app
4. You should be redirected to the dashboard

---

## 9. Production Deployment

For production:

1. **Update GitHub OAuth App:**
   - Add production URLs to callback URLs
   - Example: `https://octo.yourdomain.com/api/auth/github/callback`

2. **Update Environment Variables:**
   - Use production database URL
   - Use production callback URL
   - Set strong `SESSION_SECRET`

3. **Enable Connection Pooling:**
   - Neon recommends using connection pooling in production
   - Use the pooled connection string from Neon Dashboard

4. **Set Up SSL:**
   - Ensure `sslmode=require` in DATABASE_URL

---

## Troubleshooting

| Issue | Fix |
|-------|-----|
| "Connection refused" to Neon | Check DATABASE_URL format and network connectivity |
| "Invalid client" during OAuth | Verify GITHUB_CLIENT_ID matches your OAuth app |
| "Redirect URI mismatch" | Ensure callback URL matches exactly in GitHub settings |
| "Sessions not persisting" | Check SESSION_SECRET is set and at least 32 chars |
| "CORS errors" in browser | Verify OCTO_ALLOWED_ORIGINS includes your frontend URL |

---

## Manual Tasks Checklist

- [ ] Run `neon_migration.sql` in Neon SQL Editor
- [ ] Create GitHub OAuth App
- [ ] Copy GitHub Client ID and Secret
- [ ] Create `.env` file in project root with all variables
- [ ] Update `web/.env` with VITE_API_URL and VITE_GITHUB_CLIENT_ID
- [ ] Generate SESSION_SECRET using `openssl rand -base64 32`

Once completed, I'll update the backend code to work with Neon!
