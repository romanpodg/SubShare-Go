"use client";

import { useCallback, useEffect, useState } from "react";
import { keys as keysApi } from "@/lib/api";
import type { KeyCategory } from "@/lib/types";

async function requestCategories() {
  try {
    const response = await keysApi.listCategories();
    return response.categories || [];
  } catch {
    // Preserve existing data when this auxiliary read fails.
    return undefined;
  }
}

export function useKeyListCategoryData() {
  const [keyCategories, setKeyCategories] = useState<KeyCategory[]>([]);
  const applyCategories = useCallback((categories: KeyCategory[] | undefined) => {
    if (categories !== undefined) setKeyCategories(categories);
  }, []);
  const loadKeyCategories = useCallback(async () => {
    applyCategories(await requestCategories());
  }, [applyCategories]);
  useEffect(() => {
    void requestCategories().then(applyCategories);
  }, [applyCategories]);
  return { keyCategories, loadKeyCategories };
}
