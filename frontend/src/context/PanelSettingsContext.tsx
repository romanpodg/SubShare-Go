"use client";

import { createContext, useContext, useState, useEffect, useRef, useCallback, ReactNode } from "react";
import { usePathname } from "next/navigation";
import { panelSettings as panelSettingsAPI } from "@/lib/api";
import type { PanelSettingsResponse } from "@/lib/types";

export interface PanelSettings {
  panelTitle: string;
  logoDataUrl: string; // base64 data URL or ""
  faviconDataUrl: string; // base64 data URL or ""
  pageTitles: {
    admin: string;
    adminLogin: string;
    subscription: string;
  };
}

const DEFAULT_SETTINGS: PanelSettings = {
  panelTitle: "SubShare",
  logoDataUrl: "",
  faviconDataUrl: "",
  pageTitles: {
    admin: "Панель управления — SubShare",
    adminLogin: "Вход — SubShare",
    subscription: "VPN-подписка — SubShare",
  },
};

const STORAGE_KEY = "subshare_panel_settings";

function fromAPI(api: PanelSettingsResponse): PanelSettings {
  return {
    panelTitle: api.panelTitle || DEFAULT_SETTINGS.panelTitle,
    logoDataUrl: api.logoDataUrl || "",
    faviconDataUrl: api.faviconDataUrl || "",
    pageTitles: {
      admin: api.pageTitleAdmin || DEFAULT_SETTINGS.pageTitles.admin,
      adminLogin: api.pageTitleAdminLogin || DEFAULT_SETTINGS.pageTitles.adminLogin,
      subscription: api.pageTitleSubscription || DEFAULT_SETTINGS.pageTitles.subscription,
    },
  };
}

function toAPI(s: PanelSettings): PanelSettingsResponse {
  return {
    panelTitle: s.panelTitle,
    logoDataUrl: s.logoDataUrl,
    faviconDataUrl: s.faviconDataUrl,
    pageTitleAdmin: s.pageTitles.admin,
    pageTitleAdminLogin: s.pageTitles.adminLogin,
    pageTitleSubscription: s.pageTitles.subscription,
  };
}

function readCache(): PanelSettings {
  if (typeof window === "undefined") return DEFAULT_SETTINGS;
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const saved = JSON.parse(raw) as Partial<PanelSettings>;
      return {
        ...DEFAULT_SETTINGS,
        ...saved,
        pageTitles: { ...DEFAULT_SETTINGS.pageTitles, ...(saved.pageTitles ?? {}) },
      };
    }
  } catch {
    // ignore
  }
  return DEFAULT_SETTINGS;
}

function writeCache(s: PanelSettings) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(s));
  } catch {
    // ignore
  }
}

async function saveToAPI(s: PanelSettings): Promise<PanelSettings> {
  return fromAPI(await panelSettingsAPI.update(toAPI(s)));
}

interface PanelSettingsContextValue {
  settings: PanelSettings;
  loading: boolean;
  updateSettings: (partial: Partial<PanelSettings>) => Promise<void>;
  updatePageTitles: (partial: Partial<PanelSettings["pageTitles"]>) => Promise<void>;
}

const PanelSettingsContext = createContext<PanelSettingsContextValue | null>(null);

export function PanelSettingsProvider({ children }: { children: ReactNode }) {
  // Keep the server render and first client render identical. The backend is
  // the source of truth; localStorage is used only if it cannot be reached.
  const [settings, setSettings] = useState<PanelSettings>(DEFAULT_SETTINGS);
  const [loading, setLoading] = useState(true);
  const pathname = usePathname();
  // Queued partial updates read the latest acknowledged state when they run.
  const latestRef = useRef<PanelSettings>(settings);
  const mutationRevision = useRef(0);
  const saveQueue = useRef<Promise<void>>(Promise.resolve());

  // Применяем favicon при каждом изменении настроек и маршрута.
  // Next.js может заново инжектить <link rel="icon"> при навигации,
  // поэтому важно повторно выставлять URL на каждом pathname.
  usePanelFavicon(settings, pathname);

  // On mount: load from backend (source of truth).
  // Always apply API data — this fixes incognito/new-browser where localStorage is empty.
  useEffect(() => {
    const revision = mutationRevision.current;
    let active = true;
    panelSettingsAPI.get()
      .then((api) => {
        if (!active || mutationRevision.current !== revision) return;
        const apiSettings = fromAPI(api);
        latestRef.current = apiSettings;
        setSettings(apiSettings);
        writeCache(apiSettings);
      })
      .catch(() => {
        if (!active || mutationRevision.current !== revision) return;
        const cached = readCache();
        latestRef.current = cached;
        setSettings(cached);
      })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, []);

  const persistUpdate = useCallback((apply: (current: PanelSettings) => PanelSettings) => {
    const save = saveQueue.current.then(async () => {
      const acknowledged = await saveToAPI(apply(latestRef.current));
      mutationRevision.current += 1;
      latestRef.current = acknowledged;
      writeCache(acknowledged);
      setSettings(acknowledged);
      setLoading(false);
    });
    // A failed caller still receives its rejection; later edits can retry.
    saveQueue.current = save.catch(() => {});
    return save;
  }, []);

  const updateSettings = useCallback((partial: Partial<PanelSettings>) =>
    persistUpdate((current) => ({ ...current, ...partial })), [persistUpdate]);

  const updatePageTitles = useCallback((partial: Partial<PanelSettings["pageTitles"]>) => {
    return persistUpdate((current) => ({ ...current, pageTitles: { ...current.pageTitles, ...partial } }));
  }, [persistUpdate]);

  return (
    <PanelSettingsContext.Provider value={{ settings, loading, updateSettings, updatePageTitles }}>
      {children}
    </PanelSettingsContext.Provider>
  );
}

export function usePanelSettings() {
  const ctx = useContext(PanelSettingsContext);
  if (!ctx) throw new Error("usePanelSettings must be used within PanelSettingsProvider");
  return ctx;
}

function usePanelFavicon(settings: PanelSettings, pathname: string) {
  useEffect(() => { applyPanelFavicon(settings.faviconDataUrl); }, [settings.faviconDataUrl, pathname]);
}

function addPanelIcon(rel: string, url: string) {
  const link = document.createElement("link");
  link.rel = rel;
  link.href = url;
  document.head.appendChild(link);
}

function applyPanelFavicon(url: string) {
  const links = Array.from(document.querySelectorAll<HTMLLinkElement>("link[rel~='icon']"));
  if (!url) {
    links.forEach((link) => link.removeAttribute("href"));
    return;
  }
  if (links.length === 0) {
    addPanelIcon("icon", url);
    addPanelIcon("shortcut icon", url);
    return;
  }
  links.forEach((link) => { link.href = url; });
  if (!links.some((link) => link.rel.toLowerCase() === "shortcut icon")) addPanelIcon("shortcut icon", url);
}
