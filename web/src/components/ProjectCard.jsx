import { Globe, Lock, Star, Clock, Trash2 } from "lucide-react";
import { Card } from "./ui/Card.jsx";

const LANG_COLORS = {
  Node: "bg-foreground/60",
  JavaScript: "bg-foreground/50",
  Python: "bg-foreground/40",
  Go: "bg-foreground/60",
  Java: "bg-foreground/50",
  Rust: "bg-foreground/40",
  Ruby: "bg-foreground/50",
  HTML: "bg-foreground/40",
};

export default function ProjectCard({
  project,
  onClick,
  onDelete,
  showUser = false,
}) {
  const langColor = LANG_COLORS[project.language] || "bg-muted-foreground";

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

  return (
    <Card
      onClick={onClick}
      className="p-5 cursor-pointer hover:border-foreground/20 transition-all duration-200 group"
    >
      <div className="flex items-start justify-between gap-3 mb-3">
        <div className="flex items-center gap-2 min-w-0">
          {project.language && (
            <div className={`w-3 h-3 rounded-full ${langColor} shrink-0`} />
          )}
          <h3 className="text-sm font-semibold truncate group-hover:text-foreground transition-colors">
            {project.name}
          </h3>
        </div>
        <div className="flex items-center gap-1.5 shrink-0">
          {project.is_public ? (
            <Globe size={12} className="text-foreground/60" />
          ) : (
            <Lock size={12} className="text-muted-foreground" />
          )}
          {onDelete && (
            <button
              onClick={(e) => {
                e.stopPropagation();
                onDelete();
              }}
              className="p-1 text-muted-foreground hover:text-destructive transition-colors opacity-0 group-hover:opacity-100"
            >
              <Trash2 size={12} />
            </button>
          )}
        </div>
      </div>

      {project.description && (
        <p className="text-xs text-muted-foreground mb-3 line-clamp-2">
          {project.description}
        </p>
      )}

      <div className="flex flex-wrap items-center gap-2 text-[11px] text-muted-foreground">
        {project.language && (
          <span className="px-1.5 py-0.5 bg-muted rounded">
            {project.language}
          </span>
        )}
        {project.blueprint?.package_manager && (
          <span className="px-1.5 py-0.5 bg-muted rounded">
            {project.blueprint.package_manager}
          </span>
        )}
        {project.stars > 0 && (
          <span className="flex items-center gap-0.5">
            <Star size={10} /> {project.stars}
          </span>
        )}
        {showUser && project.user_name && (
          <span className="flex items-center gap-0.5">
            @{project.user_name}
          </span>
        )}
        {project.updated_at && (
          <span className="flex items-center gap-0.5 ml-auto">
            <Clock size={10} /> {timeAgo(project.updated_at)}
          </span>
        )}
      </div>
    </Card>
  );
}
