import { keys as keysApi } from "@/lib/api";
import type { KeyCategory, KeySummary } from "@/lib/types";
import { normalizeCategory, type KeyListToast } from "./key-list-state";

export function keyListCategoryNames(categories: KeyCategory[], keys: KeySummary[]) {
  const names = [...categories.map((category) => normalizeCategory(category.name)), ...keys.map((key) => normalizeCategory(key.category))];
  return [...new Set(names.filter(Boolean))];
}

export function keyListCategoryCounts(keys: KeySummary[]) {
  const counts = new Map<string, number>();
  for (const key of keys) {
    const category = normalizeCategory(key.category);
    if (category) counts.set(category, (counts.get(category) || 0) + 1);
  }
  return counts;
}

export function keyListCategoryMetadata(categories: KeyCategory[]) {
  return new Map(categories.map((category) => [normalizeCategory(category.name), category] as const));
}

export function keyListCategoryGroups(names: string[], keys: KeySummary[]) {
  return names.map((category) => ({ category, keys: keys.filter((key) => normalizeCategory(key.category) === category) }));
}

interface CategoryMove { categoryName: string; direction: -1 | 1; allCategoryNames: string[]; refreshKeysData: () => Promise<void>; toast: KeyListToast }

function nextCategoryOrder(move: CategoryMove) {
  const currentIndex = move.allCategoryNames.findIndex((item) => item === move.categoryName);
  if (currentIndex < 0) return null;
  const targetIndex = currentIndex + move.direction;
  if (targetIndex < 0 || targetIndex >= move.allCategoryNames.length) return null;
  const nextOrder = [...move.allCategoryNames];
  const [item] = nextOrder.splice(currentIndex, 1);
  nextOrder.splice(targetIndex, 0, item);
  return nextOrder;
}

export async function moveKeyListCategoryBlock(move: CategoryMove) {
  const nextOrder = nextCategoryOrder(move);
  if (!nextOrder) return;
  try {
    await keysApi.reorderCategories(nextOrder);
    move.toast(`Категория "${move.categoryName}" перемещена`, "success");
    await move.refreshKeysData();
  } catch (error: unknown) {
    move.toast(error instanceof Error ? error.message : "Не удалось изменить порядок категорий", "error");
  }
}

export async function moveKeyListKeyCategory({ key, nextCategory, refreshKeysData }: { key: KeySummary; nextCategory: string; refreshKeysData: () => Promise<void> }) {
  await keysApi.update(key.id, {
    label: key.label, status: key.status, kind: key.kind, category: nextCategory,
    uuid: "", host: "", port: "", query: "", fragment: "",
    template_text: key.kind === "informational" ? key.template_text : undefined,
  });
  await refreshKeysData();
}
