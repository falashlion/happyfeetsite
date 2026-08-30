import { Star } from "lucide-react";

export function Rating({ value, count }: { value: number; count?: number | null }) {
  const rounded = Math.round(value);
  return (
    <div style={{ display: "inline-flex", alignItems: "center", gap: 8 }}>
      <span style={{ display: "inline-flex", gap: 2 }}>
        {[1, 2, 3, 4, 5].map((i) => (
          <Star
            key={i}
            size={14}
            strokeWidth={1.5}
            style={{
              color: i <= rounded ? "var(--hf-gold-600)" : "var(--hf-ink-200)",
              fill: i <= rounded ? "var(--hf-gold-500)" : "transparent",
            }}
          />
        ))}
      </span>
      {count != null && (
        <span style={{ fontSize: 13, color: "var(--fg-secondary)" }}>
          {value.toFixed(1)}{" "}
          <span style={{ color: "var(--fg-muted)" }}>({count})</span>
        </span>
      )}
    </div>
  );
}
