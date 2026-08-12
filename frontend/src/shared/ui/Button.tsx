import type { CSSProperties, ReactNode } from "react";

type ButtonVariant = "primary" | "secondary" | "danger";

const baseStyle: CSSProperties = {
  padding: "var(--space-3) var(--space-4)",
  borderRadius: "8px",
  minWidth: "44px",
  minHeight: "44px",
  fontWeight: 600,
  cursor: "pointer"
};

const variantStyles: Record<ButtonVariant, CSSProperties> = {
  primary: { background: "var(--color-accent-strong)", color: "#ffffff", border: "none" },
  secondary: {
    background: "var(--color-surface)",
    color: "var(--color-text)",
    border: "1px solid var(--color-border)"
  },
  danger: { background: "var(--color-danger)", color: "#ffffff", border: "none" }
};

export function Button(props: {
  type?: "button" | "submit";
  variant?: ButtonVariant;
  disabled?: boolean;
  onClick?: () => void;
  children: ReactNode;
}) {
  const { type = "button", variant = "primary", disabled, onClick, children } = props;

  return (
    <button
      type={type}
      disabled={disabled}
      onClick={onClick}
      style={{ ...baseStyle, ...variantStyles[variant], opacity: disabled ? 0.6 : 1 }}
    >
      {children}
    </button>
  );
}
