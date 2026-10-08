"use client";

import { useCallback, useLayoutEffect, useRef } from "react";

// A result belongs only to the mounted, open profile session that requested it.
export function useEditorRequestSession(open: boolean, keyId: number | undefined) {
  const generation = useRef(0);
  const active = useRef(false);
  useLayoutEffect(() => {
    generation.current += 1;
    active.current = open;
    return () => {
      generation.current += 1;
      active.current = false;
    };
  }, [open, keyId]);

  return useCallback(() => {
    const requestedGeneration = generation.current;
    return () => active.current && requestedGeneration === generation.current;
  }, []);
}
