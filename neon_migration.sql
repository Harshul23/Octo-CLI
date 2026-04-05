-- ============================================================
-- Octo Web Platform — Neon Database Schema
-- Run this in your Neon SQL Editor (Console → SQL Editor)
-- ============================================================

-- ============================================================
-- 1. USERS TABLE (replaces Supabase auth.users)
-- ============================================================
CREATE TABLE IF NOT EXISTS public.users (
    id              UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    github_id       TEXT NOT NULL UNIQUE,
    email           TEXT,
    name            TEXT,
    avatar_url      TEXT,
    github_username TEXT,
    access_token    TEXT, -- Encrypted GitHub token
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_users_github_id ON public.users(github_id);
CREATE INDEX IF NOT EXISTS idx_users_email ON public.users(email);

-- ============================================================
-- 2. PROJECTS TABLE
-- ============================================================
CREATE TABLE IF NOT EXISTS public.projects (
    id          UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    name        TEXT NOT NULL,
    repo_url    TEXT DEFAULT '',
    description TEXT DEFAULT '',
    language    TEXT DEFAULT '',
    is_public   BOOLEAN DEFAULT true,
    stars       INTEGER DEFAULT 0,
    user_id     UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    user_name   TEXT DEFAULT '',
    avatar_url  TEXT DEFAULT '',
    blueprint   JSONB DEFAULT NULL,
    created_at  TIMESTAMPTZ DEFAULT now(),
    updated_at  TIMESTAMPTZ DEFAULT now()
);

-- Indexes for fast queries
CREATE INDEX IF NOT EXISTS idx_projects_user_id ON public.projects(user_id);
CREATE INDEX IF NOT EXISTS idx_projects_is_public ON public.projects(is_public);
CREATE INDEX IF NOT EXISTS idx_projects_language ON public.projects(language);
CREATE INDEX IF NOT EXISTS idx_projects_updated_at ON public.projects(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_projects_name_search ON public.projects USING gin(to_tsvector('english', name));

-- Unique constraint for upsert (one config per repo per user)
CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_user_repo ON public.projects(user_id, repo_url) WHERE repo_url != '';

-- ============================================================
-- 3. ACTIVITY LOG TABLE (tracks analyze/publish events)
-- ============================================================
CREATE TABLE IF NOT EXISTS public.activity_log (
    id          UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    project_id  UUID REFERENCES public.projects(id) ON DELETE SET NULL,
    action      TEXT NOT NULL, -- 'analyze', 'publish', 'update', 'delete'
    metadata    JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_activity_user_id ON public.activity_log(user_id);
CREATE INDEX IF NOT EXISTS idx_activity_created_at ON public.activity_log(created_at DESC);

-- ============================================================
-- 4. PROJECT ENV VARS TABLE
-- ============================================================
CREATE TABLE IF NOT EXISTS public.project_env_vars (
    id          UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    project_id  UUID NOT NULL REFERENCES public.projects(id) ON DELETE CASCADE,
    key         TEXT NOT NULL,
    value       TEXT DEFAULT '',
    is_secret   BOOLEAN DEFAULT false,
    required    BOOLEAN DEFAULT true,
    created_at  TIMESTAMPTZ DEFAULT now(),
    updated_at  TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_env_vars_project_id ON public.project_env_vars(project_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_env_vars_project_key ON public.project_env_vars(project_id, key);

-- ============================================================
-- 5. SESSIONS TABLE (for user authentication)
-- ============================================================
CREATE TABLE IF NOT EXISTS public.sessions (
    id          UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    token       TEXT NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON public.sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_token ON public.sessions(token);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON public.sessions(expires_at);

-- ============================================================
-- 6. AUTO-UPDATE TIMESTAMP TRIGGERS
-- ============================================================
CREATE OR REPLACE FUNCTION public.update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER projects_updated_at
    BEFORE UPDATE ON public.projects
    FOR EACH ROW
    EXECUTE FUNCTION public.update_updated_at();

CREATE TRIGGER users_updated_at
    BEFORE UPDATE ON public.users
    FOR EACH ROW
    EXECUTE FUNCTION public.update_updated_at();

CREATE TRIGGER env_vars_updated_at
    BEFORE UPDATE ON public.project_env_vars
    FOR EACH ROW
    EXECUTE FUNCTION public.update_updated_at();

-- ============================================================
-- 7. AUTO-POPULATE USER INFO TRIGGER
-- ============================================================
CREATE OR REPLACE FUNCTION public.populate_user_info()
RETURNS TRIGGER AS $$
BEGIN
    -- Pull name and avatar from users table
    NEW.user_name := COALESCE(
        (SELECT name FROM public.users WHERE id = NEW.user_id),
        ''
    );
    NEW.avatar_url := COALESCE(
        (SELECT avatar_url FROM public.users WHERE id = NEW.user_id),
        ''
    );
    -- Extract language from blueprint if available
    IF NEW.blueprint IS NOT NULL AND NEW.language = '' THEN
        NEW.language := COALESCE(NEW.blueprint->>'language', '');
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER projects_populate_user
    BEFORE INSERT ON public.projects
    FOR EACH ROW
    EXECUTE FUNCTION public.populate_user_info();

-- ============================================================
-- 8. CLEANUP EXPIRED SESSIONS FUNCTION (call periodically)
-- ============================================================
CREATE OR REPLACE FUNCTION public.cleanup_expired_sessions()
RETURNS void AS $$
BEGIN
    DELETE FROM public.sessions WHERE expires_at < now();
END;
$$ LANGUAGE plpgsql;

-- Create a cron job to clean up expired sessions (if Neon supports pg_cron)
-- SELECT cron.schedule('cleanup-sessions', '0 * * * *', 'SELECT public.cleanup_expired_sessions()');

-- ============================================================
-- DONE! ✅
-- ============================================================
-- Next steps:
-- 1. Create a GitHub OAuth App (see NEON_SETUP.md)
-- 2. Configure environment variables
-- 3. Start the backend server
