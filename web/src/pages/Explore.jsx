import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useExploreStore } from "../store/index.js";
import { motion } from "framer-motion";
import { Search, Clock, Star, Code2, Globe } from "lucide-react";
import ProjectCard from "../components/ProjectCard.jsx";
import { Button } from "../components/ui/Button.jsx";
import { Card } from "../components/ui/Card.jsx";
import { Input } from "../components/ui/Input.jsx";

const LANGUAGES = ["All", "Node", "Python", "Go", "Java", "Rust", "Ruby"];
const SORT_OPTIONS = [
  { value: "newest", label: "Newest", icon: Clock },
  { value: "stars", label: "Most Stars", icon: Star },
];

export default function Explore() {
  const navigate = useNavigate();
  const { projects, loading, filters, fetchExplore, setFilters } =
    useExploreStore();
  const [searchInput, setSearchInput] = useState("");

  useEffect(() => {
    fetchExplore(filters);
  }, []);

  const handleSearch = (e) => {
    e.preventDefault();
    const newFilters = { ...filters, search: searchInput };
    setFilters(newFilters);
    fetchExplore(newFilters);
  };

  const handleLanguageFilter = (lang) => {
    const language = lang === "All" ? "" : lang;
    const newFilters = { ...filters, language };
    setFilters(newFilters);
    fetchExplore(newFilters);
  };

  const handleSort = (sort) => {
    const newFilters = { ...filters, sort };
    setFilters(newFilters);
    fetchExplore(newFilters);
  };

  return (
    <div className="p-6 lg:p-8 max-w-6xl mx-auto">
      {/* Header */}
      <div className="mb-8">
        <div className="flex items-center gap-3 mb-2">
          <div className="w-10 h-10 rounded-lg bg-accent flex items-center justify-center">
            <Globe size={20} className="text-foreground" />
          </div>
          <div>
            <h1 className="text-2xl font-bold">Explore</h1>
            <p className="text-muted-foreground text-sm">
              Discover Octo-fied projects shared by the community
            </p>
          </div>
        </div>
      </div>

      {/* Search & Filters */}
      <Card className="p-4 mb-6">
        <form onSubmit={handleSearch} className="flex gap-3 mb-4">
          <div className="flex-1 relative">
            <Search
              size={16}
              className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"
            />
            <Input
              type="text"
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              placeholder="Search projects..."
              className="pl-10"
            />
          </div>
          <Button type="submit">Search</Button>
        </form>

        <div className="flex flex-wrap items-center justify-between gap-3">
          {/* Language pills */}
          <div className="flex flex-wrap gap-2">
            {LANGUAGES.map((lang) => {
              const isActive =
                (lang === "All" && !filters.language) ||
                filters.language === lang;
              return (
                <button
                  key={lang}
                  onClick={() => handleLanguageFilter(lang)}
                  className={`px-3 py-1.5 text-xs font-medium rounded-full transition-colors ${
                    isActive
                      ? "bg-foreground text-background"
                      : "bg-muted text-muted-foreground border border-border hover:border-foreground/20"
                  }`}
                >
                  {lang}
                </button>
              );
            })}
          </div>

          {/* Sort */}
          <div className="flex items-center gap-2">
            {SORT_OPTIONS.map((opt) => (
              <button
                key={opt.value}
                onClick={() => handleSort(opt.value)}
                className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${
                  filters.sort === opt.value
                    ? "bg-accent text-foreground"
                    : "text-muted-foreground hover:text-foreground"
                }`}
              >
                <opt.icon size={12} />
                {opt.label}
              </button>
            ))}
          </div>
        </div>
      </Card>

      {/* Results */}
      {loading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {[1, 2, 3, 4, 5, 6].map((i) => (
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
            <Code2 size={28} className="text-muted-foreground" />
          </div>
          <h3 className="text-lg font-medium mb-2">No projects found</h3>
          <p className="text-muted-foreground text-sm max-w-md mx-auto">
            Try a different search query or language filter, or be the first to
            publish a project!
          </p>
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
                showUser
                onClick={() => navigate(`/explore/${project.id}`)}
              />
            </motion.div>
          ))}
        </div>
      )}
    </div>
  );
}
