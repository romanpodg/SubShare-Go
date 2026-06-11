"use client";

import { useState, useEffect, useCallback } from "react";
import { auth, setCsrfToken } from "@/lib/api";

export function useAuth() {
  const [authenticated, setAuthenticated] = useState<boolean | null>(null);
  const [role, setRole] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const checkAuth = useCallback(async () => {
    try {
      const data = await auth.me();
      setCsrfToken(data.csrf_token);
      setRole(data.role || null);
      setAuthenticated(true);
    } catch {
      setAuthenticated(false);
      setRole(null);
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
    const meData = await auth.me();
    setRole(meData.role || null);
    setAuthenticated(true);
  };

  const logout = async () => {
    await auth.logout();
    setAuthenticated(false);
    setRole(null);
  };

  return { authenticated, role, loading, login, logout, checkAuth };
}
