import { motion } from "framer-motion";
import { Search, FileCode, Shield, CheckCircle2 } from "lucide-react";
import { Card } from "./ui/Card.jsx";

const steps = [
  {
    id: 1,
    label: "Cloning repository",
    icon: Search,
    description: "Fetching code from GitHub...",
  },
  {
    id: 2,
    label: "Analyzing codebase",
    icon: FileCode,
    description: "Detecting language, framework, and dependencies...",
  },
  {
    id: 3,
    label: "Generating blueprint",
    icon: Shield,
    description: "Building .octo.yaml configuration...",
  },
  {
    id: 4,
    label: "Complete",
    icon: CheckCircle2,
    description: "Analysis finished!",
  },
];

export default function AnalysisProgress({ step = 0 }) {
  return (
    <Card className="p-6 mb-6">
      <div className="space-y-4">
        {steps.map((s) => {
          const isActive = step === s.id;
          const isComplete = step > s.id;

          return (
            <div key={s.id} className="flex items-center gap-4">
              <div className="relative">
                <div
                  className={`w-10 h-10 rounded-full flex items-center justify-center transition-colors duration-300 ${
                    isComplete
                      ? "bg-foreground/10 text-foreground"
                      : isActive
                        ? "bg-foreground/5 text-foreground animate-pulse"
                        : "bg-muted text-muted-foreground/40"
                  }`}
                >
                  {isComplete ? (
                    <CheckCircle2 size={18} />
                  ) : (
                    <s.icon size={18} />
                  )}
                </div>
                {s.id < 4 && (
                  <div
                    className={`absolute top-10 left-1/2 -translate-x-1/2 w-0.5 h-4 ${
                      isComplete ? "bg-foreground/30" : "bg-border"
                    }`}
                  />
                )}
              </div>
              <div className="flex-1 min-w-0">
                <p
                  className={`text-sm font-medium ${
                    isActive || isComplete
                      ? "text-foreground"
                      : "text-muted-foreground"
                  }`}
                >
                  {s.label}
                </p>
                <p className="text-xs text-muted-foreground">{s.description}</p>
              </div>
              {isActive && (
                <motion.div
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  className="w-5 h-5 border-2 border-foreground border-t-transparent rounded-full animate-spin"
                />
              )}
            </div>
          );
        })}
      </div>
    </Card>
  );
}
