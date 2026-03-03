-- Octo Web Platform — Supabase Database Schema
-- Run this in the Supabase SQL Editor (Dashboard → SQL Editor → New Query)

-- ============================================================
-- 1. PROJECTS TABLE
-- ============================================================
CREATE TABLE IF NOT EXISTS public.projects (
    id          UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    name        TEXT NOT NULL,
    repo_url    TEXT DEFAULT '',
    description TEXT DEFAULT '',
    language    TEXT DEFAULT '',
    is_public   BOOLEAN DEFAULT true,
    stars       INTEGER DEFAULT 0,
    user_id     UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    user_name   TEXT DEFAULT '',
    avatar_url  TEXT DEFAULT '',
    blueprint   JSONB DEFAULT NULL,
    created_at  TIMESTAMPTZ DEFAULT now(),
    updated_at  TIMESTAMPTZ DEFAULT now()
);

-- Index for fast queries
CREATE INDEX IF NOT EXISTS idx_projects_user_id ON public.projects(user_id);
CREATE INDEX IF NOT EXISTS idx_projects_is_public ON public.projects(is_public);
CREATE INDEX IF NOT EXISTS idx_projects_language ON public.projects(language);
CREATE INDEX IF NOT EXISTS idx_projects_updated_at ON public.projects(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_projects_name_search ON public.projects USING gin(to_tsvector('english', name));

-- Unique constraint for upsert (one config per repo per user)
CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_user_repo ON public.projects(user_id, repo_url) WHERE repo_url != '';

-- ============================================================
-- 2. ACTIVITY LOG TABLE (tracks analyze/publish events)
-- ============================================================
CREATE TABLE IF NOT EXISTS public.activity_log (
    id          UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    project_id  UUID REFERENCES public.projects(id) ON DELETE SET NULL,
    action      TEXT NOT NULL, -- 'analyze', 'publish', 'update', 'delete'
    metadata    JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_activity_user_id ON public.activity_log(user_id);
CREATE INDEX IF NOT EXISTS idx_activity_created_at ON public.activity_log(created_at DESC);

-- ============================================================
-- 3. ROW LEVEL SECURITY (RLS)
-- ============================================================
ALTER TABLE public.projects ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.activity_log ENABLE ROW LEVEL SECURITY;

-- Projects: Users can CRUD their own projects
CREATE POLICY "Users can view own projects"
    ON public.projects FOR SELECT
    USING (auth.uid() = user_id);

CREATE POLICY "Users can insert own projects"
    ON public.projects FOR INSERT
    WITH CHECK (auth.uid() = user_id);

CREATE POLICY "Users can update own projects"
    ON public.projects FOR UPDATE
    USING (auth.uid() = user_id);

CREATE POLICY "Users can delete own projects"
    ON public.projects FOR DELETE
    USING (auth.uid() = user_id);

-- Projects: Anyone can view public projects
CREATE POLICY "Public projects are viewable by all"
    ON public.projects FOR SELECT
    USING (is_public = true);

-- Activity log: Users can view own activity
CREATE POLICY "Users can view own activity"
    ON public.activity_log FOR SELECT
    USING (auth.uid() = user_id);

CREATE POLICY "Users can insert own activity"
    ON public.activity_log FOR INSERT
    WITH CHECK (auth.uid() = user_id);

-- ============================================================
-- 4. AUTO-UPDATE TIMESTAMP TRIGGER
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

-- ============================================================
-- 5. AUTO-POPULATE USER INFO TRIGGER
-- ============================================================
CREATE OR REPLACE FUNCTION public.populate_user_info()
RETURNS TRIGGER AS $$
BEGIN
    -- Pull name and avatar from auth.users metadata
    NEW.user_name := COALESCE(
        (SELECT raw_user_meta_data->>'full_name' FROM auth.users WHERE id = NEW.user_id),
        (SELECT raw_user_meta_data->>'name' FROM auth.users WHERE id = NEW.user_id),
        ''
    );
    NEW.avatar_url := COALESCE(
        (SELECT raw_user_meta_data->>'avatar_url' FROM auth.users WHERE id = NEW.user_id),
        ''
    );
    -- Extract language from blueprint if available
    IF NEW.blueprint IS NOT NULL AND NEW.language = '' THEN
        NEW.language := COALESCE(NEW.blueprint->>'language', '');
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

CREATE TRIGGER projects_populate_user
    BEFORE INSERT ON public.projects
    FOR EACH ROW
    EXECUTE FUNCTION public.populate_user_info();
