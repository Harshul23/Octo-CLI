import { useState, useEffect, useMemo } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../contexts/AuthContext.jsx";
import { useAnalysisStore, useProjectStore } from "../store/index.js";
import api from "../services/api.js";
import { motion, AnimatePresence } from "framer-motion";
import {
  Search,
  GitBranch,
  Lock,
  Globe,
  Star,
  Loader2,
  Zap,
  ArrowRight,
  Clock,
  Filter,
} from "lucide-react";
import { Button } from "../components/ui/Button.jsx";
import { Input } from "../components/ui/Input.jsx";
import { Badge } from "../components/ui/Badge.jsx";
import { Card } from "../components/ui/Card.jsx";
import OctoLogo from "../components/ui/OctoLogo.jsx";

export default function RepoSelector() {
  const navigate = useNavigate();
  const { isAuthenticated, accessToken, providerToken, signInWithGitHub } =
    useAuth();
  const {
    analyzeGitHub,
    loading: analyzing,
    result,
    step,
  } = useAnalysisStore();
  const { createProject } = useProjectStore();

  const [repos, setRepos] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [visibilityFilter, setVisibilityFilter] = useState("all"); // all, public, private
  const [langFilter, setLangFilter] = useState("all");
  const [page, setPage] = useState(1);
  const perPage = 12;

  useEffect(() => {
    if (isAuthenticated && accessToken) {
      api.setToken(accessToken);
      if (providerToken) api.setProviderToken(providerToken);
      fetchRepos();
    }
  }, [isAuthenticated, accessToken, providerToken]);

  const fetchRepos = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listGitHubRepos();
      setRepos(data.repos || []);
    } catch (err) {
      setError(err.message);
    }
    setLoading(false);
  };

  const filteredRepos = useMemo(() => {
    let result = repos;
    if (searchQuery) {
      const q = searchQuery.toLowerCase();
      result = result.filter(
        (r) =>
          r.name.toLowerCase().includes(q) ||
          r.full_name.toLowerCase().includes(q) ||
          (r.description && r.description.toLowerCase().includes(q)),
      );
    }
    if (visibilityFilter === "public")
      result = result.filter((r) => !r.private);
    if (visibilityFilter === "private")
      result = result.filter((r) => r.private);
    if (langFilter !== "all")
      result = result.filter((r) => r.language === langFilter);
    return result;
  }, [repos, searchQuery, visibilityFilter, langFilter]);

  const languages = useMemo(() => {
    const langs = [...new Set(repos.map((r) => r.language).filter(Boolean))];
    return langs.sort();
  }, [repos]);

  const paginatedRepos = filteredRepos.slice(0, page * perPage);
  const hasMore = paginatedRepos.length < filteredRepos.length;

  const handleInitialize = async (repo) => {
    try {
      const result = await analyzeGitHub(repo.clone_url, repo.default_branch);
      if (result?.blueprint) {
        const project = await createProject({
          name: result.blueprint.name || repo.name,
          repo_url: repo.html_url,
          description:
            repo.description || `${result.blueprint.language} project`,
          is_public: !repo.private,
          blueprint: result.blueprint,
        });
        navigate(`/projects/${project.id}`);
      }
    } catch (err) {
      console.error("Init failed:", err);
    }
  };

  const timeAgo = (dateStr) => {
    if (!dateStr) return "";
    const diff = Date.now() - new Date(dateStr).getTime();
    const mins = Math.floor(diff / 60000);
    if (mins < 60) return `${mins}m ago`;
    const hrs = Math.floor(mins / 60);
    if (hrs < 24) return `${hrs}h ago`;
    const days = Math.floor(hrs / 24);
    if (days < 30) return `${days}d ago`;
    return `${Math.floor(days / 30)}mo ago`;
  };

  if (!isAuthenticated) {
    return (
      <div className="flex flex-col items-center justify-center h-full py-20 px-6">
        <OctoLogo size={48} className="mb-6 opacity-50" />
        <h2 className="text-xl font-bold mb-2">Connect your GitHub</h2>
        <p className="text-sm text-[hsl(var(--muted-foreground))] mb-6 text-center max-w-md">
          Sign in with GitHub to browse your repositories and initialize them
          with Octo.
        </p>
        <Button onClick={signInWithGitHub} className="gap-2">
          <GitBranch size={16} />
          Sign in with GitHub
        </Button>
      </div>
    );
  }

  return (
    <div className="p-6 lg:p-8 max-w-6xl mx-auto">
      {/* Header */}
      <div className="mb-6">
        <h1 className="text-2xl font-bold">Select Repository</h1>
        <p className="text-sm text-[hsl(var(--muted-foreground))] mt-1">
          Choose a repository to initialize with Octo. We'll scan the codebase
          and generate a configuration.
        </p>
      </div>

      {/* Scope notice */}
      <Card className="p-4 mb-6 flex items-start gap-3">
        <div className="w-8 h-8 rounded-full bg-[hsl(var(--secondary))] flex items-center justify-center shrink-0">
          <Lock size={14} />
        </div>
        <div>
          <p className="text-sm font-medium">Repository Access</p>
          <p className="text-xs text-[hsl(var(--muted-foreground))] mt-0.5">
            Octo has read access to your public and private repositories. We
            only read code to generate configurations — no changes are ever
            pushed.
          </p>
        </div>
      </Card>

      {/* Search & Filters */}
      <div className="flex flex-col sm:flex-row gap-3 mb-6">
        <div className="relative flex-1">
          <Search
            size={14}
            className="absolute left-3 top-1/2 -translate-y-1/2 text-[hsl(var(--muted-foreground))]"
          />
          <Input
            value={searchQuery}
            onChange={(e) => {
              setSearchQuery(e.target.value);
              setPage(1);
            }}
            placeholder="Search repositories..."
            className="pl-9"
          />
        </div>
        <div className="flex gap-2">
          {["all", "public", "private"].map((v) => (
            <Button
              key={v}
              variant={visibilityFilter === v ? "default" : "outline"}
              size="sm"
              onClick={() => {
                setVisibilityFilter(v);
                setPage(1);
              }}
              className="capitalize text-xs"
            >
              {v === "public" && <Globe size={12} />}
              {v === "private" && <Lock size={12} />}
              {v}
            </Button>
          ))}
        </div>
      </div>

      {/* Language filter pills */}
      {languages.length > 0 && (
        <div className="flex flex-wrap gap-1.5 mb-6">
          <button
            onClick={() => {
              setLangFilter("all");
              setPage(1);
            }}
            className={`px-2.5 py-1 text-xs rounded-md transition-colors ${
              langFilter === "all"
                ? "bg-[hsl(var(--primary))] text-[hsl(var(--primary-foreground))]"
                : "bg-[hsl(var(--secondary))] text-[hsl(var(--muted-foreground))] hover:text-[hsl(var(--foreground))]"
            }`}
          >
            All
          </button>
          {languages.map((lang) => (
            <button
              key={lang}
              onClick={() => {
                setLangFilter(lang);
                setPage(1);
              }}
              className={`px-2.5 py-1 text-xs rounded-md transition-colors ${
                langFilter === lang
                  ? "bg-[hsl(var(--primary))] text-[hsl(var(--primary-foreground))]"
                  : "bg-[hsl(var(--secondary))] text-[hsl(var(--muted-foreground))] hover:text-[hsl(var(--foreground))]"
              }`}
            >
              {lang}
            </button>
          ))}
        </div>
      )}

      {/* Repo grid */}
      {loading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <Card key={i} className="p-4 animate-pulse">
              <div className="h-4 bg-[hsl(var(--muted))] rounded w-3/4 mb-3" />
              <div className="h-3 bg-[hsl(var(--muted))] rounded w-1/2 mb-4" />
              <div className="h-3 bg-[hsl(var(--muted))] rounded w-full" />
            </Card>
          ))}
        </div>
      ) : error ? (
        <Card className="p-8 text-center">
          <p className="text-sm text-red-400 mb-3">{error}</p>
          <Button variant="outline" size="sm" onClick={fetchRepos}>
            Retry
          </Button>
        </Card>
      ) : filteredRepos.length === 0 ? (
        <Card className="p-8 text-center">
          <GitBranch
            size={24}
            className="mx-auto mb-2 text-[hsl(var(--muted-foreground))]"
          />
          <p className="text-sm text-[hsl(var(--muted-foreground))]">
            No repositories found matching your filters
          </p>
        </Card>
      ) : (
        <>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
            {paginatedRepos.map((repo, i) => (
              <motion.div
                key={repo.id}
                initial={{ opacity: 0, y: 8 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: i * 0.02 }}
              >
                <Card className="p-4 hover:border-[hsl(var(--ring))] transition-colors group">
                  <div className="flex items-start justify-between gap-2 mb-2">
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <h3 className="text-sm font-semibold truncate">
                          {repo.name}
                        </h3>
                        <Badge
                          variant={repo.private ? "outline" : "secondary"}
                          className="text-[10px] shrink-0"
                        >
                          {repo.private ? (
                            <>
                              <Lock size={8} className="mr-0.5" /> Private
                            </>
                          ) : (
                            <>
                              <Globe size={8} className="mr-0.5" /> Public
                            </>
                          )}
                        </Badge>
                      </div>
                      {repo.description && (
                        <p className="text-xs text-[hsl(var(--muted-foreground))] mt-1 line-clamp-2">
                          {repo.description}
                        </p>
                      )}
                    </div>
                  </div>
                  <div className="flex items-center justify-between mt-3">
                    <div className="flex items-center gap-2 text-[11px] text-[hsl(var(--muted-foreground))]">
                      {repo.language && (
                        <span className="px-1.5 py-0.5 bg-[hsl(var(--secondary))] rounded">
                          {repo.language}
                        </span>
                      )}
                      {repo.stargazers_count > 0 && (
                        <span className="flex items-center gap-0.5">
                          <Star size={10} /> {repo.stargazers_count}
                        </span>
                      )}
                      <span className="flex items-center gap-0.5">
                        <Clock size={10} /> {timeAgo(repo.updated_at)}
                      </span>
                    </div>
                    <Button
                      size="sm"
                      disabled={analyzing}
                      onClick={() => handleInitialize(repo)}
                      className="text-xs gap-1 opacity-0 group-hover:opacity-100 transition-opacity"
                    >
                      {analyzing ? (
                        <Loader2 size={12} className="animate-spin" />
                      ) : (
                        <Zap size={12} />
                      )}
                      Initialize
                    </Button>
                  </div>
                </Card>
              </motion.div>
            ))}
          </div>

          {hasMore && (
            <div className="text-center mt-6">
              <Button variant="outline" onClick={() => setPage((p) => p + 1)}>
                Load More ({filteredRepos.length - paginatedRepos.length}{" "}
                remaining)
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
