export type DifficultyFilter = "ALL" | "BEGINNER" | "INTERMEDIATE" | "ADVANCED";

const DIFFICULTIES: DifficultyFilter[] = ["ALL", "BEGINNER", "INTERMEDIATE", "ADVANCED"];

const LABELS: Record<DifficultyFilter, string> = {
  ALL: "All Difficulties",
  BEGINNER: "Beginner",
  INTERMEDIATE: "Intermediate",
  ADVANCED: "Advanced",
};

export interface DifficultyTabsProps {
  value: DifficultyFilter;
  onChange: (value: DifficultyFilter) => void;
}

export function DifficultyTabs({ value, onChange }: DifficultyTabsProps) {
  return (
    <div className="difficulty-tabs">
      {DIFFICULTIES.map((diff) => (
        <button
          key={diff}
          type="button"
          className={`difficulty-tab ${value === diff ? "active" : ""} ${diff.toLowerCase()}`}
          onClick={() => onChange(diff)}
        >
          {LABELS[diff]}
        </button>
      ))}
    </div>
  );
}
