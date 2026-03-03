import { supabase } from "../lib/supabase.js";

const API_BASE = import.meta.env.VITE_API_URL || "";

class ApiClient {
  constructor() {
    this.baseUrl = API_BASE;
    this.token = null;
    this.providerToken = null; // GitHub OAuth token
  }

  setToken(token) {
    this.token = token;
  }

  setProviderToken(token) {
    this.providerToken = token;
  }

  // ─── Go backend request (used ONLY for analysis) ───
  async request(endpoint, options = {}) {
    const url = `${this.baseUrl}${endpoint}`;
    const headers = {
      "Content-Type": "application/json",
      ...options.headers,
    };

    if (this.token) {
      headers["Authorization"] = `Bearer ${this.token}`;
    }

    const response = await fetch(url, { ...options, headers });

    if (!response.ok) {
      const error = await response
        .json()
        .catch(() => ({ error: "Request failed" }));
      throw new Error(error.error || `HTTP ${response.status}`);
    }

    return response.json();
  }

  // ─── Health check (Go backend) ───
  async health() {
    return this.request("/api/health");
  }

  // ─── Analysis (Go backend – requires `octo serve`) ───
  async analyzePath(path) {
    return this.request("/api/analyze", {
      method: "POST",
      body: JSON.stringify({ path }),
    });
  }

  async analyzeGitHub(repoUrl, branch = "", userToken = "") {
    const body = { repo_url: repoUrl, branch };
    if (userToken) body.user_token = userToken;

    const raw = await this.request("/api/analyze/github", {
      method: "POST",
      body: JSON.stringify(body),
    });
    return this._normalizeAnalysis(raw);
  }

  _normalizeAnalysis(raw) {
    const bp = raw.blueprint || raw.Blueprint || {};
    const rawSteps = bp.Steps || bp.steps || [];
    return {
      success: raw.success ?? raw.Success,
      duration: raw.duration ?? raw.Duration ?? "",
      project: raw.project || raw.Project || {},
      blueprint: {
        name: bp.Name || bp.name || "",
        language: bp.Language || bp.language || "",
        version: bp.Version || bp.version || "",
        run: bp.RunCommand || bp.run || "",
        setup: bp.SetupCommand || bp.setup || "",
        setup_required: bp.SetupRequired ?? bp.setup_required ?? false,
        package_manager: bp.PackageManager || bp.package_manager || "",
        is_monorepo: bp.IsMonorepo ?? bp.is_monorepo ?? false,
        monorepo_root: bp.MonorepoRoot || bp.monorepo_root || "",
        env_vars: bp.EnvVars || bp.env_vars || [],
        thermal: bp.Thermal || bp.thermal || {},
        steps: rawSteps.map((s) => ({
          id: s.ID || s.id || crypto.randomUUID(),
          name: s.Name || s.name || "",
          command: s.Command || s.command || "",
          order: s.Order ?? s.order ?? 0,
        })),
      },
    };
  }

  // ─── Projects (direct Supabase) ───
  async listProjects() {
    const {
      data: { user },
    } = await supabase.auth.getUser();
    if (!user) throw new Error("Not authenticated");

    const { data, error } = await supabase
      .from("projects")
      .select("*")
      .eq("user_id", user.id)
      .order("updated_at", { ascending: false });

    if (error) throw new Error(error.message);
    return { projects: data || [] };
  }

  async getProject(id) {
    const { data, error } = await supabase
      .from("projects")
      .select("*")
      .eq("id", id)
      .single();

    if (error) throw new Error(error.message);
    return data;
  }

  async createProject(projectData) {
    const {
      data: { user },
    } = await supabase.auth.getUser();
    if (!user) throw new Error("Not authenticated");

    const { data, error } = await supabase
      .from("projects")
      .insert({
        user_id: user.id,
        name: projectData.name,
        repo_url: projectData.repo_url || "",
        description: projectData.description || "",
        language: projectData.blueprint?.language || "",
        is_public: projectData.is_public || false,
        blueprint: projectData.blueprint || {},
      })
      .select()
      .single();

    if (error) throw new Error(error.message);
    return data;
  }

  async updateProject(id, updates) {
    const { data, error } = await supabase
      .from("projects")
      .update(updates)
      .eq("id", id)
      .select()
      .single();

    if (error) throw new Error(error.message);
    return data;
  }

  async deleteProject(id) {
    const { error } = await supabase.from("projects").delete().eq("id", id);

    if (error) throw new Error(error.message);
  }

  // ─── Blueprints (direct Supabase) ───
  async getBlueprint(projectId) {
    const { data, error } = await supabase
      .from("projects")
      .select("blueprint")
      .eq("id", projectId)
      .single();

    if (error) throw new Error(error.message);
    return { blueprint: data?.blueprint || {} };
  }

