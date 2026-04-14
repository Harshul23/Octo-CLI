import { useState, useEffect } from "react";
import { useAuth } from "../contexts/AuthContext";
import { useNavigate, useLocation } from "react-router-dom";
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
  Star,
} from "lucide-react";
import BlueprintEditor from "../components/BlueprintEditor.jsx";
import AnalysisProgress from "../components/AnalysisProgress.jsx";
import { Button } from "../components/ui/Button.jsx";
import { Card } from "../components/ui/Card.jsx";
import { Input } from "../components/ui/Input.jsx";

export default function Analyze() {
  const navigate = useNavigate();
  const location = useLocation();
  const { isAuthenticated, signInWithGitHub } = useAuth();
  const { result, loading, error, step, analyzeGitHub, reset } =
    useAnalysisStore();
  const { createProject } = useProjectStore();

  const [repoUrl, setRepoUrl] = useState(location.state?.repoUrl || "");
  const [branch, setBranch] = useState(location.state?.branch || "");
  const [ghRepos, setGhRepos] = useState([]);
  const [showRepos, setShowRepos] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (isAuthenticated) {
      api
        .listGitHubRepos()
        .then((data) => setGhRepos(data.repos || []))
        .catch((err) =>
          console.warn("Could not fetch GitHub repos:", err.message),
        );
    }
  }, [isAuthenticated]);

  const handleAnalyze = async () => {
    if (!repoUrl.trim()) return;

    try {
      await analyzeGitHub(repoUrl.trim(), branch.trim());
    } catch (err) {
      // Error handled by store
    }
  };

  const handleSave = async () => {
    if (!result?.blueprint) return;

    setSaving(true);

    try {
      const isPublic =
        location.state?.isPrivate !== undefined
          ? !location.state.isPrivate
          : true;
      const projName = location.state?.repoName || result.blueprint.name;
      const projDesc =
        location.state?.repoDescription ||
        `${result.blueprint.language} project analyzed from GitHub`;
      const finalRepoUrl = location.state?.htmlUrl || repoUrl;

      const project = await createProject({
        name: projName,
        repo_url: finalRepoUrl,
        description: projDesc,
        is_public: isPublic,
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
        <h1 className="text-2xl font-bold text-white tracking-tight">
          Analyze Repository
        </h1>
        <p className="text-neutral-400 text-sm mt-1">
          Paste a GitHub URL or pick from your repos to generate an Octo
          blueprint.
        </p>
      </div>

      <Card className="p-6 mb-6 bg-[#0A0A0A] border-[#222]">
        <label className="block text-sm font-medium mb-2 text-neutral-200">
          Repository URL
        </label>

        <div className="flex gap-3">
          <div className="flex-1 relative">
            <Github
              size={16}
              strokeWidth={2}
              className="absolute left-3 top-1/2 -translate-y-1/2 text-neutral-500"
            />

            <Input
              type="text"
              value={repoUrl}
              onChange={(e) => setRepoUrl(e.target.value)}
              placeholder="https://github.com/user/repo.git"
              className="pl-10 bg-[#141414] border-[#333] text-white placeholder:text-neutral-500 rounded-lg"
              disabled={loading}
              onKeyDown={(e) => e.key === "Enter" && handleAnalyze()}
            />
          </div>

          <Input
            type="text"
            value={branch}
            onChange={(e) => setBranch(e.target.value)}
            placeholder="branch"
            className="w-28 bg-[#141414] border-[#333] text-white placeholder:text-neutral-500 rounded-lg"
            disabled={loading}
          />
        </div>

        {isAuthenticated && ghRepos.length > 0 && (
          <div className="mt-3">
            <button
              onClick={() => setShowRepos(!showRepos)}
              className="text-xs text-neutral-400 hover:text-white transition-colors"
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
                  <div className="mt-2 max-h-48 overflow-y-auto rounded-lg border border-[#333] bg-[#141414] divide-y divide-[#222]">
                    {ghRepos.map((repo) => (
                      <button
                        key={repo.id}
                        onClick={() => handleSelectRepo(repo)}
                        className="flex items-center justify-between w-full px-3 py-2.5 text-left hover:bg-[#222] transition-colors"
                      >
                        <div className="flex items-center gap-2 min-w-0">
                          <GitBranch
                            size={14}
                            strokeWidth={2}
                            className="text-neutral-500 shrink-0"
                          />

                          <span className="text-sm text-neutral-200 truncate">
                            {repo.full_name}
                          </span>

                          {repo.language && (
                            <span className="text-[10px] px-1.5 py-0.5 bg-[#222] text-neutral-400 rounded">
                              {repo.language}
                            </span>
                          )}
                        </div>

                        <span className="text-xs text-neutral-500">
                          <Star size={10} className="inline mr-0.5" />
                          {repo.stargazers_count}
                        </span>
                      </button>
                    ))}
                  </div>
                </motion.div>
              )}
            </AnimatePresence>
          </div>
        )}

        <div className="flex items-center gap-3 mt-5">
          <Button
            onClick={handleAnalyze}
            disabled={loading || !repoUrl.trim()}
            className="bg-white text-black hover:bg-neutral-200 transition-colors"
          >
            {loading ? (
              <Loader2 size={16} className="mr-2 animate-spin" />
            ) : (
              <Zap
                size={16}
                fill="currentColor"
                strokeWidth={2}
                className="mr-2"
              />
            )}
            {loading ? "Analyzing..." : "Analyze"}
          </Button>

          {result && (
            <Button
              variant="outline"
              className="border-[#333] text-white hover:bg-[#141414]"
              onClick={handleReset}
            >
              Reset
            </Button>
          )}

          {!isAuthenticated && (
            <Button
              variant="outline"
              onClick={signInWithGitHub}
              className="gap-2 border-[#333] text-white hover:bg-[#141414]"
            >
              <Github size={14} />
              Sign in to import repos
            </Button>
          )}
        </div>
      </Card>

      {loading && <AnalysisProgress step={step} />}

      {error && (
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          className="flex items-start gap-3 p-4 mb-6 bg-red-950/20 border border-red-900/50 rounded-xl"
        >
          <AlertCircle size={18} className="text-red-500 mt-0.5 shrink-0" />

          <div>
            <p className="text-sm font-medium text-red-500">Analysis failed</p>

            <p className="text-xs text-red-400 mt-1">{error}</p>
          </div>
        </motion.div>
      )}

      {result?.blueprint && (
        <>
          <Card className="flex items-center gap-3 p-4 mb-6 bg-[#0A0A0A] border-[#222]">
            <CheckCircle2 size={18} className="text-green-500" />

            <div className="flex-1">
              <p className="text-sm font-medium text-white">
                Analysis complete in {result.duration}
              </p>

              <p className="text-xs text-neutral-400 mt-0.5">
                Detected: {result.blueprint.language}{" "}
                {result.blueprint.version && `(${result.blueprint.version})`}
                {result.blueprint.package_manager &&
                  ` • ${result.blueprint.package_manager}`}
              </p>
            </div>

            <Button
              onClick={handleSave}
              disabled={saving}
              size="sm"
              className="bg-white text-black hover:bg-neutral-200"
            >
              {saving ? (
                <Loader2 size={14} className="mr-1.5 animate-spin" />
              ) : (
                <Save size={14} fill="currentColor" className="mr-1.5" />
              )}
              Save Project
            </Button>
          </Card>

          <BlueprintEditor blueprint={result.blueprint} readOnly={false} />
        </>
      )}
    </div>
  );
}
