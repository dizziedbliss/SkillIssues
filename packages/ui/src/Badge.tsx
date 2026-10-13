import type { ReactNode } from "react";

export type BadgeVariant = "purple" | "light" | "match" | "accepted" | "rejected" | "review";

export interface BadgeProps {
  variant?: BadgeVariant;
  children: ReactNode;
  icon?: ReactNode;
}

// Thin wrapper over the existing `.pill` class vocabulary both clients
// already style (status-pill accepted/rejected/review, pill purple/light/
// match) — kept as-is per the plan: port the look first, restyle later.
export function Badge({ variant = "light", children, icon }: BadgeProps) {
  const isStatus = variant === "accepted" || variant === "rejected" || variant === "review";
  const className = isStatus ? `pill status-pill ${variant}` : `pill ${variant}`;
  return (
    <span className={className}>
      {icon}
      {children}
    </span>
  );
}
