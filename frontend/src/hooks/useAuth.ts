"use client";

import { useState, useEffect, useCallback } from "react";
import { auth, setCsrfToken } from "@/lib/api";

export function useAuth() {
  const [authenticated, setAuthenticated] = useState<boolean | null>(null);
  const [loading, setLoading] = useState(true);

  const checkAuth = useCallback(async () => {
    try {
      const data = await auth.me();
      setCsrfToken(data.csrf_token);
      setAuthenticated(true);
    } catch {
      setAuthenticated(false);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  const login = async (username: string, password: string) => {
    const data = await auth.login(username, password);
    setCsrfToken(data.csrf_token);
    setAuthenticated(true);
  };

  const logout = async () => {
    await auth.logout();
    setAuthenticated(false);
  };

  return { authenticated, loading, login, logout, checkAuth };
}
