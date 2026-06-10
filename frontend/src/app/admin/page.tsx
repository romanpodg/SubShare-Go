"use client";

import { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { useAuth } from "@/hooks/useAuth";
import { useToast } from "@/components/ui/Toast";
import { users as usersApi, keys as keysApi, subscriptionSettings as subscriptionSettingsApi } from "@/lib/api";
import type { User, VLESSKey } from "@/lib/types";
import { UsersSection } from "@/components/admin/UsersSection";
import { KeysSection } from "@/components/admin/KeysSection";
import { GlobalSubscriptionSettingsModal } from "@/components/admin/GlobalSubscriptionSettingsModal";
import { PanelSettingsModal } from "@/components/admin/PanelSettingsModal";
import { RoutingSettingsModal } from "@/components/admin/RoutingSettingsModal";
import { SubscriptionPageConfigModal } from "@/components/admin/SubscriptionPageConfigModal";
import { LoadingSpinner } from "@/components/ui/LoadingSpinner";
import { EmojiText } from "@/components/ui/EmojiText";
import { ThemeToggle } from "@/components/ui/ThemeToggle";
import { usePanelSettings } from "@/context/PanelSettingsContext";
import { LogOut, Route, Settings, Paintbrush, ArrowRight } from "lucide-react";

export default function AdminPage() {
  const router = useRouter();
  const { authenticated, loading: authLoading, logout } = useAuth();
  const { toast } = useToast();
  const { settings } = usePanelSettings();
  const [usersList, setUsersList] = useState<User[]>([]);
  const [keysList, setKeysList] = useState<VLESSKey[]>([]);
  const [subscriptionFormat, setSubscriptionFormat] = useState<"links" | "xray-json">("links");
  const [loading, setLoading] = useState(true);
  const [showGlobalSubscriptionSettings, setShowGlobalSubscriptionSettings] = useState(false);
  const [showRoutingSettings, setShowRoutingSettings] = useState(false);
  const [showPanelSettings, setShowPanelSettings] = useState(false);
  const [showSubscriptionPageConfig, setShowSubscriptionPageConfig] = useState(false);

  useEffect(() => {
    document.title = settings.pageTitles.admin;
  }, [settings.pageTitles.admin]);

  const fetchData = useCallback(async () => {
    try {
      const [usersRes, keysRes, subscriptionSettingsRes] = await Promise.all([
        usersApi.list(),
        keysApi.list(),
        subscriptionSettingsApi.get(),
      ]);
      setUsersList(usersRes.users);
      setKeysList(keysRes.keys);
      setSubscriptionFormat(subscriptionSettingsRes.subscription_format === "xray-json" ? "xray-json" : "links");
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
  const navItemClass =
    "inline-flex h-9 items-center gap-2 rounded-lg border border-border bg-surface-2 px-3 text-sm font-medium transition-all hover:bg-surface-1 hover:border-accent/50 shadow-sm";
  const navDangerClass =
    "inline-flex h-9 items-center gap-2 rounded-lg border border-danger/25 bg-danger/10 px-3 text-sm font-medium text-danger transition-all hover:bg-danger/20 hover:border-danger/40 shadow-sm";

  return (
    <div className="min-h-screen flex flex-col">
      <header className="sticky top-0 z-50 backdrop-blur-xl bg-bg/85 border-b border-border px-6 py-4 flex items-center justify-between gap-6 shadow-sm dark:shadow-none transition-all">
        <div className="flex min-w-0 flex-1 items-center gap-2">
          {/* Логотип */}
          {settings.logoDataUrl && (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={settings.logoDataUrl}
              alt="Логотип"
              className="h-[1.25rem] w-auto object-contain"
              style={{ imageRendering: "auto" }}
            />
          )}
          {/* Название панели */}
          <h1 className="text-xl font-bold">
            <EmojiText text={settings.panelTitle} />
          </h1>
          {/* Кнопка настроек панели */}
          <button
            onClick={() => setShowPanelSettings(true)}
            title="Настройки панели"
            className="w-9 h-9 flex items-center justify-center rounded-lg border border-border bg-surface-2 hover:bg-surface-1 hover:border-accent/50 transition-all text-base leading-none shadow-sm"
          >
            <Settings className="w-4 h-4" />
          </button>
          <Link href="/subscription" className={`${navItemClass} ml-6 shrink-0`}>
            <span>К клиентской панели</span>
            <ArrowRight className="w-4 h-4 opacity-70" />
          </Link>
        </div>
        <div className="flex shrink-0 items-center gap-3">
          <ThemeToggle />
          <div className="w-px h-6 bg-border mx-1"></div>
          <button
            onClick={() => setShowRoutingSettings(true)}
            className={navItemClass}
          >
            <Route className="w-4 h-4 opacity-70" />
            <span>Роутинг</span>
          </button>
          <button
            onClick={() => setShowGlobalSubscriptionSettings(true)}
            className={navItemClass}
          >
            <Settings className="w-4 h-4 opacity-70" />
            <span>Подписка</span>
          </button>
          <button
            onClick={() => setShowSubscriptionPageConfig(true)}
            className={navItemClass}
          >
            <Paintbrush className="w-4 h-4 opacity-70" />
            <span>Дизайн /sub</span>
          </button>
          <button onClick={handleLogout} className={navDangerClass}>
            <LogOut className="w-4 h-4 opacity-70" />
            <span>Выйти</span>
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
          subscriptionFormat={subscriptionFormat}
          onRefresh={fetchData}
        />
      </main>

      <GlobalSubscriptionSettingsModal
        open={showGlobalSubscriptionSettings}
        onClose={() => setShowGlobalSubscriptionSettings(false)}
        onSaved={fetchData}
      />
      <RoutingSettingsModal
        open={showRoutingSettings}
        onClose={() => setShowRoutingSettings(false)}
      />
      <PanelSettingsModal
        open={showPanelSettings}
        onClose={() => setShowPanelSettings(false)}
      />
      <SubscriptionPageConfigModal
        open={showSubscriptionPageConfig}
        onClose={() => setShowSubscriptionPageConfig(false)}
      />
    </div>
  );
}
