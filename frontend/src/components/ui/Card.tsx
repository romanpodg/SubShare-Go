import { ReactNode } from "react";

interface CardProps {
  children: ReactNode;
  className?: string;
}

export function Card({ children, className = "" }: CardProps) {
  return (
    <div className={`technical-frame border border-border bg-surface-1 p-6 ${className}`}>
      {children}
    </div>
  );
}
