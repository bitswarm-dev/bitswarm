import type { ReactNode } from "react";

interface Props {
  size?: number;
  children: ReactNode;
  fill?: string;
  strokeWidth?: number;
}

export function Svg({ size = 14, children, fill = "none", strokeWidth = 2 }: Props) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill={fill}
      stroke={fill === "none" ? "currentColor" : undefined}
      strokeWidth={strokeWidth}
      strokeLinejoin="round"
      strokeLinecap="round"
      aria-hidden="true"
    >
      {children}
    </svg>
  );
}

export const Logo = ({ size = 26 }: { size?: number }) => (
  <svg width={size} height={size} viewBox="0 0 26 26" aria-hidden="true">
    <g fill="var(--accent)">
      <circle cx="13" cy="13" r="3.4" />
      <circle cx="4.5" cy="7" r="2.2" opacity=".55" />
      <circle cx="21.5" cy="7" r="2.2" opacity=".75" />
      <circle cx="5" cy="20" r="2.2" opacity=".75" />
      <circle cx="21" cy="20" r="2.2" opacity=".55" />
      <circle cx="13" cy="2.8" r="1.6" opacity=".4" />
    </g>
    <g stroke="var(--accent)" strokeWidth="1.2" opacity=".5">
      <path d="M13 13 4.5 7M13 13 21.5 7M13 13 5 20M13 13 21 20M13 13V2.8" />
    </g>
  </svg>
);
