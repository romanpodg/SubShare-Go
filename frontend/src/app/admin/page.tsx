"use client";

import { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/hooks/useAuth";
import { useToast } from "@/components/ui/Toast";
import { users as usersApi, keys as keysApi } from "@/lib/api";
import type { User, VLESSKey } from "@/lib/types";
import { UsersSection } from "@/components/admin/UsersSection";
import { KeysSection } from "@/components/admin/KeysSection";

export default function AdminPage() {
  const router = useRouter();
  const { authenticated, loading: authLoading, logout } = useAuth();
  const { toast } = useToast();
  const [usersList, setUsersList] = useState<User[]>([]);
  const [keysList, setKeysList] = useState<VLESSKey[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchData = useCallback(async () => {
    try {
      const [usersRes, keysRes] = await Promise.all([usersApi.list(), keysApi.list()]);
      setUsersList(usersRes.users);
      setKeysList(keysRes.keys);
    } catch {
      toast("Failed to load data", "error");
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    if (authLoading) return;
    if (!authenticated) {
      router.push("/admin/login");
      return;
    }
    fetchData();
  }, [authenticated, authLoading, router, fetchData]);

  const handleLogout = async () => {
    await logout();
    router.push("/admin/login");
  };

  if (authLoading || loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <p className="text-zinc-500">Loading...</p>
      </div>
    );
  }

  if (!authenticated) return null;

  const assignableKeys = keysList.filter((k) => k.status === "active");

  return (
    <div className="min-h-screen">
      <header className="border-b border-border px-6 py-4 flex items-center justify-between">
        <h1 className="text-lg font-semibold">Xray Sub</h1>
        <div className="flex items-center gap-4">
          <a href="/subscription" className="text-sm text-zinc-400 hover:text-zinc-200 transition-colors">
            Клиентская страница
          </a>
          <button onClick={handleLogout} className="text-sm text-zinc-400 hover:text-zinc-200 transition-colors">
            Выйти
          </button>
        </div>
      </header>

      <main className="max-w-6xl mx-auto p-6 flex flex-col gap-8">
        <UsersSection
          users={usersList}
          assignableKeys={assignableKeys}
          onRefresh={fetchData}
        />
        <KeysSection
          keys={keysList}
          onRefresh={fetchData}
        />
      </main>
    </div>
  );
}
