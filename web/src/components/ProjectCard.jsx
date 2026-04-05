import {
  Github,
  Lock,
  Globe,
  GitBranch,
  Check,
  MoreHorizontal,
  AlertCircle,
  Clock,
} from "lucide-react";
import { Card } from "./ui/Card.jsx";
import { useMemo } from "react";
import OctoLogo from "./ui/OctoLogo.jsx";

export default function ProjectCard({
  project,
  onClick,
  onDelete,
  showUser = false,
}) {
  const formattedDate = useMemo(() => {
    if (!project?.updated_at) return "Just now";
    const d = new Date(project.updated_at);
    return d.toLocaleDateString("en-US", { month: "short", day: "numeric" });
  }, [project?.updated_at]);

  const commitMsg =
    project.description ||
    "feat: update configuration settings and environment";
  let repoName = "Unknown/Repo";
  if (project.repo_url) {
    const parts = project.repo_url.split("/");
    if (parts.length >= 2) {
      repoName = parts.slice(-2).join("/").replace(".git", "");
    }
  } else if (project.full_name) {
    repoName = project.full_name;
  }

  const branchName = project.branch || project.default_branch || "main";

  // mock status for visual parity with screenshot
  const status = project.is_public !== false ? "success" : "warning";

  return (
    <Card
      onClick={onClick}
      className="p-5 cursor-pointer bg-[#0A0A0A] border border-[#222] hover:border-[#444] transition-all duration-200 group flex flex-col justify-between"
    >
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-4">
          <div className="w-10 h-10 bg-white rounded-full flex items-center justify-center shrink-0">
            <OctoLogo size={24} className="text-black" />
          </div>
          <div className="flex flex-col">
            <h3 className="text-[15px] font-semibold text-white tracking-tight">
              {project.name}
            </h3>
            <span className="text-[13px] text-[#888]">
              {project.name.toLowerCase().replace(/\s+/g, "-")}.vercel.app
            </span>
          </div>
        </div>
        <div className="flex items-center gap-4 text-neutral-400">
          {status === "success" ? (
            <div className="w-[22px] h-[22px] rounded-full bg-neutral-800/80 border border-[#333] flex items-center justify-center">
              <Check strokeWidth={3} size={10} className="text-neutral-400" />
            </div>
          ) : (
            <div className="w-[22px] h-[22px] rounded-full bg-neutral-800/80 border border-[#333] flex items-center justify-center">
              <span className="text-neutral-400 text-xs font-bold font-mono">
                !
              </span>
            </div>
          )}
          <button
            onClick={(e) => {
              e.stopPropagation();
              // maybe open menu, for now just optional delete
              if (onDelete) onDelete();
            }}
            className="hover:text-white transition-colors"
          >
            <MoreHorizontal size={18} />
          </button>
        </div>
      </div>

      <div className="mt-5 mb-3">
        <div className="inline-flex items-center gap-1.5 px-2.5 py-1 bg-[#111] border border-[#222] rounded-full text-xs font-medium text-neutral-300 mb-3">
          <Github fill="currentColor" size={12} />
          {repoName}
        </div>
        <p className="text-[13.5px] text-[#EAEAEA] font-medium tracking-wide line-clamp-1">
          {commitMsg}
        </p>
      </div>

      <div className="flex items-center gap-1.5 text-[12.5px] text-[#888] font-medium">
        <span>{formattedDate} on</span>
        <GitBranch size={13} className="ml-0.5" />
        <span>{branchName}</span>
      </div>
    </Card>
  );
}
