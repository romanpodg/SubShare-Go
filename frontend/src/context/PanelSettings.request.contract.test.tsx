import React from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { PanelSettingsProvider, usePanelSettings } from "./PanelSettingsContext";
import { setCsrfToken } from "@/lib/api";

vi.mock("next/navigation", () => ({ usePathname: () => "/admin" }));
beforeEach(() => { localStorage.clear(); });
afterEach(() => { cleanup(); localStorage.clear(); setCsrfToken(null); vi.unstubAllGlobals(); });

const wire = { panelTitle: "API panel", logoDataUrl: "", faviconDataUrl: "", pageTitleAdmin: "API admin", pageTitleAdminLogin: "API login", pageTitleSubscription: "API subscription" };
function Consumer() {
  const { settings, updateSettings } = usePanelSettings();
  return <><output data-testid="branding-state">{JSON.stringify(settings)}</output><button onClick={() => updateSettings({ panelTitle: "Edited panel" })}>Edit</button></>;
}
function mount() { render(<PanelSettingsProvider><Consumer /></PanelSettingsProvider>); }

it("loads public branding and maps flat API titles to the existing context", async () => {
  const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify(wire), { status: 200 }));
  vi.stubGlobal("fetch", fetch);
  mount();
  await waitFor(() => expect(screen.getByTestId("branding-state")).toHaveTextContent("API panel"));
  expect(fetch.mock.calls[0][0]).toBe("/api/v1/panel-settings");
  expect(JSON.parse(screen.getByTestId("branding-state").textContent!)).toEqual({ panelTitle: wire.panelTitle, logoDataUrl: "", faviconDataUrl: "", pageTitles: { admin: wire.pageTitleAdmin, adminLogin: wire.pageTitleAdminLogin, subscription: wire.pageTitleSubscription } });
});

it("retains cache fallback when the public GET cannot be loaded", async () => {
  localStorage.setItem("subshare_panel_settings", JSON.stringify({ panelTitle: "Cached panel", pageTitles: { admin: "Cached admin" } }));
  vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("fixture network failure")));
  mount();
  await waitFor(() => expect(screen.getByTestId("branding-state")).toHaveTextContent("Cached panel"));
  const state = JSON.parse(screen.getByTestId("branding-state").textContent!);
  expect(state.pageTitles.admin).toBe("Cached admin");
  expect(state.pageTitles.adminLogin).toBe("Вход — SubShare");
});

it("retains PUT mapping, CSRF and the existing local update while acknowledgment stays R21", async () => {
  const fetch = vi.fn().mockResolvedValueOnce(new Response(JSON.stringify(wire), { status: 200 })).mockResolvedValueOnce(new Response('{"error":"owner required"}', { status: 403 }));
  vi.stubGlobal("fetch", fetch);
  setCsrfToken("fixture-csrf");
  mount();
  await waitFor(() => expect(screen.getByTestId("branding-state")).toHaveTextContent("API panel"));
  fireEvent.click(screen.getByRole("button", { name: "Edit" }));
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
  expect(screen.getByTestId("branding-state")).toHaveTextContent("Edited panel");
  expect(fetch.mock.calls[1]).toEqual(["/api/v1/panel-settings", expect.objectContaining({ method: "PUT", credentials: "same-origin", headers: { "Content-Type": "application/json", "X-CSRF-Token": "fixture-csrf" }, body: JSON.stringify({ ...wire, panelTitle: "Edited panel" }) })]);
});
