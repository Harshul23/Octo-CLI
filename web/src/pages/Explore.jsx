import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useExploreStore } from "../store/index.js";
import { motion } from "framer-motion";
import { Search, Clock, Star, Code2, Globe } from "lucide-react";
import ProjectCard from "../components/ProjectCard.jsx";
import { Button } from "../components/ui/Button.jsx";
import { Card } from "../components/ui/Card.jsx";
import { Input } from "../components/ui/Input.jsx";
import { GoTelescopeFill } from "react-icons/go";

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
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fetchExplore, filters]);

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
          <div className="w-10 h-10 rounded-lg bg-neutral-800/80 border border-[#333] flex items-center justify-center">
            <GoTelescopeFill />
          </div>
          <div>
            <h1 className="text-2xl font-bold text-white tracking-tight">Explore</h1>
            <p className="text-neutral-400 text-sm mt-1">
              Discover Octo-fied projects shared by the community
            </p>
          </div>
        </div>
      </div>

      {/* Search & Filters */}
      <Card className="p-4 mb-6 bg-[#0A0A0A] border-[#222]">
        <form onSubmit={handleSearch} className="flex gap-3 mb-4">
          <div className="flex-1 relative">
            <Search
              size={16}
              strokeWidth={2}
              className="absolute left-3 top-1/2 -translate-y-1/2 text-neutral-500"
            />
            <Input
              type="text"
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              placeholder="Search projects..."
              className="pl-10 bg-[#141414] border-[#333] text-white placeholder:text-neutral-500 rounded-lg"
            />
          </div>
          <Button type="submit" className="bg-white text-black hover:bg-neutral-200">
            Search
          </Button>
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
                      ? "bg-white text-black"
                      : "bg-[#141414] text-neutral-400 border border-[#333] hover:text-white"
                  }`}
                >
                  {lang}
                </button>
              );
            })}
          </div>

          {/* Sort */}
          <div className="flex items-center gap-2">
            {SORT_OPTIONS.map((opt) => {
              const isActive = filters.sort === opt.value;
              return (
                <button
                  key={opt.value}
                  onClick={() => handleSort(opt.value)}
                  className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${
                    isActive
                      ? "bg-[#222] text-white"
                      : "text-neutral-400 hover:text-white hover:bg-[#141414]"
                  }`}
                >
                  <opt.icon 
                    size={14} 
                    strokeWidth={2} 
                    fill={isActive ? "currentColor" : "none"} 
                  />
                  {opt.label}
                </button>
              );
            })}
          </div>
        </div>
      </Card>

      {/* Results */}
      {loading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {[1, 2, 3, 4, 5, 6].map((i) => (
            <Card
              key={i}
              className="p-5 border border-[#222] bg-[#0A0A0A]"
            >
              <div className="flex justify-between items-start mb-4">
                <div className="w-10 h-10 rounded-full bg-[#1A1A1A] animate-pulse" />
                <div className="w-16 h-5 rounded-md bg-[#1A1A1A] animate-pulse" />
              </div>
              <div className="w-3/4 h-5 mb-3 rounded-md bg-[#1A1A1A] animate-pulse" />
              <div className="w-1/2 h-4 mb-6 rounded-md bg-[#1A1A1A] animate-pulse" />
              <div className="flex justify-between items-center">
                <div className="w-16 h-4 rounded-md bg-[#1A1A1A] animate-pulse" />
                <div className="w-20 h-4 rounded-md bg-[#1A1A1A] animate-pulse" />
              </div>
            </Card>
          ))}
        </div>
      ) : projects.length === 0 ? (
        <div className="text-center py-16">
          <div className="w-16 h-16 rounded-2xl bg-[#141414] border border-[#222] flex items-center justify-center mx-auto mb-4">
            <Code2 size={28} strokeWidth={2} className="text-neutral-500" />
          </div>
          <h3 className="text-lg font-medium text-white mb-2">No projects found</h3>
          <p className="text-neutral-400 text-sm max-w-md mx-auto">
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
