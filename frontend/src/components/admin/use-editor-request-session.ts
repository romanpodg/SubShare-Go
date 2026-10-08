"use client";

import { useCallback, useLayoutEffect, useRef } from "react";

// A result belongs only to the mounted, open profile session that requested it.
interface SessionTransientState {
  setLoading: (value: boolean) => void;
  setRevealing: (value: boolean) => void;
  setShowConflictDialog: (value: boolean) => void;
}

export function useEditorRequestSession(open: boolean, keyId: number | undefined, transient: SessionTransientState) {
  const { setLoading, setRevealing, setShowConflictDialog } = transient;
  const generation = useRef(0);
  const active = useRef(false);
  useLayoutEffect(() => {
    generation.current += 1;
    active.current = open;
    setLoading(false);
    setRevealing(false);
    setShowConflictDialog(false);
    return () => {
      generation.current += 1;
      active.current = false;
    };
  }, [open, keyId, setLoading, setRevealing, setShowConflictDialog]);

  return useCallback(() => {
    const requestedGeneration = generation.current;
    return <Result,>(apply: () => Result): Result | undefined => {
      if (active.current && requestedGeneration === generation.current) return apply();
    };
  }, []);
}
