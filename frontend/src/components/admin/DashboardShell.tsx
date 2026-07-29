"use client";

import { ComponentType, ReactNode, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import Image from "next/image";
import { usePathname, useRouter } from "next/navigation";
import {
  Activity,
  BookOpenText,
  ChevronLeft,
  ChevronRight,
  FileJson2,
  Gauge,
  KeyRound,
  LogOut,
  Menu,
  Paintbrush,
  RadioTower,
  ScrollText,
  Settings,
  ShieldCheck,
  Users,
  X,
} from "lucide-react";
import { useAuth } from "@/hooks/useAuth";
import { usePanelSettings } from "@/context/PanelSettingsContext";
import { LoadingSpinner } from "@/components/ui/LoadingSpinner";
import { ThemeToggle } from "@/components/ui/ThemeToggle";
import { apiV1 } from "@/lib/api";

interface NavItem {
  href: string;
  label: string;
  icon: ComponentType<{ className?: string }>;
  ownerOnly?: boolean;
}

const navigation: Array<{ label: string; items: NavItem[] }> = [
  {
    label: "Обзор",
    items: [{ href: "/admin/overview", label: "Состояние", icon: Gauge }],
  },
  {
    label: "Управление",
    items: [
      { href: "/admin/users", label: "Пользователи", icon: Users },
      { href: "/admin/keys", label: "Ключи", icon: KeyRound },
      { href: "/admin/sources", label: "Источники", icon: RadioTower, ownerOnly: true },
    ],
  },
  {
    label: "Подписка",
    items: [
      { href: "/admin/templates", label: "Шаблоны", icon: FileJson2 },
      { href: "/admin/response-rules", label: "Правила ответов", icon: Activity },
      { href: "/admin/settings/subscription", label: "Настройки", icon: Settings },
      { href: "/admin/settings/branding", label: "Брендинг", icon: Paintbrush, ownerOnly: true },
    ],
  },
  {
    label: "Система",
    items: [
      { href: "/admin/admins", label: "Администраторы", icon: ShieldCheck, ownerOnly: true },
      { href: "/admin/audit", label: "Журнал действий", icon: ScrollText },
      { href: "/admin/settings/security", label: "Безопасность", icon: BookOpenText, ownerOnly: true },
    ],
  },
];

function isActivePath(pathname: string, href: string) {
  return pathname === href || pathname.startsWith(`${href}/`);
}

export function DashboardShell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { authenticated, role, loading, logout } = useAuth();
  const { settings } = usePanelSettings();
  const [collapsed, setCollapsed] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const [version, setVersion] = useState("dev");

  const isLogin = pathname === "/admin/login" || pathname === "/admin/login/";
  const isOwner = role === "owner";

  useEffect(() => {
    if (!isLogin && !loading && authenticated === false) {
      router.replace("/admin/login");
    }
  }, [authenticated, isLogin, loading, router]);

  useEffect(() => {
    if (!authenticated || isLogin) return;
    apiV1
      .buildInfo()
      .then((data) => setVersion(data.version))
      .catch(() => setVersion("dev"));
  }, [authenticated, isLogin]);

  const crumbs = useMemo(() => {
    const active = navigation
      .flatMap((section) => section.items)
      .find((item) => isActivePath(pathname, item.href));
    return active?.label ?? "Панель";
  }, [pathname]);

  if (isLogin) {
    return children;
  }
  if (loading || authenticated !== true) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-bg">
        <LoadingSpinner />
      </div>
    );
  }

  const handleLogout = async () => {
    await logout();
    router.replace("/admin/login");
  };

  const sidebar = (
    <aside
      className={`flex h-full flex-col border-r border-border bg-[#0b1119]/95 shadow-2xl backdrop-blur ${
        collapsed ? "w-[76px]" : "w-[272px]"
      } transition-[width] duration-200`}
    >
      <div className="flex h-20 items-center gap-3 border-b border-border px-5">
        {settings.logoDataUrl ? (
          <Image src={settings.logoDataUrl} alt="" width={40} height={40} unoptimized className="h-10 w-10 rounded-xl object-contain" />
        ) : (
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border border-cyan-400/25 bg-cyan-400/10 text-cyan-300">
            <Activity className="h-5 w-5" />
          </div>
        )}
        {!collapsed && (
          <div className="min-w-0">
            <div className="truncate text-base font-bold tracking-tight text-zinc-100">
              {settings.panelTitle || "SubShare"}
            </div>
            <div className="text-[11px] uppercase tracking-[0.22em] text-cyan-400/70">
              Subscription Hub
            </div>
          </div>
        )}
      </div>

      <nav className="flex-1 space-y-6 overflow-y-auto px-3 py-5" aria-label="Основная навигация">
        {navigation.map((section) => {
          const visible = section.items.filter((item) => !item.ownerOnly || isOwner);
          if (visible.length === 0) return null;
          return (
            <div key={section.label}>
              {!collapsed && (
                <div className="mb-2 px-3 text-[10px] font-semibold uppercase tracking-[0.2em] text-zinc-600">
                  {section.label}
                </div>
              )}
              <div className="space-y-1">
                {visible.map((item) => {
                  const Icon = item.icon;
                  const active = isActivePath(pathname, item.href);
                  return (
                    <Link
                      key={item.href}
                      href={item.href}
                      onClick={() => setMobileOpen(false)}
                      title={collapsed ? item.label : undefined}
                      className={`group flex h-10 items-center rounded-xl border px-3 text-sm transition ${
                        active
                          ? "border-cyan-400/20 bg-cyan-400/10 text-cyan-200"
                          : "border-transparent text-zinc-400 hover:border-zinc-800 hover:bg-zinc-900/70 hover:text-zinc-100"
                      } ${collapsed ? "justify-center" : "gap-3"}`}
                    >
                      <Icon className={`h-[18px] w-[18px] shrink-0 ${active ? "text-cyan-300" : "text-zinc-500 group-hover:text-zinc-300"}`} />
                      {!collapsed && <span className="truncate">{item.label}</span>}
                    </Link>
                  );
                })}
              </div>
            </div>
          );
        })}
      </nav>

      <div className="border-t border-border p-3">
        {!collapsed && (
          <div className="mb-3 rounded-xl border border-border bg-zinc-950/50 px-3 py-2">
            <div className="text-xs text-zinc-500">Сборка</div>
            <div className="mt-0.5 truncate font-mono text-xs text-cyan-300">{version}</div>
          </div>
        )}
        <button
          type="button"
          onClick={() => setCollapsed((value) => !value)}
          className="hidden h-9 w-full items-center justify-center rounded-lg text-zinc-500 transition hover:bg-zinc-900 hover:text-zinc-200 lg:flex"
          aria-label={collapsed ? "Развернуть меню" : "Свернуть меню"}
        >
          {collapsed ? <ChevronRight className="h-4 w-4" /> : <ChevronLeft className="h-4 w-4" />}
        </button>
      </div>
    </aside>
  );

  return (
    <div className="min-h-screen bg-bg text-zinc-100">
      <div className="fixed inset-y-0 left-0 z-40 hidden lg:block">{sidebar}</div>

      {mobileOpen && (
        <div className="fixed inset-0 z-50 lg:hidden">
          <button
            type="button"
            className="absolute inset-0 bg-black/70 backdrop-blur-sm"
            aria-label="Закрыть меню"
            onClick={() => setMobileOpen(false)}
          />
          <div className="relative h-full w-[292px]">
            {sidebar}
            <button
              type="button"
              onClick={() => setMobileOpen(false)}
              className="absolute right-3 top-3 rounded-lg p-2 text-zinc-400 hover:bg-zinc-900 hover:text-white"
              aria-label="Закрыть меню"
            >
              <X className="h-5 w-5" />
            </button>
          </div>
        </div>
      )}

      <div className={`${collapsed ? "lg:pl-[76px]" : "lg:pl-[272px]"} transition-[padding] duration-200`}>
        <header className="sticky top-0 z-30 flex h-16 items-center justify-between border-b border-border bg-[#0b1119]/85 px-4 backdrop-blur-xl sm:px-6">
          <div className="flex items-center gap-3">
            <button
              type="button"
              onClick={() => setMobileOpen(true)}
              className="rounded-lg border border-border bg-surface-1 p-2 text-zinc-400 lg:hidden"
              aria-label="Открыть меню"
            >
              <Menu className="h-5 w-5" />
            </button>
            <div>
              <div className="text-[11px] uppercase tracking-[0.18em] text-zinc-600">SubShare</div>
              <div className="text-sm font-semibold text-zinc-200">{crumbs}</div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <div className="hidden rounded-lg border border-border bg-zinc-950/40 px-3 py-2 text-xs text-zinc-500 sm:block">
              {isOwner ? "Владелец" : "Оператор"}
            </div>
            <ThemeToggle />
            <button
              type="button"
              onClick={handleLogout}
              className="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-border bg-surface-1 text-zinc-500 transition hover:border-rose-500/30 hover:bg-rose-500/10 hover:text-rose-300"
              aria-label="Выйти"
              title="Выйти"
            >
              <LogOut className="h-4 w-4" />
            </button>
          </div>
        </header>
        <main id="main-content" className="mx-auto w-full max-w-[1600px] p-4 sm:p-6 lg:p-8">
          {children}
        </main>
      </div>
    </div>
  );
}
