import type { KeySummary } from "@/lib/types";
import type { useToast } from "@/components/ui/Toast";
export type KeyListToast = ReturnType<typeof useToast>["toast"];

export function normalizeCategory(value: string | null | undefined): string {
  return (value || "").trim();
}

export function moveArrayItem<T>(items: T[], fromIndex: number, toIndex: number): T[] {
  if (fromIndex < 0 || fromIndex >= items.length) {
    return items;
  }

  const next = [...items];
  const [item] = next.splice(fromIndex, 1);

  let targetIndex = toIndex;
  if (fromIndex < targetIndex) {
    targetIndex -= 1;
  }
  if (targetIndex < 0) {
    targetIndex = 0;
  }
  if (targetIndex > next.length) {
    targetIndex = next.length;
  }

  next.splice(targetIndex, 0, item);
  return next;
}

export interface KeyListFilters { searchQuery: string; statusFilter: "all" | "active" | "inactive" }
function keyMatchesQuery(key: KeySummary, query: string) {
  if (!query) return true;
  const fields = [key.label, key.category, key.external_source_name, key.protocol];
  return fields.some((value) => value?.toLowerCase().includes(query)) || String(key.id).includes(query);
}

function keyMatchesStatus(key: KeySummary, status: KeyListFilters["statusFilter"]) {
  return status === "all" || key.status === status;
}

export function filterKeyList(orderedKeys: KeySummary[], { searchQuery, statusFilter }: KeyListFilters) {
  const query = searchQuery.toLowerCase().trim();
  return orderedKeys.filter((key) => keyMatchesQuery(key, query) && keyMatchesStatus(key, statusFilter));
}

export function keyInactiveInLinkFormat(key: KeySummary, subscriptionFormat: "links" | "xray-json") {
  const jsonProtocols = ["vless", "vmess", "trojan", "legacy"];
  return key.kind !== "informational" && subscriptionFormat === "links" && jsonProtocols.includes(key.protocol || "");
}
