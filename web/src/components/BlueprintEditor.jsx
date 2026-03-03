import { useState, useEffect, useMemo } from "react";
import { stringify, parse } from "yaml";
import { Prism as SyntaxHighlighter } from "react-syntax-highlighter";
import { vscDarkPlus } from "react-syntax-highlighter/dist/esm/styles/prism";
import { Copy, Check, FileCode, Code2, SlidersHorizontal } from "lucide-react";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "./ui/Tabs";
import { Button } from "./ui/Button";
import { Input } from "./ui/Input";
import { Label } from "./ui/Label";
import { Switch } from "./ui/Switch";
import { Card } from "./ui/Card";

export default function BlueprintEditor({
  blueprint,
  readOnly = true,
  onChange,
}) {
  const [copied, setCopied] = useState(false);
  const [yamlText, setYamlText] = useState("");
  const [parseError, setParseError] = useState(null);
  const [activeTab, setActiveTab] = useState("visual");

  const yamlString = useMemo(() => {
    try {
      return stringify(blueprint, { indent: 2 });
    } catch {
      return "# Error generating YAML";
    }
  }, [blueprint]);

  useEffect(() => {
    setYamlText(yamlString);
    setParseError(null);
  }, [yamlString]);

  const handleCopy = () => {
    navigator.clipboard.writeText(yamlText);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleYamlChange = (e) => {
    const text = e.target.value;
    setYamlText(text);
    setParseError(null);
    try {
      const parsed = parse(text);
      if (parsed && typeof parsed === "object") {
        onChange?.(parsed);
        setParseError(null);
      }
    } catch (err) {
      setParseError(err.message);
    }
  };

  const handleFieldChange = (field, value) => {
    if (!onChange) return;
    const updated = { ...blueprint, [field]: value };
    onChange(updated);
  };

  return (
    <Card className="overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-[hsl(var(--border))]">
        <div className="flex items-center gap-2">
          <FileCode size={14} className="text-[hsl(var(--muted-foreground))]" />
          <span className="text-xs font-medium text-[hsl(var(--muted-foreground))]">
            .octo.yaml
          </span>
        </div>
        <div className="flex items-center gap-2">
          {!readOnly && (
            <Tabs value={activeTab} onValueChange={setActiveTab}>
              <TabsList className="h-7">
                <TabsTrigger
                  value="visual"
                  className="text-xs px-2 py-0.5 h-6 gap-1"
                >
                  <SlidersHorizontal size={10} />
                  Visual
                </TabsTrigger>
                <TabsTrigger
                  value="yaml"
                  className="text-xs px-2 py-0.5 h-6 gap-1"
                >
                  <Code2 size={10} />
                  YAML
                </TabsTrigger>
              </TabsList>
            </Tabs>
          )}
          <Button
            variant="outline"
            size="sm"
            onClick={handleCopy}
            className="h-7 text-xs gap-1.5"
          >
            {copied ? <Check size={11} /> : <Copy size={11} />}
            {copied ? "Copied" : "Copy"}
          </Button>
        </div>
      </div>

      {/* Content */}
      {readOnly ? (
        <SyntaxHighlighter
          language="yaml"
          style={vscDarkPlus}
          customStyle={{
            margin: 0,
            padding: "16px 20px",
            background: "transparent",
            fontSize: "13px",
            lineHeight: "1.6",
          }}
          showLineNumbers
          lineNumberStyle={{ color: "#4a4a4a", fontSize: "11px" }}
        >
          {yamlText}
        </SyntaxHighlighter>
      ) : activeTab === "visual" ? (
        <VisualEditor blueprint={blueprint} onChange={handleFieldChange} />
      ) : (
        <div className="relative">
          <textarea
            value={yamlText}
            onChange={handleYamlChange}
            spellCheck={false}
            className="w-full min-h-[300px] p-4 bg-transparent text-sm font-mono text-[hsl(var(--foreground))] resize-y focus:outline-none leading-relaxed"
            style={{ tabSize: 2 }}
          />
        </div>
      )}

      {parseError && (
        <div className="px-4 py-2 border-t border-red-500/20 bg-red-500/5">
          <p className="text-xs text-red-400 font-mono">{parseError}</p>
        </div>
      )}
    </Card>
  );
}

function VisualEditor({ blueprint, onChange }) {
  return (
    <div className="p-5 space-y-5">
      {/* Project Info */}
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>Project Name</Label>
          <Input
            value={blueprint?.name || ""}
            onChange={(e) => onChange("name", e.target.value)}
            placeholder="my-project"
          />
        </div>
        <div className="space-y-2">
          <Label>Language</Label>
          <Input
            value={blueprint?.language || ""}
            onChange={(e) => onChange("language", e.target.value)}
            placeholder="Node"
          />
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>Version</Label>
          <Input
            value={blueprint?.version || ""}
            onChange={(e) => onChange("version", e.target.value)}
            placeholder="20.x"
          />
        </div>
        <div className="space-y-2">
          <Label>Package Manager</Label>
          <Input
            value={blueprint?.package_manager || ""}
            onChange={(e) => onChange("package_manager", e.target.value)}
            placeholder="npm"
          />
        </div>
      </div>

      {/* Commands */}
      <div className="space-y-2">
        <Label>Run Command</Label>
        <Input
          value={blueprint?.run || ""}
          onChange={(e) => onChange("run", e.target.value)}
          placeholder="npm run dev"
          className="font-mono text-xs"
        />
      </div>

      <div className="space-y-2">
        <Label>Setup Command</Label>
        <Input
          value={blueprint?.setup || ""}
          onChange={(e) => onChange("setup", e.target.value)}
          placeholder="npm install && npm run build"
          className="font-mono text-xs"
        />
      </div>

      {/* Flags */}
      <div className="flex items-center gap-6 pt-2">
        <div className="flex items-center gap-2">
          <Switch
            checked={blueprint?.setup_required || false}
            onCheckedChange={(val) => onChange("setup_required", val)}
          />
          <Label className="text-xs text-[hsl(var(--muted-foreground))]">
            Setup Required
          </Label>
        </div>
        <div className="flex items-center gap-2">
          <Switch
            checked={blueprint?.is_monorepo || false}
            onCheckedChange={(val) => onChange("is_monorepo", val)}
          />
          <Label className="text-xs text-[hsl(var(--muted-foreground))]">
            Monorepo
          </Label>
        </div>
      </div>

      {blueprint?.is_monorepo && (
        <div className="space-y-2">
          <Label>Monorepo Root</Label>
          <Input
            value={blueprint?.monorepo_root || ""}
            onChange={(e) => onChange("monorepo_root", e.target.value)}
            placeholder="packages/"
          />
        </div>
      )}
    </div>
  );
}
