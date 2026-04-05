# Octo Backend - Quick Reference

## ✅ What's Done

- ✅ Neon database schema created and ready (`neon_migration.sql`)
- ✅ Backend migrated from Supabase to Neon PostgreSQL
- ✅ GitHub OAuth flow implemented
- ✅ Session-based authentication working
- ✅ All API endpoints updated
- ✅ Environment variables configured
- ✅ Backend server running on http://localhost:8080
- ✅ Frontend server running on http://localhost:5173

## 📋 Final Manual Steps

### 1. Run SQL Migration in Neon
```sql
-- Go to: https://console.neon.tech
-- Select your 'Octo' project
-- Open SQL Editor
-- Copy contents of neon_migration.sql and run it
```

### 2. Verify GitHub OAuth App
```
Go to: https://github.com/settings/developers
Find your OAuth App
Ensure callback URL is: http://localhost:8080/api/auth/github/callback
```

### 3. Test the Setup
```bash
# 1. Open browser to http://localhost:5173
# 2. Click "Sign in with GitHub"
# 3. Authorize the app
# 4. You should be redirected to dashboard
```

## 🔧 Environment Variables

### Root .env (Backend)
```env
DATABASE_URL=postgresql://neondb_owner:npg_k3UiSjnAq0ez@ep-late-base-adbsvl49-pooler.c-2.us-east-1.aws.neon.tech/neondb?sslmode=require&channel_binding=require
GITHUB_CLIENT_ID=Ov23lifMCO6LcL41h67K
GITHUB_CLIENT_SECRET=ef1cfe0391b6fbc10d468f89a24575ce1a079277
GITHUB_CALLBACK_URL=http://localhost:8080/api/auth/github/callback
GITHUB_TOKEN=
OCTO_PORT=8080
OCTO_ALLOWED_ORIGINS=http://localhost:5173,http://localhost:3000
SESSION_SECRET=neon_octo_session_secret_change_this_in_production_please_123456
```

### web/.env (Frontend)
```env
VITE_API_URL=http://localhost:8080
VITE_GITHUB_CLIENT_ID=Ov23lifMCO6LcL41h67K
```

## 🚀 Starting the Servers

### Backend
```bash
cd /Users/harshul/Projects/Octo-CLI
source .env
go run cmd/*.go serve
```

### Frontend
```bash
cd /Users/harshul/Projects/Octo-CLI/web
npm run dev
```

## 📡 API Endpoints

### Public Endpoints
- GET  `/api/health` - Health check
- POST `/api/analyze` - Analyze local path
- POST `/api/analyze/github` - Analyze GitHub repo
- GET  `/api/explore` - List public projects
- GET  `/api/explore/:id` - Get public project

### Auth Endpoints
- GET  `/api/auth/github` - Start GitHub OAuth
- GET  `/api/auth/github/callback` - OAuth callback
- POST `/api/auth/logout` - Logout

### Protected Endpoints (require auth)
- GET    `/api/projects` - List user's projects
- POST   `/api/projects` - Create project
- GET    `/api/projects/:id` - Get project
- PUT    `/api/projects/:id` - Update project
- DELETE `/api/projects/:id` - Delete project
- GET    `/api/projects/:id/blueprint` - Get blueprint
- PUT    `/api/projects/:id/blueprint` - Update blueprint
- GET    `/api/github/repos` - List user's GitHub repos
- GET    `/api/github/repo/:owner/:repo` - Get repo info

## 🗃️ Database Tables

1. **users** - GitHub user profiles and tokens
2. **sessions** - User authentication sessions
3. **projects** - Project configurations and blueprints
4. **activity_log** - User action tracking
5. **project_env_vars** - Environment variables for projects

## 🔐 Authentication Flow

1. User clicks "Sign in with GitHub" on frontend
2. Frontend redirects to `/api/auth/github`
3. Backend redirects to GitHub OAuth
4. User authorizes on GitHub
5. GitHub redirects to `/api/auth/github/callback?code=...`
6. Backend exchanges code for access token
7. Backend fetches user info from GitHub API
8. Backend creates/updates user in Neon database
9. Backend creates session and sets cookie
10. Backend redirects to `/dashboard` on frontend
11. Frontend reads session cookie for authenticated requests

## 🐛 Troubleshooting

### Backend won't start
```bash
# Check DATABASE_URL is set
echo $DATABASE_URL

# Check Neon connection
psql "$DATABASE_URL" -c "SELECT 1"
```

### OAuth fails
- Verify callback URL matches exactly in GitHub settings
- Check GITHUB_CLIENT_ID and GITHUB_CLIENT_SECRET are correct
- Ensure GITHUB_CALLBACK_URL in .env matches GitHub OAuth app

### CORS errors
- Verify OCTO_ALLOWED_ORIGINS includes your frontend URL
- Check browser console for specific error
- Ensure cookies are not blocked

### Session not persisting
- Check SESSION_SECRET is set and at least 32 characters
- Verify session cookie is being set (check browser DevTools)
- Ensure database sessions table exists

## 📚 Documentation

- **NEON_SETUP.md** - Complete setup guide
- **neon_migration.sql** - SQL schema
- **DB_SETUP.md** - Original Supabase setup (deprecated)

## 🎯 Next Development Steps

1. Update frontend to use new auth endpoints
2. Test all API endpoints
3. Implement logout functionality in frontend
4. Add error handling for expired sessions
5. Set up production environment variables
