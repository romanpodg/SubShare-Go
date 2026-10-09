"use client";

import { useCallback, useLayoutEffect, useRef } from "react";

// A result belongs only to the mounted, open profile session that requested it.
interface SessionTransientState {
  setLoading: (value: boolean) => void;
  setRevealing: (value: boolean) => void;
  setShowConflictDialog: (value: boolean) => void;
  setSaving: (value: boolean) => void;
  setCloning: (value: boolean) => void;
}

interface ProfileSessionIdentity {
  open: boolean;
  keyId?: number;
  profileRevision?: number;
}

export function useEditorRequestSession(identity: ProfileSessionIdentity, transient: SessionTransientState) {
  const { open, keyId, profileRevision } = identity;
  const { setLoading, setRevealing, setShowConflictDialog, setSaving, setCloning } = transient;
  const generation = useRef(0);
  const active = useRef(false);
  const loadedRevision = useRef<number | undefined>(profileRevision);
  const resetProfileRequests = useCallback(() => {
    setRevealing(false);
    setShowConflictDialog(false);
  }, [setRevealing, setShowConflictDialog]);
  useLayoutEffect(() => {
    generation.current += 1;
    active.current = open;
    setLoading(false);
    resetProfileRequests();
    setSaving(false);
    setCloning(false);
    return () => {
      generation.current += 1;
      active.current = false;
    };
  }, [open, keyId, setLoading, setSaving, setCloning, resetProfileRequests]);

  useLayoutEffect(() => {
    loadedRevision.current = profileRevision;
    resetProfileRequests();
  }, [profileRevision, resetProfileRequests]);

  return useCallback((scope: "session" | "profile" = "session") => {
    const requestedGeneration = generation.current;
    const requestedRevision = loadedRevision.current;
    const currentSession = () => active.current && requestedGeneration === generation.current;
    const currentProfile = () => scope === "session" || requestedRevision === loadedRevision.current;
    return <Result,>(apply: () => Result): Result | undefined => {
      if (!currentSession()) return;
      if (!currentProfile()) return;
      return apply();
    };
  }, []);
}