  async updateBlueprint(projectId, blueprint) {
    const { data, error } = await supabase
      .from("projects")
      .update({ blueprint })
      .eq("id", projectId)
      .select()
      .single();

    if (error) throw new Error(error.message);
    return data;
  }

  // ─── Explore (direct Supabase – public projects) ───
  async explore({ language, search, sort } = {}) {
    let query = supabase.from("projects").select("*").eq("is_public", true);

    if (language) {
      query = query.eq("language", language);
    }
    if (search) {
      query = query.or(`name.ilike.%${search}%,description.ilike.%${search}%`);
    }
    if (sort === "stars") {
      query = query.order("stars", { ascending: false });
    } else {
      query = query.order("updated_at", { ascending: false });
    }

    const { data, error } = await query;
    if (error) throw new Error(error.message);
    return { projects: data || [] };
  }

  async getExploreProject(id) {
    const { data, error } = await supabase
      .from("projects")
      .select("*")
      .eq("id", id)
      .eq("is_public", true)
      .single();

    if (error) throw new Error(error.message);
    return data;
  }

  // ─── GitHub (direct GitHub API using OAuth provider token) ───
  async listGitHubRepos() {
    let token = this.providerToken;

    if (!token) {
      // Try session first
      const {
        data: { session },
      } = await supabase.auth.getSession();
      if (session?.provider_token) {
        this.providerToken = session.provider_token;
        token = session.provider_token;
      }
    }

    if (!token) {
      // Try localStorage fallback
      token = localStorage.getItem("gh_provider_token");
      if (token) this.providerToken = token;
    }

    if (!token) {
      throw new Error(
        "No GitHub token available. Please sign out and sign in again.",
      );
    }

    return this._fetchGitHubRepos(token);
  }

  async _fetchGitHubRepos(token) {
    const response = await fetch(
      "https://api.github.com/user/repos?per_page=100&sort=updated&type=owner",
      {
        headers: {
          Authorization: `Bearer ${token}`,
          Accept: "application/vnd.github.v3+json",
        },
      },
    );

    if (!response.ok) {
      throw new Error(`GitHub API error: ${response.status}`);
    }

    const repos = await response.json();
    return {
      repos: repos.map((r) => ({
        id: r.id,
        name: r.name,
        full_name: r.full_name,
        description: r.description,
        clone_url: r.clone_url,
        html_url: r.html_url,
        language: r.language,
        stargazers_count: r.stargazers_count,
        default_branch: r.default_branch,
        private: r.private,
        updated_at: r.updated_at,
      })),
    };
  }

  async getGitHubRepoInfo(owner, repo) {
    const token = this.providerToken;
    const headers = {
      Accept: "application/vnd.github.v3+json",
    };
    if (token) headers["Authorization"] = `Bearer ${token}`;

    const response = await fetch(
      `https://api.github.com/repos/${owner}/${repo}`,
      { headers },
    );

    if (!response.ok) {
      throw new Error(`GitHub API error: ${response.status}`);
    }

    return response.json();
  }

  // ─── Env Variables (direct Supabase) ───
  async getProjectEnvVars(projectId) {
    const { data, error } = await supabase
      .from("project_env_vars")
      .select("*")
      .eq("project_id", projectId)
      .order("created_at", { ascending: true });

    if (error) throw new Error(error.message);
    return data || [];
  }

  async saveProjectEnvVars(projectId, envVars) {
    // Delete existing env vars for this project
    await supabase
      .from("project_env_vars")
      .delete()
      .eq("project_id", projectId);

    if (!envVars || envVars.length === 0) return [];

    const rows = envVars.map((v) => ({
      project_id: projectId,
      key: v.key,
      value: v.value,
      is_secret: v.isSecret ?? v.is_secret ?? false,
      required: v.required ?? false,
    }));

    const { data, error } = await supabase
      .from("project_env_vars")
      .insert(rows)
      .select();

    if (error) throw new Error(error.message);
    return data;
  }

  // ─── User Tokens (direct Supabase) ───
  async upsertUserToken(provider, accessToken, scopes = "") {
    const {
      data: { user },
    } = await supabase.auth.getUser();
    if (!user) throw new Error("Not authenticated");

    const { data, error } = await supabase
      .from("user_tokens")
      .upsert(
        {
          user_id: user.id,
          provider,
          access_token: accessToken,
          scopes,
        },
        { onConflict: "user_id,provider" },
      )
      .select()
      .single();

    if (error) throw new Error(error.message);
    return data;
  }

  async getUserToken(provider) {
    const {
      data: { user },
    } = await supabase.auth.getUser();
    if (!user) return null;

    const { data } = await supabase
      .from("user_tokens")
      .select("access_token, scopes")
      .eq("user_id", user.id)
      .eq("provider", provider)
      .single();

    return data?.access_token || null;
  }

  getProviderToken() {
    return (
      this.providerToken || localStorage.getItem("gh_provider_token") || null
    );
  }
}

export const api = new ApiClient();
export default api;
