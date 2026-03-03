import { useState } from "react";
import {
  Plus,
  Trash2,
  GripVertical,
  ChevronUp,
  ChevronDown,
  Play,
  Terminal,
} from "lucide-react";
import { Button } from "./ui/Button";
import { Input } from "./ui/Input";
import { Card } from "./ui/Card";

/**
 * CommandPipeline — Visual step-by-step command execution map.
 * Users can add, remove, edit, and reorder steps.
 */
export default function CommandPipeline({
  steps = [],
  onChange,
  readOnly = false,
}) {
  const [editingId, setEditingId] = useState(null);

  const handleAdd = () => {
    const newStep = {
      id: `step-${Date.now()}`,
      name: `Step ${steps.length + 1}`,
      command: "",
      order: steps.length,
    };
    onChange([...steps, newStep]);
    setEditingId(newStep.id);
  };

  const handleRemove = (id) => {
    onChange(
      steps.filter((s) => s.id !== id).map((s, i) => ({ ...s, order: i })),
    );
  };

  const handleUpdate = (id, field, value) => {
    onChange(steps.map((s) => (s.id === id ? { ...s, [field]: value } : s)));
  };

  const handleMove = (index, direction) => {
    const newSteps = [...steps];
    const targetIndex = index + direction;
    if (targetIndex < 0 || targetIndex >= newSteps.length) return;
    [newSteps[index], newSteps[targetIndex]] = [
      newSteps[targetIndex],
      newSteps[index],
    ];
    onChange(newSteps.map((s, i) => ({ ...s, order: i })));
  };

  return (
    <div className="space-y-3">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Terminal size={16} className="text-[hsl(var(--muted-foreground))]" />
          <h3 className="text-sm font-semibold">Command Pipeline</h3>
          <span className="text-xs text-[hsl(var(--muted-foreground))]">
            {steps.length} step{steps.length !== 1 ? "s" : ""}
          </span>
        </div>
        {!readOnly && (
          <Button
            variant="outline"
            size="sm"
            onClick={handleAdd}
            className="gap-1.5 text-xs"
          >
            <Plus size={12} />
            Add Step
          </Button>
        )}
      </div>

      {/* Pipeline */}
      {steps.length === 0 ? (
        <Card className="p-8 text-center">
          <Terminal
            size={24}
            className="mx-auto mb-2 text-[hsl(var(--muted-foreground))]"
          />
          <p className="text-sm text-[hsl(var(--muted-foreground))] mb-3">
            No steps configured yet
          </p>
          {!readOnly && (
            <Button
              variant="outline"
              size="sm"
              onClick={handleAdd}
              className="gap-1.5"
            >
              <Plus size={14} />
              Add First Step
            </Button>
          )}
        </Card>
      ) : (
        <div className="relative">
          {/* Vertical connector line */}
          {steps.length > 1 && (
            <div className="absolute left-5 top-6 bottom-6 w-px bg-[hsl(var(--border))]" />
          )}

          <div className="space-y-2">
            {steps.map((step, index) => (
              <StepCard
                key={step.id}
                step={step}
                index={index}
                total={steps.length}
                isEditing={editingId === step.id}
                readOnly={readOnly}
                onEdit={() =>
                  setEditingId(editingId === step.id ? null : step.id)
                }
                onUpdate={(field, value) => handleUpdate(step.id, field, value)}
                onRemove={() => handleRemove(step.id)}
                onMoveUp={() => handleMove(index, -1)}
                onMoveDown={() => handleMove(index, 1)}
              />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function StepCard({
  step,
  index,
  total,
  isEditing,
  readOnly,
  onEdit,
  onUpdate,
  onRemove,
  onMoveUp,
  onMoveDown,
}) {
  return (
    <Card
      className={`relative transition-all ${
        isEditing ? "ring-1 ring-[hsl(var(--ring))]" : ""
      }`}
    >
      <div className="flex items-center gap-3 p-3">
        {/* Step number indicator */}
        <div className="relative z-10 flex items-center justify-center w-10 h-10 rounded-full bg-[hsl(var(--secondary))] border border-[hsl(var(--border))] text-xs font-bold shrink-0">
          {index + 1}
        </div>

        {/* Step info */}
        <div className="flex-1 min-w-0">
          {isEditing ? (
            <div className="space-y-2">
              <Input
                value={step.name}
                onChange={(e) => onUpdate("name", e.target.value)}
                placeholder="Step name"
                className="h-7 text-xs"
              />
              <Input
                value={step.command}
                onChange={(e) => onUpdate("command", e.target.value)}
                placeholder="e.g. npm install"
                className="h-7 text-xs font-mono"
              />
            </div>
          ) : (
            <div
              className="cursor-pointer"
              onClick={!readOnly ? onEdit : undefined}
            >
              <p className="text-sm font-medium truncate">
                {step.name || `Step ${index + 1}`}
              </p>
              {step.command ? (
                <code className="text-xs text-[hsl(var(--muted-foreground))] font-mono truncate block">
                  $ {step.command}
                </code>
              ) : (
                <p className="text-xs text-[hsl(var(--muted-foreground))] italic">
                  No command set
                </p>
              )}
            </div>
          )}
        </div>

        {/* Actions */}
        {!readOnly && (
          <div className="flex items-center gap-0.5 shrink-0">
            <button
              onClick={onMoveUp}
              disabled={index === 0}
              className="p-1.5 text-[hsl(var(--muted-foreground))] hover:text-[hsl(var(--foreground))] disabled:opacity-30 transition-colors"
            >
              <ChevronUp size={14} />
            </button>
            <button
              onClick={onMoveDown}
              disabled={index === total - 1}
              className="p-1.5 text-[hsl(var(--muted-foreground))] hover:text-[hsl(var(--foreground))] disabled:opacity-30 transition-colors"
            >
              <ChevronDown size={14} />
            </button>
            <button
              onClick={onRemove}
              className="p-1.5 text-[hsl(var(--muted-foreground))] hover:text-red-400 transition-colors"
            >
              <Trash2 size={14} />
            </button>
          </div>
        )}
      </div>
    </Card>
  );
}
