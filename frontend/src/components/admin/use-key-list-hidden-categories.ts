"use client";

import { useState } from "react";

function pruneHiddenCategories(previous: Record<string, boolean>, categoryNames: string[]) {
  const next: Record<string, boolean> = {};
  for (const categoryName of categoryNames) {
    if (previous[categoryName]) next[categoryName] = true;
  }
  return next;
}

export function useKeyListHiddenCategories(categoryNames: string[]) {
  const [hiddenCategories, setHiddenCategories] = useState<Record<string, boolean>>({});
  const [previousNames, setPreviousNames] = useState(categoryNames);
  if (previousNames !== categoryNames) {
    setPreviousNames(categoryNames);
    setHiddenCategories((previous) => pruneHiddenCategories(previous, categoryNames));
  }
  return { hiddenCategories, setHiddenCategories };
}
