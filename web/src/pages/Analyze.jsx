import { useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../contexts/AuthContext.jsx";
import { useAnalysisStore, useProjectStore } from "../store/index.js";
import api from "../services/api.js";
import { motion, AnimatePresence } from "framer-motion";
import {
  Github,
  Zap,
  CheckCircle2,
  Loader2,
  GitBranch,
  AlertCircle,
  Save,
} from "lucide-react";
import BlueprintEditor from "../components/BlueprintEditor.jsx";
import AnalysisProgress from "../components/AnalysisProgress.jsx";
import { Button } from "../components/ui/Button.jsx";
import { Card } from "../components/ui/Card.jsx";
import { Input } from "../components/ui/Input.jsx";

export default function Analyze() {
  const navigate = useNavigate();
  const { isAuthenticated, accessToken, providerToken, signInWithGitHub } =
    useAuth();
  const { result, loading, error, step, analyzeGitHub, reset } =
    useAnalysisStore();
  const { createProject } = useProjectStore();

  const [repoUrl, setRepoUrl] = useState("");
  const [branch, setBranch] = useState("");
  const [ghRepos, setGhRepos] = useState([]);
  const [showRepos, setShowRepos] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (isAuthenticated && accessToken) {
      api.setToken(accessToken);
      if (providerToken) api.setProviderToken(providerToken);
      api
        .listGitHubRepos()
        .then((data) => setGhRepos(data.repos || []))
        .catch((err) =>
          console.warn("Could not fetch GitHub repos:", err.message),
        );
    }
  }, [isAuthenticated, accessToken, providerToken]);

  const handleAnalyze = async () => {
    if (!repoUrl.trim()) return;
    const token = api.getProviderToken();
    try {
      await analyzeGitHub(repoUrl.trim(), branch.trim(), token);
    } catch (err) {
      // Error handled by store
    }
  };

  const handleSave = async () => {
    if (!result?.blueprint) return;
    setSaving(true);
    try {
      const project = await createProject({
        name: result.blueprint.name,
        repo_url: repoUrl,
        description: `${result.blueprint.language} project analyzed from GitHub`,
        is_public: true,
        blueprint: result.blueprint,
      });
      reset();
      navigate(`/projects/${project.id}`);
    } catch (err) {
      console.error("Save failed:", err);
    }
    setSaving(false);
  };

  const handleSelectRepo = (repo) => {
    setRepoUrl(repo.clone_url);
    setBranch(repo.default_branch);
    setShowRepos(false);
  };

  const handleReset = () => {
    reset();
    setRepoUrl("");
    setBranch("");
  };

  return (
    <div className="p-6 lg:p-8 max-w-4xl mx-auto">
      <div className="mb-8">
        <h1 className="text-2xl font-bold">Analyze Repository</h1>
        <p className="text-muted-foreground text-sm mt-1">
          Paste a GitHub URL or pick from your repos to generate an Octo
          blueprint.
        </p>
      </div>

      {/* URL input */}
      <Card className="p-6 mb-6">
        <label className="block text-sm font-medium mb-2">Repository URL</label>
        <div className="flex gap-3">
          <div className="flex-1 relative">
            <Github
              size={16}
              className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"
            />
            <Input
              type="text"
              value={repoUrl}
              onChange={(e) => setRepoUrl(e.target.value)}
              placeholder="https://github.com/user/repo.git"
              className="pl-10"
              disabled={loading}
              onKeyDown={(e) => e.key === "Enter" && handleAnalyze()}
            />
          </div>
          <Input
            type="text"
            value={branch}
            onChange={(e) => setBranch(e.target.value)}
            placeholder="branch"
            className="w-28"
            disabled={loading}
          />
        </div>

        {/* Quick select from GitHub repos */}
        {isAuthenticated && ghRepos.length > 0 && (
          <div className="mt-3">
            <button
              onClick={() => setShowRepos(!showRepos)}
              className="text-xs text-muted-foreground hover:text-foreground transition-colors"
            >
              {showRepos
                ? "Hide repos"
                : `Or pick from your ${ghRepos.length} repos →`}
            </button>
            <AnimatePresence>
              {showRepos && (
                <motion.div
                  initial={{ height: 0, opacity: 0 }}
                  animate={{ height: "auto", opacity: 1 }}
                  exit={{ height: 0, opacity: 0 }}
                  className="overflow-hidden"
                >
                  <div className="mt-2 max-h-48 overflow-y-auto rounded-lg border border-border divide-y divide-border">
                    {ghRepos.map((repo) => (
                      <button
                        key={repo.id}
                        onClick={() => handleSelectRepo(repo)}
                        className="flex items-center justify-between w-full px-3 py-2 text-left hover:bg-accent/50 transition-colors"
                      >
                        <div className="flex items-center gap-2 min-w-0">
                          <GitBranch
                            size={14}
                            className="text-muted-foreground shrink-0"
                          />
                          <span className="text-sm truncate">
                            {repo.full_name}
                          </span>
                          {repo.language && (
                            <span className="text-[10px] px-1.5 py-0.5 bg-muted text-muted-foreground rounded">
                              {repo.language}
                            </span>
                          )}
                        </div>
                        <span className="text-xs text-muted-foreground">
                          {repo.stargazers_count}★
                        </span>
                      </button>
                    ))}
                  </div>
                </motion.div>
              )}
            </AnimatePresence>
          </div>
        )}

        {/* Action buttons */}
        <div className="flex items-center gap-3 mt-4">
          <Button onClick={handleAnalyze} disabled={loading || !repoUrl.trim()}>
            {loading ? (
              <Loader2 size={16} className="mr-2 animate-spin" />
            ) : (
              <Zap size={16} className="mr-2" />
            )}
            {loading ? "Analyzing..." : "Analyze"}
          </Button>

          {result && (
            <Button variant="outline" onClick={handleReset}>
              Reset
            </Button>
          )}

          {!isAuthenticated && (
            <Button
              variant="outline"
              onClick={signInWithGitHub}
              className="gap-2"
            >
              <Github size={14} />
              Sign in to import repos
            </Button>
          )}
        </div>
      </Card>

      {/* Analysis progress */}
      {loading && <AnalysisProgress step={step} />}

      {/* Error */}
      {error && (
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          className="flex items-start gap-3 p-4 mb-6 bg-destructive/10 border border-destructive/20 rounded-xl"
        >
          <AlertCircle size={18} className="text-destructive mt-0.5 shrink-0" />
          <div>
            <p className="text-sm font-medium text-destructive">
              Analysis failed
            </p>
            <p className="text-xs text-muted-foreground mt-1">{error}</p>
          </div>
        </motion.div>
      )}

      {/* Result */}
      {result?.blueprint && (
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
        >
          {/* Success banner */}
          <Card className="flex items-center gap-3 p-4 mb-6 border-foreground/20">
            <CheckCircle2 size={18} className="text-foreground" />
            <div className="flex-1">
              <p className="text-sm font-medium">
                Analysis complete in {result.duration}
              </p>
              <p className="text-xs text-muted-foreground mt-0.5">
                Detected: {result.blueprint.language}{" "}
                {result.blueprint.version && `(${result.blueprint.version})`}
                {result.blueprint.package_manager &&
                  ` • ${result.blueprint.package_manager}`}
              </p>
            </div>
            <Button onClick={handleSave} disabled={saving} size="sm">
              {saving ? (
                <Loader2 size={14} className="mr-1.5 animate-spin" />
              ) : (
                <Save size={14} className="mr-1.5" />
              )}
              Save Project
            </Button>
          </Card>

          {/* Blueprint editor */}
          <BlueprintEditor blueprint={result.blueprint} readOnly={false} />
        </motion.div>
      )}
    </div>
  );
}
