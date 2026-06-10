import { ReactNode } from "react";

interface CardProps {
  children: ReactNode;
  className?: string;
}

export function Card({ children, className = "" }: CardProps) {
  return (
    <div className={`bg-surface-1 border border-border rounded-2xl p-6 shadow-sm dark:shadow-none transition-shadow ${className}`}>
      {children}
    </div>
  );
}
