-- Octo Web Platform — Migration 002: Env Variables & User Tokens
-- Run this in the Supabase SQL Editor

-- ============================================================
-- 1. PROJECT ENV VARS TABLE
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

ALTER TABLE public.project_env_vars ENABLE ROW LEVEL SECURITY;

-- Env vars inherit project access (via project ownership)
CREATE POLICY "Users can view own project env vars"
    ON public.project_env_vars FOR SELECT
    USING (EXISTS (SELECT 1 FROM public.projects WHERE id = project_id AND user_id = auth.uid()));

CREATE POLICY "Users can insert own project env vars"
    ON public.project_env_vars FOR INSERT
    WITH CHECK (EXISTS (SELECT 1 FROM public.projects WHERE id = project_id AND user_id = auth.uid()));

CREATE POLICY "Users can update own project env vars"
    ON public.project_env_vars FOR UPDATE
    USING (EXISTS (SELECT 1 FROM public.projects WHERE id = project_id AND user_id = auth.uid()));

CREATE POLICY "Users can delete own project env vars"
    ON public.project_env_vars FOR DELETE
    USING (EXISTS (SELECT 1 FROM public.projects WHERE id = project_id AND user_id = auth.uid()));

-- Auto-update timestamp
CREATE OR REPLACE FUNCTION update_env_vars_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER env_vars_updated_at
    BEFORE UPDATE ON public.project_env_vars
    FOR EACH ROW EXECUTE FUNCTION update_env_vars_timestamp();

-- ============================================================
-- 2. USER TOKENS TABLE (encrypted GitHub provider tokens)
-- ============================================================
CREATE TABLE IF NOT EXISTS public.user_tokens (
    id             UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id        UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    provider       TEXT NOT NULL DEFAULT 'github',
    access_token   TEXT NOT NULL,
    scopes         TEXT DEFAULT '',
    created_at     TIMESTAMPTZ DEFAULT now(),
    updated_at     TIMESTAMPTZ DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_tokens_user_provider ON public.user_tokens(user_id, provider);

ALTER TABLE public.user_tokens ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Users can view own tokens"
    ON public.user_tokens FOR SELECT
    USING (auth.uid() = user_id);

CREATE POLICY "Users can upsert own tokens"
    ON public.user_tokens FOR INSERT
    WITH CHECK (auth.uid() = user_id);

CREATE POLICY "Users can update own tokens"
    ON public.user_tokens FOR UPDATE
    USING (auth.uid() = user_id);

CREATE POLICY "Users can delete own tokens"
    ON public.user_tokens FOR DELETE
    USING (auth.uid() = user_id);

CREATE TRIGGER user_tokens_updated_at
    BEFORE UPDATE ON public.user_tokens
    FOR EACH ROW EXECUTE FUNCTION update_env_vars_timestamp();
