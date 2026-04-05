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

    const response = await fetch(url, { ...options, headers, credentials: "include" });

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

  // ─── Projects (Neon DB via Go backend) ───
  async listProjects() {
    return this.request("/api/projects");
  }

  async getProject(id) {
    return this.request(`/api/projects/${id}`);
  }

  async createProject(projectData) {
    return this.request("/api/projects", {
      method: "POST",
      body: JSON.stringify(projectData),
    });
  }

  async updateProject(id, updates) {
    return this.request(`/api/projects/${id}`, {
      method: "PUT",
      body: JSON.stringify(updates),
    });
  }

  async deleteProject(id) {
    return this.request(`/api/projects/${id}`, {
      method: "DELETE",
    }).then(() => undefined);
  }

  // ─── Blueprints (Neon DB via Go backend) ───
  async getBlueprint(projectId) {
    const blueprint = await this.request(`/api/projects/${projectId}/blueprint`);
    return { blueprint };
  }

  async updateBlueprint(projectId, blueprint) {
    return this.request(`/api/projects/${projectId}/blueprint`, {
      method: "PUT",
      body: JSON.stringify({ blueprint }),
    });
  }

  // ─── Explore (Neon DB via Go backend) ───
  async explore({ language, search, sort } = {}) {
    const params = new URLSearchParams();
    if (language) params.append("language", language);
    if (search) params.append("search", search);
    if (sort) params.append("sort", sort);
    
    return this.request(`/api/explore?${params.toString()}`);
  }

  async getExploreProject(id) {
    return this.request(`/api/explore/${id}`);
  }

  // ─── GitHub (via Go backend) ───
  async listGitHubRepos() {
    return this.request("/api/github/repos");
  }

  async getGitHubRepoInfo(owner, repo) {
    return this.request(`/api/github/repo/${owner}/${repo}`);
  }

  // ─── Env Variables (Neon DB via Go Backend) ───
  async getProjectEnvVars(projectId) {
    // The backend doesn't seem to have a dedicated endpoint yet.
    // If it's stored inside blueprint, return from there, else mock.
    try {
      const { blueprint } = await this.getBlueprint(projectId);
      return blueprint.env_vars || [];
    } catch {
      return [];
    }
  }

  async saveProjectEnvVars(projectId, envVars) {
    // Mock for now or update blueprint env vars
    try {
      const { blueprint } = await this.getBlueprint(projectId);
      blueprint.env_vars = envVars;
      await this.updateBlueprint(projectId, blueprint);
      return envVars;
    } catch {
      return envVars;
    }
  }

  // ─── User Tokens (Mocked, handled via session for now) ───
  async upsertUserToken(provider, accessToken, scopes = "") {
    return { provider, access_token: accessToken, scopes };
  }

  async getUserToken(provider) {
    return null;
  }

  getProviderToken() {
    return (
      this.providerToken || localStorage.getItem("gh_provider_token") || null
    );
  }
}

export const api = new ApiClient();
export default api;
