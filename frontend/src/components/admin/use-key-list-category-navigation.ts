"use client";
import { useCallback, useRef } from "react";
export function useKeyListCategoryNavigation() {
const uncategorizedBlockRef = useRef<HTMLDivElement | null>(null);

const categoryBlockRefs = useRef<Map<string, HTMLDivElement>>(new Map());

const scrollToCategory = useCallback((categoryName: string | null) => {
    const target = categoryName
      ? categoryBlockRefs.current.get(categoryName)
      : uncategorizedBlockRef.current;
    if (!target) {
      return;
    }

    const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    target.scrollIntoView({
      behavior: reduceMotion ? "auto" : "smooth",
      block: "start",
    });
    window.requestAnimationFrame(() => target.focus({ preventScroll: true }));
  }, []);
return { uncategorizedBlockRef, categoryBlockRefs, scrollToCategory };
}
