import { useEffect, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useProjectStore, useEnvVarStore } from "../store/index.js";
import api from "../services/api.js";
import {
  ArrowLeft,
  Globe,
  Lock,
  Copy,
  Check,
  ExternalLink,
  Trash2,
  Save,
  Terminal,
  Edit3,
  Loader2,
} from "lucide-react";
import { motion } from "framer-motion";
import BlueprintEditor from "../components/BlueprintEditor.jsx";
import CommandPipeline from "../components/CommandPipeline.jsx";
import EnvManager from "../components/EnvManager.jsx";
import { Button } from "../components/ui/Button.jsx";
import { Card, CardContent } from "../components/ui/Card.jsx";
import { Badge } from "../components/ui/Badge.jsx";
import {
  Tabs,
  TabsList,
  TabsTrigger,
  TabsContent,
} from "../components/ui/Tabs.jsx";

export default function ProjectDetail() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { isAuthenticated } = useAuth();
  const {
    currentProject,
    loading,
    fetchProject,
    updateProject,
    updateBlueprint,
    deleteProject,
  } = useProjectStore();
  const { envVars, fetchEnvVars, saveEnvVars } = useEnvVarStore();

  const [copied, setCopied] = useState(false);
  const [editing, setEditing] = useState(false);
  const [editedBlueprint, setEditedBlueprint] = useState(null);
  const [editedSteps, setEditedSteps] = useState([]);
  const [editedEnvVars, setEditedEnvVars] = useState([]);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (isAuthenticated) {
      fetchProject(id);
      fetchEnvVars(id);
    }
  }, [id, isAuthenticated]);

  const project = currentProject;

  useEffect(() => {
    if (project?.blueprint) {
      setEditedSteps(project.blueprint.steps || []);
    }
  }, [project]);

  useEffect(() => {
    setEditedEnvVars(
      envVars.map((v) => ({
        key: v.key,
        value: v.value,
        isSecret: v.is_secret,
        required: v.required,
      })),
    );
  }, [envVars]);

  const handleCopyCommand = () => {
    const cmd = `octo init --from=${window.location.origin}/api/explore/${id}/blueprint`;
    navigator.clipboard.writeText(cmd);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleToggleVisibility = async () => {
    await updateProject(id, { is_public: !project.is_public });
  };

  const handleSave = async () => {
    if (!editedBlueprint && !editing) return;
    setSaving(true);
    try {
      const bp = editedBlueprint || project.blueprint;
      const merged = { ...bp, steps: editedSteps };
      await updateBlueprint(id, merged);
      await saveEnvVars(id, editedEnvVars);
      setEditing(false);
    } catch (err) {
      console.error("Failed to save:", err);
    }
    setSaving(false);
  };

  const handleDelete = async () => {
    if (window.confirm("Delete this project permanently?")) {
      await deleteProject(id);
      navigate("/dashboard");
    }
  };

  if (loading || !project) {
    return (
      <div className="p-6 lg:p-8 max-w-4xl mx-auto">
        <div className="animate-pulse space-y-4">
          <div className="h-8 bg-muted rounded w-48" />
          <div className="h-4 bg-muted rounded w-96" />
          <div className="h-64 bg-muted rounded-xl" />
        </div>
      </div>
    );
  }

  return (
    <div className="p-6 lg:p-8 max-w-4xl mx-auto">
      {/* Back button */}
      <button
        onClick={() => navigate("/dashboard")}
        className="flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground mb-6 transition-colors"
      >
        <ArrowLeft size={16} />
        Back to Dashboard
      </button>

      {/* Project header */}
      <Card className="p-6 mb-6">
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <div className="flex items-center gap-3 mb-2">
              <h1 className="text-2xl font-bold truncate">{project.name}</h1>
              <Badge variant={project.is_public ? "default" : "secondary"}>
                {project.is_public ? (
                  <>
                    <Globe size={10} className="mr-1" /> Public
                  </>
                ) : (
                  <>
                    <Lock size={10} className="mr-1" /> Private
                  </>
                )}
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
              {project.blueprint?.package_manager && (
                <Badge variant="outline">
                  {project.blueprint.package_manager}
                </Badge>
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
          <div className="flex items-center gap-2 shrink-0">
            <Button
              variant="ghost"
              size="icon"
              onClick={handleToggleVisibility}
              title={project.is_public ? "Make private" : "Make public"}
            >
              {project.is_public ? <Lock size={16} /> : <Globe size={16} />}
            </Button>
            <Button
              variant="ghost"
              size="icon"
              onClick={handleDelete}
              className="hover:text-destructive"
            >
              <Trash2 size={16} />
            </Button>
          </div>
        </div>
      </Card>

      {/* Quick command */}
      <Card className="p-4 mb-6">
        <div className="flex items-center justify-between gap-4">
          <div className="flex items-center gap-3 min-w-0">
            <Terminal size={16} className="text-foreground shrink-0" />
            <code className="text-sm text-muted-foreground truncate font-mono">
              octo init --from={window.location.origin}/api/explore/{id}
              /blueprint
            </code>
          </div>
          <Button variant="outline" size="sm" onClick={handleCopyCommand}>
            {copied ? (
              <Check size={12} className="mr-1.5" />
            ) : (
              <Copy size={12} className="mr-1.5" />
            )}
            {copied ? "Copied!" : "Copy"}
          </Button>
        </div>
      </Card>

      {/* Edit / Save controls */}
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold">Configuration</h2>
        <div className="flex items-center gap-2">
          {editing ? (
            <>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setEditing(false)}
              >
                Cancel
              </Button>
              <Button size="sm" onClick={handleSave} disabled={saving}>
                {saving ? (
                  <Loader2 size={14} className="mr-1.5 animate-spin" />
                ) : (
                  <Save size={14} className="mr-1.5" />
                )}
                {saving ? "Saving..." : "Save All"}
              </Button>
            </>
          ) : (
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                setEditing(true);
                setEditedBlueprint(project.blueprint);
                setEditedSteps(project.blueprint?.steps || []);
              }}
            >
              <Edit3 size={14} className="mr-1.5" />
              Edit
            </Button>
          )}
        </div>
      </div>

      {/* Tabbed interface */}
      {project.blueprint ? (
        <Tabs defaultValue="blueprint" className="w-full">
          <TabsList className="w-full mb-4">
            <TabsTrigger value="blueprint" className="flex-1">
              Blueprint
            </TabsTrigger>
            <TabsTrigger value="pipeline" className="flex-1">
              Command Pipeline
            </TabsTrigger>
            <TabsTrigger value="env" className="flex-1">
              Environment Variables
            </TabsTrigger>
          </TabsList>

          <TabsContent value="blueprint">
            <BlueprintEditor
              blueprint={editing ? editedBlueprint : project.blueprint}
              readOnly={!editing}
              onChange={setEditedBlueprint}
            />
          </TabsContent>

          <TabsContent value="pipeline">
            <CommandPipeline
              steps={editing ? editedSteps : project.blueprint?.steps || []}
              readOnly={!editing}
              onChange={setEditedSteps}
            />
          </TabsContent>

          <TabsContent value="env">
            <EnvManager
              envVars={editedEnvVars}
              readOnly={!editing}
              onChange={setEditedEnvVars}
            />
          </TabsContent>
        </Tabs>
      ) : (
        <Card className="text-center py-12">
          <p className="text-muted-foreground text-sm">
            No blueprint generated yet.
          </p>
          <Button onClick={() => navigate("/analyze")} className="mt-4">
            Run Analysis
          </Button>
        </Card>
      )}
    </div>
  );
}
