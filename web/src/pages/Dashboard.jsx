import { useEffect, useState } from "react";
import { useAuth } from "../contexts/AuthContext";
import { useNavigate, useOutletContext } from "react-router-dom";
import { useProjectStore } from "../store/index.js";
import api from "../services/api.js";
import {
  Plus,
  Zap,
  Folder,
  Globe,
  FolderGit2,
  Search,
  Grid,
  List,
  ChevronLeft,
  ChevronRight,
} from "lucide-react";
import { motion } from "framer-motion";
import ProjectCard from "../components/ProjectCard.jsx";
import { Button } from "../components/ui/Button.jsx";
import { Card, CardContent } from "../components/ui/Card.jsx";

export default function Dashboard() {
  const navigate = useNavigate();
  const { isExpanded, setIsExpanded } = useOutletContext();
  const { isAuthenticated, signInWithGitHub } = useAuth();
  const { projects, loading, fetchProjects, deleteProject } = useProjectStore();
  const [searchQuery, setSearchQuery] = useState("");
  const [viewMode, setViewMode] = useState("grid");

  useEffect(() => {
    const handleKeyDown = (e) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        document.getElementById("project-search")?.focus();
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, []);

  useEffect(() => {
    if (isAuthenticated) {
      fetchProjects();
    }
  }, [isAuthenticated]);

  // Compute stats directly from projects
  const stats = {
    total: projects.length,
    public: projects.filter((p) => p.is_public).length,
    languages: [...new Set(projects.map((p) => p.language).filter(Boolean))],
  };

  const handleDelete = async (id) => {
    if (window.confirm("Are you sure you want to delete this project?")) {
      await deleteProject(id);
    }
  };

  const filteredProjects = projects.filter(
    (p) =>
      (p.repo_name || "Untitled")
        .toLowerCase()
        .includes(searchQuery.toLowerCase()) ||
      (p.language || "").toLowerCase().includes(searchQuery.toLowerCase()),
  );

  return (
    <div className="max-w-6xl ">
      <div className="flex items-center px-4 justify-center border-b-2 h-17.5 border-neutral-600 relative">
        <div className="absolute left-4 hidden lg:flex">
          <button
            onClick={() => setIsExpanded(!isExpanded)}
            className="p-1.5 rounded-md text-neutral-400 hover:text-white hover:bg-neutral-800 transition-colors"
          >
            {isExpanded ? (
              <ChevronLeft size={18} />
            ) : (
              <ChevronRight size={18} />
            )}
          </button>
        </div>
        <span className="text-xl font-medium text-neutral-300">Overview</span>
      </div>

      <div className="flex flex-col px-8 py-6">
        {/* Header */}
        <div className="flex flex-col items-start justify-between gap-4 mb-8 sm:flex-row sm:items-center">
          <div className="relative w-full sm:max-w-md bg-neutral-900 border border-neutral-700 focus-within:border-neutral-500 rounded-lg flex items-center px-3 transition-colors">
            <Search className="text-neutral-400 shrink-0" size={16} />
            <input
              id="project-search"
              type="text"
              placeholder="Search projects by name or language..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full h-10 px-3 text-sm bg-transparent outline-none text-neutral-200 placeholder:text-neutral-500"
            />
            <div className="hidden sm:flex border border-neutral-700 bg-neutral-800 rounded px-1.5 py-0.5 items-center justify-center shrink-0">
              <span className="text-[10px] font-medium text-neutral-400">
                ⌘K
              </span>
            </div>
          </div>

          <div className="flex flex-col sm:flex-row w-full sm:w-auto items-stretch sm:items-center gap-3">
            <div className="hidden sm:flex items-center bg-neutral-900 border border-neutral-700 rounded-lg p-0.5">
              <button
                onClick={() => setViewMode("grid")}
                className={`p-1.5 rounded-md transition-colors ${viewMode === "grid" ? "bg-neutral-700 text-white shadow-sm" : "text-neutral-400 hover:text-white"}`}
                title="Grid View"
              >
                <Grid size={16} />
              </button>
              <button
                onClick={() => setViewMode("list")}
                className={`p-1.5 rounded-md transition-colors ${viewMode === "list" ? "bg-neutral-700 text-white shadow-sm" : "text-neutral-400 hover:text-white"}`}
                title="List View"
              >
                <List size={16} />
              </button>
            </div>

            <Button
              variant="outline"
              size="sm"
              onClick={() => navigate("/repos")}
              className="gap-2 h-10 bg-[#171717] border-[#393939] border rounded-lg hover:bg-[#252525] transition-colors"
            >
              <FolderGit2 size={16} />
              Browse Repos
            </Button>

            <Button
              size="sm"
              onClick={() => navigate("/analyze")}
              className="gap-2 h-10 text-sm font-medium rounded-lg bg-white text-black hover:bg-neutral-200 transition-colors"
            >
              <Plus size={16} />
              New Project
            </Button>
          </div>
        </div>

        {/* Stats */}
        <div className="grid grid-cols-1 gap-4 mb-8 sm:grid-cols-3">
          <Card>
            <CardContent className="p-4">
              <div className="flex items-center gap-3">
                <div className="flex items-center justify-center w-10 h-10 rounded-lg bg-accent">
                  <Folder size={18} className="text-foreground" />
                </div>

                <div>
                  <p className="text-2xl font-bold">{stats.total}</p>
                  <p className="text-xs text-muted-foreground">
                    Total Projects
                  </p>
                </div>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardContent className="p-4">
              <div className="flex items-center gap-3">
                <div className="flex items-center justify-center w-10 h-10 rounded-lg bg-accent">
                  <Globe size={18} className="text-foreground" />
                </div>

                <div>
                  <p className="text-2xl font-bold">{stats.public}</p>
                  <p className="text-xs text-muted-foreground">
                    Public Configs
                  </p>
                </div>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardContent className="p-4">
              <div className="flex items-center gap-3">
                <div className="flex items-center justify-center w-10 h-10 rounded-lg bg-accent">
                  <Zap size={18} className="text-foreground" />
                </div>

                <div>
                  <p className="text-2xl font-bold">{stats.languages.length}</p>
                  <p className="text-xs text-muted-foreground">Languages</p>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Project list */}
        {loading ? (
          <div
            className={
              viewMode === "grid"
                ? "grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3"
                : "flex flex-col gap-4"
            }
          >
            {[1, 2, 3, 4, 5, 6].map((i) => (
              <Card
                key={i}
                className="p-5 border border-neutral-800 bg-neutral-900/50"
              >
                <div className="flex justify-between items-start mb-4">
                  <div className="w-8 h-8 rounded-full bg-neutral-800 animate-pulse" />
                  <div className="w-16 h-5 rounded-md bg-neutral-800 animate-pulse" />
                </div>
                <div className="w-3/4 h-5 mb-3 rounded-md bg-neutral-800 animate-pulse" />
                <div className="w-1/2 h-4 mb-6 rounded-md bg-neutral-800 animate-pulse" />
                <div className="flex justify-between items-center">
                  <div className="w-16 h-4 rounded-md bg-neutral-800 animate-pulse" />
                  <div className="w-20 h-4 rounded-md bg-neutral-800 animate-pulse" />
                </div>
              </Card>
            ))}
          </div>
        ) : projects.length === 0 ? (
          <div className="py-16 text-center">
            <div className="flex items-center justify-center w-16 h-16 mx-auto mb-4 rounded-2xl bg-neutral-800/50 border border-neutral-700">
              <Folder size={28} className="text-neutral-400" />
            </div>
            <h3 className="mb-2 text-lg font-medium">No projects yet</h3>
            <p className="max-w-md mx-auto mb-6 text-sm text-neutral-400">
              Analyze a GitHub repository to create your first Octo
              configuration.
            </p>
            <Button onClick={() => navigate("/analyze")}>
              Analyze a Repository
            </Button>
          </div>
        ) : filteredProjects.length === 0 ? (
          <div className="py-16 text-center">
            <div className="flex items-center justify-center w-16 h-16 mx-auto mb-4 rounded-2xl bg-neutral-800/50 border border-neutral-700">
              <Search size={28} className="text-neutral-400" />
            </div>
            <h3 className="mb-2 text-lg font-medium">No matches found</h3>
            <p className="max-w-md mx-auto mb-6 text-sm text-neutral-400">
              We couldn't find any projects matching "{searchQuery}".
            </p>
            <Button variant="outline" onClick={() => setSearchQuery("")}>
              Clear Search
            </Button>
          </div>
        ) : (
          <div
            className={
              viewMode === "grid"
                ? "grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3"
                : "flex flex-col gap-4"
            }
          >
            {filteredProjects.map((project, i) => (
              <motion.div
                key={project.id}
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: i * 0.05 }}
              >
                <ProjectCard
                  project={project}
                  onDelete={() => handleDelete(project.id)}
                  onClick={() => navigate(`/projects/${project.id}`)}
                />
              </motion.div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
