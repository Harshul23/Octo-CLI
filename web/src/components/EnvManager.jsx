import { useState } from "react";
import {
  Plus,
  Trash2,
  Eye,
  EyeOff,
  Upload,
  KeyRound,
  AlertCircle,
} from "lucide-react";
import { Button } from "./ui/Button";
import { Input } from "./ui/Input";
import { Card } from "./ui/Card";
import { Badge } from "./ui/Badge";
import { Label } from "./ui/Label";
import { Switch } from "./ui/Switch";

/**
 * EnvManager — Vercel-style environment variables manager.
 * Key/value table with sensitive toggle, bulk paste, required/optional badge.
 */
export default function EnvManager({
  envVars = [],
  onChange,
  readOnly = false,
}) {
  const [showBulkPaste, setShowBulkPaste] = useState(false);
  const [bulkText, setBulkText] = useState("");
  const [revealedKeys, setRevealedKeys] = useState(new Set());

  const handleAdd = () => {
    const newVar = {
      id: `env-${Date.now()}`,
      key: "",
      value: "",
      is_secret: false,
      required: true,
    };
    onChange([...envVars, newVar]);
  };

  const handleRemove = (id) => {
    onChange(envVars.filter((v) => v.id !== id));
  };

  const handleUpdate = (id, field, value) => {
    onChange(envVars.map((v) => (v.id === id ? { ...v, [field]: value } : v)));
  };

  const toggleReveal = (id) => {
    setRevealedKeys((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const handleBulkPaste = () => {
    if (!bulkText.trim()) return;
    const lines = bulkText
      .split("\n")
      .filter((l) => l.trim() && !l.startsWith("#"));
    const newVars = lines.map((line, i) => {
      const eqIdx = line.indexOf("=");
      const key = eqIdx > 0 ? line.slice(0, eqIdx).trim() : line.trim();
      const value =
        eqIdx > 0
          ? line
              .slice(eqIdx + 1)
              .trim()
              .replace(/^["']|["']$/g, "")
          : "";
      return {
        id: `env-bulk-${Date.now()}-${i}`,
        key,
        value,
        is_secret:
          key.toLowerCase().includes("secret") ||
          key.toLowerCase().includes("password") ||
          key.toLowerCase().includes("key"),
        required: true,
      };
    });
    onChange([...envVars, ...newVars]);
    setBulkText("");
    setShowBulkPaste(false);
  };

  return (
    <div className="space-y-3">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <KeyRound size={16} className="text-[hsl(var(--muted-foreground))]" />
          <h3 className="text-sm font-semibold">Environment Variables</h3>
          <span className="text-xs text-[hsl(var(--muted-foreground))]">
            {envVars.length} variable{envVars.length !== 1 ? "s" : ""}
          </span>
        </div>
        {!readOnly && (
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setShowBulkPaste(!showBulkPaste)}
              className="gap-1.5 text-xs"
            >
              <Upload size={12} />
              Paste .env
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={handleAdd}
              className="gap-1.5 text-xs"
            >
              <Plus size={12} />
              Add Variable
            </Button>
          </div>
        )}
      </div>

      {/* Bulk paste */}
      {showBulkPaste && (
        <Card className="p-4 space-y-3">
          <Label className="text-xs">Paste your .env file contents:</Label>
          <textarea
            value={bulkText}
            onChange={(e) => setBulkText(e.target.value)}
            placeholder={`DATABASE_URL=postgres://...\nAPI_KEY=sk-...\nDEBUG=true`}
            className="w-full h-28 p-3 bg-transparent border border-[hsl(var(--border))] rounded-md text-xs font-mono text-[hsl(var(--foreground))] placeholder:text-[hsl(var(--muted-foreground))] focus:outline-none focus:ring-1 focus:ring-[hsl(var(--ring))] resize-none"
          />
          <div className="flex gap-2">
            <Button size="sm" onClick={handleBulkPaste} className="text-xs">
              Import Variables
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setShowBulkPaste(false)}
              className="text-xs"
            >
              Cancel
            </Button>
          </div>
        </Card>
      )}

      {/* Table */}
      {envVars.length === 0 ? (
        <Card className="p-8 text-center">
          <KeyRound
            size={24}
            className="mx-auto mb-2 text-[hsl(var(--muted-foreground))]"
          />
          <p className="text-sm text-[hsl(var(--muted-foreground))] mb-1">
            No environment variables
          </p>
          <p className="text-xs text-[hsl(var(--muted-foreground))] mb-3">
            Add variables your project needs to run
          </p>
          {!readOnly && (
            <Button
              variant="outline"
              size="sm"
              onClick={handleAdd}
              className="gap-1.5"
            >
              <Plus size={14} />
              Add Variable
            </Button>
          )}
        </Card>
      ) : (
        <Card className="overflow-hidden">
          {/* Table header */}
          <div className="grid grid-cols-[1fr_1fr_auto_auto_auto] gap-2 px-4 py-2.5 border-b border-[hsl(var(--border))] text-xs font-medium text-[hsl(var(--muted-foreground))]">
            <span>Key</span>
            <span>Value</span>
            <span className="text-center w-16">Secret</span>
            <span className="text-center w-20">Required</span>
            {!readOnly && <span className="w-8" />}
          </div>

          {/* Rows */}
          <div className="divide-y divide-[hsl(var(--border))]">
            {envVars.map((envVar) => (
              <div
                key={envVar.id || envVar.key}
                className="grid grid-cols-[1fr_1fr_auto_auto_auto] gap-2 px-4 py-2.5 items-center"
              >
                {readOnly ? (
                  <code className="text-xs font-mono truncate">
                    {envVar.key || envVar.name}
                  </code>
                ) : (
                  <Input
                    value={envVar.key || envVar.name || ""}
                    onChange={(e) =>
                      handleUpdate(envVar.id, "key", e.target.value)
                    }
                    placeholder="KEY"
                    className="h-7 text-xs font-mono"
                  />
                )}

                {readOnly ? (
                  <span className="text-xs text-[hsl(var(--muted-foreground))] truncate">
                    {envVar.is_secret ? "••••••••" : envVar.value || "—"}
                  </span>
                ) : (
                  <div className="relative flex items-center">
                    <Input
                      type={
                        envVar.is_secret && !revealedKeys.has(envVar.id)
                          ? "password"
                          : "text"
                      }
                      value={envVar.value || ""}
                      onChange={(e) =>
                        handleUpdate(envVar.id, "value", e.target.value)
                      }
                      placeholder="value"
                      className="h-7 text-xs font-mono pr-7"
                    />
                    {envVar.is_secret && (
                      <button
                        type="button"
                        onClick={() => toggleReveal(envVar.id)}
                        className="absolute right-1.5 text-[hsl(var(--muted-foreground))] hover:text-[hsl(var(--foreground))]"
                      >
                        {revealedKeys.has(envVar.id) ? (
                          <EyeOff size={12} />
                        ) : (
                          <Eye size={12} />
                        )}
                      </button>
                    )}
                  </div>
                )}

                <div className="w-16 flex justify-center">
                  {readOnly ? (
                    envVar.is_secret ? (
                      <Badge variant="secondary" className="text-[10px]">
                        Secret
                      </Badge>
                    ) : null
                  ) : (
                    <Switch
                      checked={envVar.is_secret || false}
                      onCheckedChange={(val) =>
                        handleUpdate(envVar.id, "is_secret", val)
                      }
                    />
                  )}
                </div>

                <div className="w-20 flex justify-center">
                  {readOnly ? (
                    <Badge
                      variant={envVar.required ? "default" : "secondary"}
                      className="text-[10px]"
                    >
                      {envVar.required ? "Required" : "Optional"}
                    </Badge>
                  ) : (
                    <Switch
                      checked={envVar.required ?? true}
                      onCheckedChange={(val) =>
                        handleUpdate(envVar.id, "required", val)
                      }
                    />
                  )}
                </div>

                {!readOnly && (
                  <button
                    onClick={() => handleRemove(envVar.id)}
                    className="w-8 h-8 flex items-center justify-center text-[hsl(var(--muted-foreground))] hover:text-red-400 transition-colors"
                  >
                    <Trash2 size={14} />
                  </button>
                )}
              </div>
            ))}
          </div>
        </Card>
      )}
    </div>
  );
}
