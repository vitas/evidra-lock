interface BadgeProps {
  variant: "allow" | "deny" | "low" | "medium" | "high";
  children: React.ReactNode;
}

const variantClass: Record<BadgeProps["variant"], string> = {
  allow: "badge--success",
  deny: "badge--danger",
  low: "badge--success",
  medium: "badge--warning",
  high: "badge--danger",
};

export function Badge({ variant, children }: BadgeProps) {
  return (
    <span className={`badge ${variantClass[variant]}`}>{children}</span>
  );
}
