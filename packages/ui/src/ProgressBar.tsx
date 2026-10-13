export interface ProgressBarProps {
  percent: number; // 0-100
  label?: string;
}

export function ProgressBar({ percent, label }: ProgressBarProps) {
  const clamped = Math.max(0, Math.min(100, percent));
  return (
    <div className="ui-progress-bar" role="progressbar" aria-valuenow={clamped} aria-valuemin={0} aria-valuemax={100}>
      <div className="ui-progress-bar-track">
        <div className="ui-progress-bar-fill" style={{ width: `${clamped}%` }} />
      </div>
      {label && <span className="ui-progress-bar-label">{label}</span>}
    </div>
  );
}
