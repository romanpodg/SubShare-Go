import React, { useState } from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { PanelSettingsProvider, usePanelSettings } from "./PanelSettingsContext";
import { PanelSettingsModal } from "@/components/admin/PanelSettingsModal";

vi.mock("next/navigation", () => ({ usePathname: () => "/admin" }));
beforeEach(() => { localStorage.clear(); });
afterEach(() => { cleanup(); localStorage.clear(); vi.unstubAllGlobals(); });
const wire = { panelTitle: "Server panel", logoDataUrl: "", faviconDataUrl: "", pageTitleAdmin: "Server admin", pageTitleAdminLogin: "Server login", pageTitleSubscription: "Server subscription" };
function Harness({ onClose }: { onClose: () => void }) {
  const [open, setOpen] = useState(false);
  const { settings } = usePanelSettings();
  return <><output data-testid="agreed-title">{settings.panelTitle}</output><button onClick={() => setOpen(true)}>Open branding</button><PanelSettingsModal open={open} onClose={() => { onClose(); setOpen(false); }} /></>;
}
function mount(onClose = vi.fn()) { render(<PanelSettingsProvider><Harness onClose={onClose} /></PanelSettingsProvider>); return onClose; }
async function openLoadedModal() {
  await waitFor(() => expect(screen.getByTestId("agreed-title")).toHaveTextContent("Server panel"));
  fireEvent.click(screen.getByRole("button", { name: "Open branding" }));
}

it("an editor opened after API loading reflects the agreed backend data", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify(wire), { status: 200 })));
  mount();
  await openLoadedModal();
  expect(screen.getByLabelText("Заголовок панели")).toHaveValue("Server panel");
});

for (const failure of ["denied", "server", "network", "invalid-response"] as const) {
it(failure+" save keeps the editor open and preserves agreed title/cache", async () => {
  const fetch = vi.fn().mockResolvedValueOnce(new Response(JSON.stringify(wire), { status: 200 }));
  if (failure === "network") fetch.mockRejectedValueOnce(new TypeError("fixture network failure"));
  else if (failure === "invalid-response") fetch.mockResolvedValueOnce(new Response("{invalid", { status: 200 }));
  else fetch.mockResolvedValueOnce(new Response('{"code":"save_failed","message":"fixture save failed"}', { status: failure === "denied" ? 403 : 500 }));
  vi.stubGlobal("fetch", fetch);
  const onClose = mount();
  await openLoadedModal();
  fireEvent.change(screen.getByLabelText("Заголовок панели"), { target: { value: "Unsaved title" } });
  fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
  expect(onClose).not.toHaveBeenCalled();
  expect(screen.getByTestId("agreed-title")).toHaveTextContent("Server panel");
  expect(JSON.parse(localStorage.getItem("subshare_panel_settings")!).panelTitle).toBe("Server panel");
  await waitFor(()=>expect(screen.getByRole("alert")).toBeVisible());
});
}

it("cancel and reopen discard the abandoned draft in favor of agreed data", async () => {
  vi.stubGlobal("fetch",vi.fn().mockResolvedValue(new Response(JSON.stringify(wire),{status:200})));
  mount(); await openLoadedModal();
  fireEvent.change(screen.getByLabelText("Заголовок панели"),{target:{value:"Abandoned title"}});
  fireEvent.click(screen.getByRole("button",{name:"Отмена"}));
  fireEvent.click(screen.getByRole("button",{name:"Open branding"}));
  expect(screen.getByLabelText("Заголовок панели")).toHaveValue("Server panel");
});

function SaveConsumer() {
  const {settings,updateSettings}=usePanelSettings();
  return <><output data-testid="race-title">{settings.panelTitle}</output><button onClick={()=>updateSettings({panelTitle:"Saved title"})}>Save title</button></>;
}

it("a late initial GET cannot overwrite a successfully saved title", async () => {
  let resolveGet!: (value:Response)=>void;
  const pendingGet=new Promise<Response>(resolve=>{resolveGet=resolve});
  const fetch=vi.fn().mockReturnValueOnce(pendingGet).mockResolvedValueOnce(new Response(JSON.stringify({...wire,panelTitle:"Saved title"}),{status:200}));
  vi.stubGlobal("fetch",fetch);
  render(<PanelSettingsProvider><SaveConsumer /></PanelSettingsProvider>);
  fireEvent.click(screen.getByRole("button",{name:"Save title"}));
  await waitFor(()=>expect(fetch).toHaveBeenCalledTimes(2));
  await waitFor(()=>expect(screen.getByTestId("race-title")).toHaveTextContent("Saved title"));
  await act(async()=>{resolveGet(new Response(JSON.stringify(wire),{status:200}));await pendingGet});
  expect(screen.getByTestId("race-title")).toHaveTextContent("Saved title");
});

it("a pending save keeps the editor open until remote acknowledgment", async () => {
  let resolvePut!: (value:Response)=>void;
  const pendingPut=new Promise<Response>(resolve=>{resolvePut=resolve});
  const fetch=vi.fn().mockResolvedValueOnce(new Response(JSON.stringify(wire),{status:200})).mockReturnValueOnce(pendingPut);
  vi.stubGlobal("fetch",fetch);
  const onClose=mount();await openLoadedModal();
  fireEvent.change(screen.getByLabelText("Заголовок панели"),{target:{value:"Saved title"}});
  fireEvent.click(screen.getByRole("button",{name:"Сохранить"}));
  await waitFor(()=>expect(fetch).toHaveBeenCalledTimes(2));
  fireEvent.click(screen.getByRole("button",{name:"Сохранить"}));
  expect(fetch).toHaveBeenCalledTimes(2);
  expect(onClose).not.toHaveBeenCalled();
  expect(screen.getByTestId("agreed-title")).toHaveTextContent("Server panel");
  await act(async()=>{resolvePut(new Response(JSON.stringify({...wire,panelTitle:"Saved title"}),{status:200}));await pendingPut});
  await waitFor(()=>expect(onClose).toHaveBeenCalledOnce());
  expect(screen.getByTestId("agreed-title")).toHaveTextContent("Saved title");
});
