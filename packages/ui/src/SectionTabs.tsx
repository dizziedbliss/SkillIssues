export interface SectionTabItem {
  key: string;
  label: string;
  count: number;
}

export interface SectionTabsProps {
  items: SectionTabItem[];
  active: string;
  onChange: (key: string) => void;
}

// The top-level Explore/Recommended/Saved/Working/Submissions nav, shared
// by both clients so the active-tab class can't drift out of sync with
// state the way desktop's `activeTab` currently does (D3c).
export function SectionTabs({ items, active, onChange }: SectionTabsProps) {
  return (
    <nav className="web-tabs">
      {items.map((item) => (
        <button
          key={item.key}
          type="button"
          className={active === item.key ? "active" : ""}
          onClick={() => onChange(item.key)}
        >
          {item.label} <b>{item.count}</b>
        </button>
      ))}
    </nav>
  );
}
