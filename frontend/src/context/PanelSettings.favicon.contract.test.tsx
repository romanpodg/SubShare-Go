import React from "react";
import { afterEach, expect, it, vi } from "vitest";
import { cleanup, render, waitFor } from "@testing-library/react";
import { PanelSettingsProvider } from "./PanelSettingsContext";

vi.mock("next/navigation",()=>({usePathname:()=>"/admin"}));
afterEach(()=>{cleanup();document.querySelectorAll("link[rel~='icon']").forEach(node=>node.remove());localStorage.clear();vi.unstubAllGlobals()});
const wire={panelTitle:"Server",logoDataUrl:"",faviconDataUrl:"data:image/png;base64,Zml4dHVyZQ==",pageTitleAdmin:"Admin",pageTitleAdminLogin:"Login",pageTitleSubscription:"Subscription"};
function mount(){vi.stubGlobal("fetch",vi.fn().mockResolvedValue(new Response(JSON.stringify(wire),{status:200})));render(<PanelSettingsProvider><div /></PanelSettingsProvider>)}

it("keeps normal and shortcut icon publication when no icons exist",async()=>{
  document.querySelectorAll("link[rel~='icon']").forEach(node=>node.remove());mount();
  await waitFor(()=>expect(document.querySelectorAll("link[rel~='icon']")).toHaveLength(2));
  document.querySelectorAll<HTMLLinkElement>("link[rel~='icon']").forEach(link=>expect(link.getAttribute("href")).toBe(wire.faviconDataUrl));
});

it("updates existing icon aliases without duplicating a shortcut",async()=>{
  document.querySelectorAll("link[rel~='icon']").forEach(node=>node.remove());
  for(const rel of ["icon","SHORTCUT ICON"]){const link=document.createElement("link");link.rel=rel;link.href="/old.png";document.head.appendChild(link)}
  mount();await waitFor(()=>expect(document.querySelector<HTMLLinkElement>("link[rel='icon']")?.getAttribute("href")).toBe(wire.faviconDataUrl));
  expect(document.querySelectorAll("link[rel~='icon']")).toHaveLength(2);
});
