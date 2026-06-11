"use client";

import { createContext, useContext, useState, useEffect, useRef, useCallback, ReactNode } from "react";
import { usePathname } from "next/navigation";
import { getCsrfToken } from "@/lib/api";

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

// Shape returned by GET /api/panel-settings (flat)
interface APIPanelSettings {
  panelTitle: string;
  logoDataUrl: string;
  faviconDataUrl: string;
  pageTitleAdmin: string;
  pageTitleAdminLogin: string;
  pageTitleSubscription: string;
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

function fromAPI(api: APIPanelSettings): PanelSettings {
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

function toAPI(s: PanelSettings): APIPanelSettings {
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

/** Returns true if the settings are purely the built-in defaults (i.e. nothing has been customised). */
function isDefaultSettings(s: PanelSettings): boolean {
  return (
    s.panelTitle === DEFAULT_SETTINGS.panelTitle &&
    !s.logoDataUrl &&
    !s.faviconDataUrl &&
    s.pageTitles.admin === DEFAULT_SETTINGS.pageTitles.admin &&
    s.pageTitles.adminLogin === DEFAULT_SETTINGS.pageTitles.adminLogin &&
    s.pageTitles.subscription === DEFAULT_SETTINGS.pageTitles.subscription
  );
}

async function saveToAPI(s: PanelSettings): Promise<boolean> {
  try {
    const token = getCsrfToken();
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (token) headers["X-CSRF-Token"] = token;
    const r = await fetch("/api/admin/panel-settings", {
      method: "PUT",
      headers,
      credentials: "same-origin",
      body: JSON.stringify(toAPI(s)),
    });
    return r.ok;
  } catch {
    return false;
  }
}

interface PanelSettingsContextValue {
  settings: PanelSettings;
  updateSettings: (partial: Partial<PanelSettings>) => void;
  updatePageTitles: (partial: Partial<PanelSettings["pageTitles"]>) => void;
}

const PanelSettingsContext = createContext<PanelSettingsContextValue | null>(null);

export function PanelSettingsProvider({ children }: { children: ReactNode }) {
  // Start from localStorage cache for instant render, no flash
  const [settings, setSettings] = useState<PanelSettings>(readCache);
  const pathname = usePathname();
  // Keep a mutable ref that is always in sync — lets updateSettings/updatePageTitles
  // read the CURRENT state even when called sequentially in the same event handler.
  const latestRef = useRef<PanelSettings>(settings);

  // Применяем favicon при каждом изменении настроек и маршрута.
  // Next.js может заново инжектить <link rel="icon"> при навигации,
  // поэтому важно повторно выставлять URL на каждом pathname.
  useEffect(() => {
    const links = Array.from(document.querySelectorAll<HTMLLinkElement>("link[rel~='icon']"));

    if (settings.faviconDataUrl) {
      if (links.length === 0) {
        const iconLink = document.createElement("link");
        iconLink.rel = "icon";
        iconLink.href = settings.faviconDataUrl;
        document.head.appendChild(iconLink);

        const shortcutLink = document.createElement("link");
        shortcutLink.rel = "shortcut icon";
        shortcutLink.href = settings.faviconDataUrl;
        document.head.appendChild(shortcutLink);
      } else {
        links.forEach((link) => {
          link.href = settings.faviconDataUrl;
        });

        if (!links.some((link) => link.rel.toLowerCase() === "shortcut icon")) {
          const shortcutLink = document.createElement("link");
          shortcutLink.rel = "shortcut icon";
          shortcutLink.href = settings.faviconDataUrl;
          document.head.appendChild(shortcutLink);
        }
      }
    } else if (links.length > 0) {
      // Favicon was removed — restore browser default by clearing href
      links.forEach((link) => link.removeAttribute("href"));
    }
  }, [settings.faviconDataUrl, pathname]);

  // On mount: load from backend (source of truth).
  // Always apply API data — this fixes incognito/new-browser where localStorage is empty.
  // If the API still has pure defaults but we have richer local cache, push the cache up
  // (migrates users who saved before the backend endpoint existed).
  useEffect(() => {
    fetch("/api/panel-settings")
      .then((r) => (r.ok ? (r.json() as Promise<APIPanelSettings>) : Promise.reject(r.status)))
      .then((api) => {
        const apiSettings = fromAPI(api);

        if (isDefaultSettings(apiSettings) && !isDefaultSettings(latestRef.current)) {
          // API has no saved data yet — migrate local cache to backend
          saveToAPI(latestRef.current);
          // Keep displaying the local cache while migration happens
        } else {
          // API is source of truth — always use it
          latestRef.current = apiSettings;
          setSettings(apiSettings);
          writeCache(apiSettings);
        }
      })
      .catch(() => {
        // Backend unreachable — keep localStorage cache as-is
      });
  }, []);

  const updateSettings = useCallback((partial: Partial<PanelSettings>) => {
    const next = { ...latestRef.current, ...partial };
    latestRef.current = next;
    writeCache(next);
    setSettings(next);
    saveToAPI(next);
  }, []);

  const updatePageTitles = useCallback((partial: Partial<PanelSettings["pageTitles"]>) => {
    const next = {
      ...latestRef.current,
      pageTitles: { ...latestRef.current.pageTitles, ...partial },
    };
    latestRef.current = next;
    writeCache(next);
    setSettings(next);
    saveToAPI(next);
  }, []);

  return (
    <PanelSettingsContext.Provider value={{ settings, updateSettings, updatePageTitles }}>
      {children}
    </PanelSettingsContext.Provider>
  );
}

export function usePanelSettings() {
  const ctx = useContext(PanelSettingsContext);
  if (!ctx) throw new Error("usePanelSettings must be used within PanelSettingsProvider");
  return ctx;
}
