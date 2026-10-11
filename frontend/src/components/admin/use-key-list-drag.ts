"use client";
import { useCallback, useEffect, useRef, useState, type DragEvent as ReactDragEvent } from "react";
import type { KeySummary } from "@/lib/types";
import { dragScrollTargetAt } from "./key-list-drag";
interface KeyDragScrollState { frame: number | null; target: HTMLElement | null; velocity: number }
function startKeyDragScroll(dragAutoScrollRef: { current: KeyDragScrollState }) {
const scrollFrame = () => {
        const current = dragAutoScrollRef.current;
        if (!current.target || current.velocity === 0) {
          current.frame = null;
          return;
        }

        const before = current.target.scrollTop;
        current.target.scrollTop += current.velocity;
        if (current.target.scrollTop === before) {
          current.frame = null;
          current.target = null;
          current.velocity = 0;
          return;
        }
        current.frame = window.requestAnimationFrame(scrollFrame);
      };
return window.requestAnimationFrame(scrollFrame);
}

export function useKeyListDrag(moveKeyToIndex: (id: number, index: number) => void) {
const [dragging, setDragging] = useState<{ id: number } | null>(null);

const dragAutoScrollRef = useRef<{
    frame: number | null;
    target: HTMLElement | null;
    velocity: number;
  }>({ frame: null, target: null, velocity: 0 });

const stopDragAutoScroll = useCallback(() => {
    const state = dragAutoScrollRef.current;
    if (state.frame !== null) {
      window.cancelAnimationFrame(state.frame);
    }
    state.frame = null;
    state.target = null;
    state.velocity = 0;
  }, []);

const updateDragAutoScroll = useCallback(
    (clientX: number, clientY: number) => {
      const decision = dragScrollTargetAt({ clientX, clientY });
      if (!decision) {
        stopDragAutoScroll();
        return;
      }

      const state = dragAutoScrollRef.current;
      state.target = decision.target;
      state.velocity = decision.velocity;
      if (state.frame !== null) {
        return;
      }



      state.frame = startKeyDragScroll(dragAutoScrollRef);
    },
    [stopDragAutoScroll]
  );

useEffect(() => {
    if (!dragging) {
      stopDragAutoScroll();
      return;
    }

    const handleDragOver = (event: DragEvent) => {
      updateDragAutoScroll(event.clientX, event.clientY);
    };
    const handleDragFinished = () => {
      stopDragAutoScroll();
    };

    document.addEventListener("dragover", handleDragOver, true);
    document.addEventListener("drop", handleDragFinished, true);
    document.addEventListener("dragend", handleDragFinished, true);

    return () => {
      document.removeEventListener("dragover", handleDragOver, true);
      document.removeEventListener("drop", handleDragFinished, true);
      document.removeEventListener("dragend", handleDragFinished, true);
      stopDragAutoScroll();
    };
  }, [dragging, stopDragAutoScroll, updateDragAutoScroll]);
const beginKeyDrag = (event: ReactDragEvent, id: number) => {
  setDragging({ id });
  event.dataTransfer.effectAllowed = "move";
  event.dataTransfer.setData("text/plain", String(id));
};
const dropKeyOnTarget = (event: ReactDragEvent, target: { id: number; index: number }) => {
  event.preventDefault();
  event.stopPropagation();
  if (!dragging || dragging.id === target.id) return;
  moveKeyToIndex(dragging.id, target.index);
  setDragging(null);
};
const dropKeyAtEnd = (event: ReactDragEvent, area: { sectionKeys: KeySummary[]; keyIndexByID: Map<number, number>; totalCount: number }) => {
  event.preventDefault();
  if (!dragging) return;
  const lastKey = area.sectionKeys[area.sectionKeys.length - 1];
  const targetIndex = lastKey ? (area.keyIndexByID.get(lastKey.id) ?? 0) + 1 : area.totalCount;
  moveKeyToIndex(dragging.id, targetIndex);
  setDragging(null);
};
return { dragging, setDragging, beginKeyDrag, dropKeyOnTarget, dropKeyAtEnd };
}
