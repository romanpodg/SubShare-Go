"use client";

import {
  createContext,
  ReactNode,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import {
  auth,
  setCsrfToken,
  setUnauthorizedHandler,
} from "@/lib/api";

export type AdminRole = "owner" | "operator" | "viewer";

interface AuthContextValue {
  authenticated: boolean | null;
  role: AdminRole | null;
  loading: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  checkAuth: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [authenticated, setAuthenticated] = useState<boolean | null>(null);
  const [role, setRole] = useState<AdminRole | null>(null);
  const [loading, setLoading] = useState(true);

  const markUnauthorized = useCallback(() => {
    setCsrfToken(null);
    setRole(null);
    setAuthenticated(false);
    setLoading(false);
  }, []);

  const checkAuth = useCallback(async () => {
    setLoading(true);
    try {
      const data = await auth.me();
      setCsrfToken(data.csrf_token);
      setRole(data.role || null);
      setAuthenticated(true);
    } catch {
      markUnauthorized();
    } finally {
      setLoading(false);
    }
  }, [markUnauthorized]);

  useEffect(() => {
    setUnauthorizedHandler(markUnauthorized);
    return () => setUnauthorizedHandler(null);
  }, [markUnauthorized]);

  useEffect(() => {
    void checkAuth();
  }, [checkAuth]);

  const login = useCallback(async (username: string, password: string) => {
    const data = await auth.login(username, password);
    setCsrfToken(data.csrf_token);
    const me = await auth.me();
    setRole(me.role || null);
    setAuthenticated(true);
    setLoading(false);
  }, []);

  const logout = useCallback(async () => {
    try {
      await auth.logout();
    } finally {
      markUnauthorized();
    }
  }, [markUnauthorized]);

  const value = useMemo<AuthContextValue>(() => ({
    authenticated,
    role,
    loading,
    login,
    logout,
    checkAuth,
  }), [authenticated, role, loading, login, logout, checkAuth]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuthContext(): AuthContextValue {
  const value = useContext(AuthContext);
  if (!value) {
    throw new Error("useAuth must be used inside AuthProvider");
  }
  return value;
}
