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
import { OperationalStatus, SystemLabel } from "@/components/ui/Technical";
import { apiV1 } from "@/lib/api";

interface NavItem {
  href: string;
  label: string;
  code: string;
  icon: ComponentType<{ className?: string }>;
  ownerOnly?: boolean;
}

const navigation: Array<{ label: string; code: string; items: NavItem[] }> = [
  {
    label: "Обзор",
    code: "SYS",
    items: [{ href: "/admin/overview", label: "Состояние", code: "01", icon: Gauge }],
  },
  {
    label: "Управление",
    code: "OPS",
    items: [
      { href: "/admin/users", label: "Пользователи", code: "02", icon: Users },
      { href: "/admin/keys", label: "Ключи", code: "03", icon: KeyRound },
      { href: "/admin/sources", label: "Источники", code: "04", icon: RadioTower, ownerOnly: true },
    ],
  },
  {
    label: "Подписка",
    code: "SUB",
    items: [
      { href: "/admin/templates", label: "Шаблоны", code: "05", icon: FileJson2 },
      { href: "/admin/response-rules", label: "Правила ответов", code: "06", icon: Activity },
      { href: "/admin/settings/subscription", label: "Настройки", code: "07", icon: Settings },
      { href: "/admin/settings/branding", label: "Брендинг", code: "08", icon: Paintbrush, ownerOnly: true },
    ],
  },
  {
    label: "Система",
    code: "ADM",
    items: [
      { href: "/admin/admins", label: "Администраторы", code: "09", icon: ShieldCheck, ownerOnly: true },
      { href: "/admin/audit", label: "Журнал действий", code: "10", icon: ScrollText },
      { href: "/admin/settings/security", label: "Безопасность", code: "11", icon: BookOpenText, ownerOnly: true },
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
    if (!isLogin && !loading && authenticated === false) router.replace("/admin/login");
  }, [authenticated, isLogin, loading, router]);

  useEffect(() => {
    if (!authenticated || isLogin) return;
    apiV1.buildInfo().then((data) => setVersion(data.version)).catch(() => setVersion("dev"));
  }, [authenticated, isLogin]);

  const current = useMemo(
    () => navigation.flatMap((section) => section.items).find((item) => isActivePath(pathname, item.href)),
    [pathname],
  );

  if (isLogin) return children;

  if (loading || authenticated !== true) {
    return (
      <div className="technical-grid flex min-h-screen items-center justify-center bg-bg">
        <div className="technical-frame flex min-h-40 min-w-64 flex-col items-center justify-center gap-4 border border-border bg-surface-1">
          <LoadingSpinner />
          <SystemLabel>Authorizing control plane</SystemLabel>
        </div>
      </div>
    );
  }

  const handleLogout = async () => {
    await logout();
    router.replace("/admin/login");
  };

  const sidebar = (
    <aside
      data-collapsed={collapsed}
      className={`flex h-full flex-col border-r border-border bg-[#070808] ${
        collapsed ? "w-[72px]" : "w-[248px]"
      } transition-[width] duration-200`}
    >
      <div className="flex h-[88px] items-center gap-3 border-b border-border px-4">
        {settings.logoDataUrl ? (
          <Image src={settings.logoDataUrl} alt="" width={38} height={38} unoptimized className="h-[38px] w-[38px] object-contain" />
        ) : (
          <div className="relative flex h-[38px] w-[38px] shrink-0 items-center justify-center border border-accent/30 bg-accent/5 text-accent">
            <Activity className="h-[18px] w-[18px]" />
            <span className="absolute -right-1 -top-1 h-1.5 w-1.5 bg-accent" aria-hidden="true" />
          </div>
        )}
        {!collapsed && (
          <div className="min-w-0">
            <div className="truncate font-display text-[17px] font-medium tracking-[-0.03em] text-zinc-100">
              {settings.panelTitle || "SubShare"}
            </div>
            <SystemLabel className="mt-0.5 block text-accent/70">Subscription infrastructure</SystemLabel>
          </div>
        )}
      </div>

      <nav className="ui-sidebar-nav flex-1 overflow-y-auto py-4" aria-label="Основная навигация">
        {navigation.map((section) => {
          const visible = section.items.filter((item) => !item.ownerOnly || isOwner);
          if (!visible.length) return null;
          return (
            <div key={section.label} className="mb-5">
              <div
                aria-hidden={collapsed}
                className={`mb-2 flex h-3.5 items-center justify-between overflow-hidden ${
                  collapsed ? "invisible px-0" : "px-4"
                }`}
              >
                <SystemLabel>{section.label}</SystemLabel>
                <span className="font-mono text-[9px] text-zinc-700">{section.code}</span>
              </div>
              <div className="border-y border-border/60">
                {visible.map((item, index) => {
                  const Icon = item.icon;
                  const active = isActivePath(pathname, item.href);
                  return (
                    <Link
                      key={item.href}
                      href={item.href}
                      onClick={() => setMobileOpen(false)}
                      title={collapsed ? item.label : undefined}
                      className={`ui-sidebar-nav-item group relative flex min-h-11 items-center border-b border-border/60 px-4 text-sm transition-colors last:border-b-0 ${
                        active
                          ? "bg-accent/[0.075] text-zinc-100"
                          : "text-zinc-500 hover:bg-surface-1 hover:text-zinc-100"
                      } ${collapsed ? "justify-center px-0" : "gap-3"}`}
                    >
                      {active && <span className="absolute inset-y-0 left-0 w-px bg-accent" aria-hidden="true" />}
                      <Icon className={`h-4 w-4 shrink-0 ${active ? "text-accent" : "text-zinc-600 group-hover:text-zinc-300"}`} />
                      {!collapsed && (
                        <>
                          <span className="min-w-0 flex-1 truncate">{item.label}</span>
                          <span className="font-mono text-[9px] text-zinc-700">{item.code}.{index + 1}</span>
                        </>
                      )}
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
          <div className="mb-3 border border-border bg-surface-1 px-3 py-3">
            <div className="flex items-center justify-between gap-3">
              <SystemLabel>Build</SystemLabel>
              <OperationalStatus label="Connected" />
            </div>
            <div className="mt-2 truncate font-mono text-[10px] text-zinc-400">{version}</div>
          </div>
        )}
        <button
          type="button"
          onClick={() => setCollapsed((value) => !value)}
          className="hidden min-h-11 w-full items-center justify-center border border-transparent text-zinc-600 transition-colors hover:border-border hover:bg-surface-1 hover:text-zinc-200 lg:flex"
          aria-label={collapsed ? "Развернуть меню" : "Свернуть меню"}
        >
          {collapsed ? <ChevronRight className="h-4 w-4" /> : <ChevronLeft className="h-4 w-4" />}
        </button>
      </div>
    </aside>
  );

  return (
    <div className="admin-control-plane min-h-screen bg-bg text-zinc-100">
      <div className="fixed inset-y-0 left-0 z-40 hidden lg:block">{sidebar}</div>

      {mobileOpen && (
        <div className="fixed inset-0 z-50 lg:hidden">
          <button
            type="button"
            className="absolute inset-0 bg-black/85"
            aria-label="Закрыть меню"
            onClick={() => setMobileOpen(false)}
          />
          <div className="relative h-full w-[292px]">
            {sidebar}
            <button
              type="button"
              onClick={() => setMobileOpen(false)}
              className="absolute right-2 top-2 flex h-11 w-11 items-center justify-center border border-border bg-surface-1 text-zinc-500 hover:text-zinc-100"
              aria-label="Закрыть меню"
            >
              <X className="h-5 w-5" />
            </button>
          </div>
        </div>
      )}

      <div className={`${collapsed ? "lg:pl-[72px]" : "lg:pl-[248px]"} transition-[padding] duration-200`}>
        <header className="sticky top-0 z-30 flex h-16 items-center justify-between border-b border-border bg-[#070808] px-4 sm:px-6">
          <div className="flex min-w-0 items-center gap-3">
            <button
              type="button"
              onClick={() => setMobileOpen(true)}
              className="flex h-11 w-11 items-center justify-center border border-border bg-surface-1 text-zinc-400 lg:hidden"
              aria-label="Открыть меню"
            >
              <Menu className="h-5 w-5" />
            </button>
            <div className="min-w-0">
              <SystemLabel className="block">SubShare / control plane</SystemLabel>
              <div className="truncate font-display text-sm font-medium text-zinc-200">{current?.label ?? "Панель"}</div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <div className="hidden min-h-9 items-center gap-2 border border-border bg-surface-1 px-3 sm:flex">
              <span className="h-1.5 w-1.5 bg-accent" aria-hidden="true" />
              <span className="font-mono text-[9px] font-semibold uppercase tracking-[0.12em] text-zinc-500">
                {isOwner ? "OWNER ACCESS" : "OPERATOR ACCESS"}
              </span>
            </div>
            <button
              type="button"
              onClick={handleLogout}
              className="flex h-11 w-11 items-center justify-center border border-border bg-surface-1 text-zinc-500 transition-colors hover:border-danger/30 hover:bg-danger/5 hover:text-danger"
              aria-label="Выйти"
              title="Выйти"
            >
              <LogOut className="h-4 w-4" />
            </button>
          </div>
        </header>
        <main id="main-content" className="mx-auto w-full max-w-[1280px] p-3 sm:p-5 lg:p-6">
          <div className="space-y-px">{children}</div>
        </main>
      </div>
    </div>
  );
}
