import type { Dispatch, SetStateAction, RefObject } from "react";
import type { KeySummary } from "@/lib/types";
import type { useKeyListDrag } from "./use-key-list-drag";
import type { useKeyListSelection } from "./use-key-list-selection";
import type { useKeyListHealth } from "./use-key-list-health";
import type { useKeyListSingleDeletion } from "./use-key-list-deletion";
import type { useKeyListCreation } from "./use-key-list-creation";

type DragActions = Pick<ReturnType<typeof useKeyListDrag>, "beginKeyDrag" | "dropKeyOnTarget" | "dropKeyAtEnd" | "setDragging">;
export interface KeyListCardContext extends DragActions {
  subscriptionFormat: "links" | "xray-json";
  selectedKeyIDs: number[];
  dragging: { id: number } | null;
  reordering: boolean;
  keyIndexByID: Map<number, number>;
  toggleKeySelection: ReturnType<typeof useKeyListSelection>["toggleKeySelection"];
  handleCheck: ReturnType<typeof useKeyListHealth>["handleCheck"];
  handleDelete: ReturnType<typeof useKeyListSingleDeletion>["handleDelete"];
  setEditKey: Dispatch<SetStateAction<KeySummary | null>>;
}
export interface KeyListRowsContext extends KeyListCardContext {
  orderedKeys: KeySummary[];
  informationalHidden: boolean;
  realHidden: boolean;
  startAddConfiguration: ReturnType<typeof useKeyListCreation>["startAddConfiguration"];
}
export interface KeyListCategoryContext extends KeyListRowsContext {
  hiddenCategories: Record<string, boolean>;
  categoryCounts: Map<string, number>;
  getCategoryColor: (name: string) => string;
  allCategoryNames: string[];
  categoryBlockRefs: RefObject<Map<string, HTMLDivElement>>;
  moveCategoryBlock: (name: string, direction: -1 | 1) => Promise<void>;
  openCategoryEditor: (name: string) => void;
  setHiddenCategories: Dispatch<SetStateAction<Record<string, boolean>>>;
}
