import { useEffect, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import api from "../services/api.js";
import {
  ArrowLeft,
  Globe,
  Star,
  ExternalLink,
  Copy,
  Check,
  Terminal,
  User,
} from "lucide-react";
import BlueprintEditor from "../components/BlueprintEditor.jsx";
import { Button } from "../components/ui/Button.jsx";
import { Card } from "../components/ui/Card.jsx";
import { Badge } from "../components/ui/Badge.jsx";

export default function ExploreDetail() {
  const { id } = useParams();
  const navigate = useNavigate();
  const [project, setProject] = useState(null);
  const [loading, setLoading] = useState(true);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    api
      .getExploreProject(id)
      .then(setProject)
      .catch(() => navigate("/explore"))
      .finally(() => setLoading(false));
  }, [id]);

  const handleCopy = () => {
    const cmd = `octo init --from=${window.location.origin}/api/explore/${id}/blueprint && octo run`;
    navigator.clipboard.writeText(cmd);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  if (loading) {
    return (
      <div className="p-6 lg:p-8 max-w-4xl mx-auto animate-pulse space-y-4">
        <div className="h-8 bg-muted rounded w-48" />
        <div className="h-4 bg-muted rounded w-96" />
        <div className="h-64 bg-muted rounded-xl" />
      </div>
    );
  }

  if (!project) return null;

  return (
    <div className="p-6 lg:p-8 max-w-4xl mx-auto">
      <button
        onClick={() => navigate("/explore")}
        className="flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground mb-6 transition-colors"
      >
        <ArrowLeft size={16} />
        Back to Explore
      </button>

      {/* Project header */}
      <Card className="p-6 mb-6">
        <div className="flex items-start justify-between gap-4">
          <div>
            <div className="flex items-center gap-3 mb-2">
              <h1 className="text-2xl font-bold">{project.name}</h1>
              <Badge>
                <Globe size={10} className="mr-1" /> Public
              </Badge>
            </div>
            {project.description && (
              <p className="text-sm text-muted-foreground mb-3">
                {project.description}
              </p>
            )}
            <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
              {project.language && (
                <Badge variant="outline">{project.language}</Badge>
              )}
              {project.user_name && (
                <span className="flex items-center gap-1">
                  <User size={12} /> {project.user_name}
                </span>
              )}
              {project.stars > 0 && (
                <span className="flex items-center gap-1">
                  <Star size={12} /> {project.stars}
                </span>
              )}
              {project.repo_url && (
                <a
                  href={project.repo_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center gap-1 hover:text-foreground transition-colors"
                >
                  <ExternalLink size={12} />
                  Repository
                </a>
              )}
            </div>
          </div>
        </div>
      </Card>

      {/* Run command */}
      <Card className="p-4 mb-6">
        <p className="text-xs text-muted-foreground mb-2">
          Run this project locally:
        </p>
        <div className="flex items-center justify-between gap-4">
          <div className="flex items-center gap-3 min-w-0">
            <Terminal size={16} className="text-foreground shrink-0" />
            <code className="text-sm text-muted-foreground truncate font-mono">
              octo init --from={window.location.origin}/api/explore/{id}
              /blueprint && octo run
            </code>
          </div>
          <Button variant="outline" size="sm" onClick={handleCopy}>
            {copied ? (
              <Check size={12} className="mr-1.5" />
            ) : (
              <Copy size={12} className="mr-1.5" />
            )}
            {copied ? "Copied!" : "Copy"}
          </Button>
        </div>
      </Card>

      {/* Blueprint viewer */}
      {project.blueprint && (
        <div>
          <h2 className="text-lg font-semibold mb-4">Blueprint</h2>
          <BlueprintEditor blueprint={project.blueprint} readOnly />
        </div>
      )}
    </div>
  );
}
