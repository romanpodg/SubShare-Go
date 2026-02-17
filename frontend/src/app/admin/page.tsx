"use client";

import { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { useAuth } from "@/hooks/useAuth";
import { useToast } from "@/components/ui/Toast";
import { users as usersApi, keys as keysApi } from "@/lib/api";
import type { User, VLESSKey } from "@/lib/types";
import { UsersSection } from "@/components/admin/UsersSection";
import { KeysSection } from "@/components/admin/KeysSection";
import { GlobalSubscriptionSettingsModal } from "@/components/admin/GlobalSubscriptionSettingsModal";
import { LoadingSpinner } from "@/components/ui/LoadingSpinner";

export default function AdminPage() {
  const router = useRouter();
  const { authenticated, loading: authLoading, logout } = useAuth();
  const { toast } = useToast();
  const [usersList, setUsersList] = useState<User[]>([]);
  const [keysList, setKeysList] = useState<VLESSKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [showGlobalSubscriptionSettings, setShowGlobalSubscriptionSettings] = useState(false);

  useEffect(() => {
    document.title = "Панель управления — Xray Sub";
  }, []);

  const fetchData = useCallback(async () => {
    try {
      const [usersRes, keysRes] = await Promise.all([usersApi.list(), keysApi.list()]);
      setUsersList(usersRes.users);
      setKeysList(keysRes.keys);
    } catch {
      toast("Не удалось загрузить данные", "error");
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
        <LoadingSpinner size="lg" />
      </div>
    );
  }

  if (!authenticated) return null;

  const assignableKeys = keysList.filter((k) => k.status === "active");

  return (
    <div className="min-h-screen">
      <header className="border-b border-border px-6 py-4 flex items-center justify-between">
        <h1 className="text-xl font-bold">Xray Sub</h1>
        <div className="flex items-center gap-4">
          <button
            onClick={() => setShowGlobalSubscriptionSettings(true)}
            className="text-sm text-zinc-400 hover:text-zinc-200 transition-colors"
          >
            Настройки
          </button>
          <Link href="/subscription" className="text-sm text-zinc-400 hover:text-zinc-200 transition-colors">
            Клиентская страница
          </Link>
          <button onClick={handleLogout} className="text-sm text-zinc-400 hover:text-zinc-200 transition-colors">
            Выйти
          </button>
        </div>
      </header>

      <main id="main-content" className="mx-auto w-full max-w-[96rem] p-6 flex flex-col gap-8">
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

      <GlobalSubscriptionSettingsModal
        open={showGlobalSubscriptionSettings}
        onClose={() => setShowGlobalSubscriptionSettings(false)}
      />
    </div>
  );
}
