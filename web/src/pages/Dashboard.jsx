import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../contexts/AuthContext.jsx";
import { useProjectStore } from "../store/index.js";
import api from "../services/api.js";
import { Plus, Zap, Folder, Globe, FolderGit2 } from "lucide-react";
import { motion } from "framer-motion";
import ProjectCard from "../components/ProjectCard.jsx";
import { Button } from "../components/ui/Button.jsx";
import { Card, CardContent } from "../components/ui/Card.jsx";

export default function Dashboard() {
  const navigate = useNavigate();
  const { isAuthenticated, signInWithGitHub, accessToken, providerToken } =
    useAuth();
  const { projects, loading, fetchProjects, deleteProject } = useProjectStore();
  const [stats, setStats] = useState({ total: 0, public: 0, languages: [] });

  useEffect(() => {
    if (isAuthenticated && accessToken) {
      api.setToken(accessToken);
      if (providerToken) api.setProviderToken(providerToken);
      fetchProjects();
    }
  }, [isAuthenticated, accessToken, providerToken]);

  useEffect(() => {
    if (projects.length > 0) {
      const langs = [
        ...new Set(projects.map((p) => p.language).filter(Boolean)),
      ];
      setStats({
        total: projects.length,
        public: projects.filter((p) => p.is_public).length,
        languages: langs,
      });
    }
  }, [projects]);

  if (!isAuthenticated) {
    return (
      <div className="flex flex-col items-center justify-center h-full py-20 px-6">
        <div className="w-16 h-16 rounded-2xl bg-accent flex items-center justify-center mb-6">
          <Zap size={28} className="text-foreground" />
        </div>
        <h2 className="text-2xl font-bold mb-2">Welcome to Octo</h2>
        <p className="text-muted-foreground mb-8 text-center max-w-md">
          Sign in with GitHub to start analyzing repositories and managing your
          project configurations.
        </p>
        <Button onClick={signInWithGitHub}>Sign in with GitHub</Button>
      </div>
    );
  }

  const handleDelete = async (id) => {
    if (window.confirm("Are you sure you want to delete this project?")) {
      await deleteProject(id);
    }
  };

  return (
    <div className="p-6 lg:p-8 max-w-6xl mx-auto">
      {/* Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 mb-8">
        <div>
          <h1 className="text-2xl font-bold">Dashboard</h1>
          <p className="text-muted-foreground text-sm mt-1">
            Manage your Octo-fied projects
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => navigate("/repos")}
            className="gap-2"
          >
            <FolderGit2 size={16} />
            Browse Repos
          </Button>
          <Button
            size="sm"
            onClick={() => navigate("/analyze")}
            className="gap-2"
          >
            <Plus size={16} />
            New Project
          </Button>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-8">
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-lg bg-accent flex items-center justify-center">
                <Folder size={18} className="text-foreground" />
              </div>
              <div>
                <p className="text-2xl font-bold">{stats.total}</p>
                <p className="text-xs text-muted-foreground">Total Projects</p>
              </div>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-lg bg-accent flex items-center justify-center">
                <Globe size={18} className="text-foreground" />
              </div>
              <div>
                <p className="text-2xl font-bold">{stats.public}</p>
                <p className="text-xs text-muted-foreground">Public Configs</p>
              </div>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-lg bg-accent flex items-center justify-center">
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
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {[1, 2, 3].map((i) => (
            <Card key={i} className="p-5 animate-pulse">
              <div className="h-5 bg-muted rounded w-3/4 mb-3" />
              <div className="h-4 bg-muted rounded w-1/2 mb-4" />
              <div className="h-3 bg-muted rounded w-full" />
            </Card>
          ))}
        </div>
      ) : projects.length === 0 ? (
        <div className="text-center py-16">
          <div className="w-16 h-16 rounded-2xl bg-muted flex items-center justify-center mx-auto mb-4">
            <Folder size={28} className="text-muted-foreground" />
          </div>
          <h3 className="text-lg font-medium mb-2">No projects yet</h3>
          <p className="text-muted-foreground text-sm mb-6 max-w-md mx-auto">
            Analyze a GitHub repository to create your first Octo configuration.
          </p>
          <Button onClick={() => navigate("/analyze")}>
            Analyze a Repository
          </Button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {projects.map((project, i) => (
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
  );
}
