import React, { useState } from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { PanelSettingsProvider, usePanelSettings } from "./PanelSettingsContext";
import { PanelSettingsModal } from "@/components/admin/PanelSettingsModal";

vi.mock("next/navigation",()=>({usePathname:()=>"/admin"}));
beforeEach(()=>{localStorage.clear()});
afterEach(()=>{cleanup();localStorage.clear();vi.unstubAllGlobals()});
const wire={panelTitle:"Server panel",logoDataUrl:"",faviconDataUrl:"",pageTitleAdmin:"Server admin",pageTitleAdminLogin:"Server login",pageTitleSubscription:"Server subscription"};
function Consumer(){
  const {settings,updateSettings,updatePageTitles}=usePanelSettings();
  const [open,setOpen]=useState(false);
  return <><output data-testid="agreed-state">{JSON.stringify(settings)}</output><button onClick={()=>{void Promise.resolve(updateSettings({panelTitle:"External title"})).catch(()=>{})}}>External update</button><button onClick={()=>{void Promise.resolve(updatePageTitles({admin:"External admin"})).catch(()=>{})}}>Update page title</button><button onClick={()=>setOpen(true)}>Open branding</button><PanelSettingsModal open={open} onClose={()=>setOpen(false)}/></>;
}
function mount(){render(<PanelSettingsProvider><Consumer/></PanelSettingsProvider>)}
function state(){return JSON.parse(screen.getByTestId("agreed-state").textContent!)}

it("cold-cache public defaults remain the backend truth",async()=>{
  const defaults={panelTitle:"SubShare",logoDataUrl:"",faviconDataUrl:"",pageTitleAdmin:"Панель управления — SubShare",pageTitleAdminLogin:"Вход — SubShare",pageTitleSubscription:"VPN-подписка — SubShare"};
  const fetch=vi.fn().mockResolvedValue(new Response(JSON.stringify(defaults),{status:200}));vi.stubGlobal("fetch",fetch);mount();
  await waitFor(()=>expect(localStorage.getItem("subshare_panel_settings")).not.toBeNull());
  expect(state().panelTitle).toBe("SubShare");expect(fetch).toHaveBeenCalledTimes(1);
});

it("failed public loading preserves fallback cache and missing defaults",async()=>{
  localStorage.setItem("subshare_panel_settings",JSON.stringify({panelTitle:"Cached title",pageTitles:{admin:"Cached admin"}}));
  vi.stubGlobal("fetch",vi.fn().mockRejectedValue(new TypeError("fixture GET failure")));mount();
  await waitFor(()=>expect(state().panelTitle).toBe("Cached title"));
  expect(state().pageTitles.admin).toBe("Cached admin");expect(state().pageTitles.adminLogin).toBe("Вход — SubShare");
});

it("a reopened editor takes a later acknowledged external update",async()=>{
  vi.stubGlobal("fetch",vi.fn().mockResolvedValueOnce(new Response(JSON.stringify(wire),{status:200})).mockResolvedValueOnce(new Response(JSON.stringify({...wire,panelTitle:"External title"}),{status:200})));
  mount();await waitFor(()=>expect(state().panelTitle).toBe("Server panel"));
  fireEvent.click(screen.getByRole("button",{name:"External update"}));await waitFor(()=>expect(state().panelTitle).toBe("External title"));
  fireEvent.click(screen.getByRole("button",{name:"Open branding"}));expect(screen.getByLabelText("Заголовок панели")).toHaveValue("External title");
});

it("a late failed GET cannot replace an acknowledged save with cached data",async()=>{
  let rejectGet!:(error:Error)=>void;const pending=new Promise<Response>((_,reject)=>{rejectGet=reject});
  const fetch=vi.fn().mockReturnValueOnce(pending).mockResolvedValueOnce(new Response(JSON.stringify({...wire,panelTitle:"External title"}),{status:200}));vi.stubGlobal("fetch",fetch);mount();
  fireEvent.click(screen.getByRole("button",{name:"External update"}));await waitFor(()=>expect(state().panelTitle).toBe("External title"));
  localStorage.setItem("subshare_panel_settings",JSON.stringify({panelTitle:"Old cache"}));
  await act(async()=>{rejectGet(new Error("late GET failure"));await pending.catch(()=>{})});
  expect(state().panelTitle).toBe("External title");
});

it("sequential title/page-title mutations preserve both acknowledged changes",async()=>{
  const fetch=vi.fn().mockResolvedValueOnce(new Response(JSON.stringify(wire),{status:200})).mockImplementation(async(_url:string,options:RequestInit)=>new Response(String(options.body),{status:200}));
  vi.stubGlobal("fetch",fetch);mount();await waitFor(()=>expect(state().panelTitle).toBe("Server panel"));
  fireEvent.click(screen.getByRole("button",{name:"External update"}));fireEvent.click(screen.getByRole("button",{name:"Update page title"}));
  await waitFor(()=>expect(state().pageTitles.admin).toBe("External admin"));
  expect(state().panelTitle).toBe("External title");expect(JSON.parse(String(fetch.mock.calls[2][1]?.body))).toMatchObject({panelTitle:"External title",pageTitleAdmin:"External admin"});
});

it("failed queued edits do not poison the next partial update",async()=>{
  const fetch=vi.fn().mockResolvedValueOnce(new Response(JSON.stringify(wire),{status:200})).mockRejectedValueOnce(new TypeError("fixture failure")).mockImplementation(async(_url:string,options:RequestInit)=>new Response(String(options.body),{status:200}));
  vi.stubGlobal("fetch",fetch);mount();await waitFor(()=>expect(state().panelTitle).toBe("Server panel"));
  fireEvent.click(screen.getByRole("button",{name:"External update"}));fireEvent.click(screen.getByRole("button",{name:"Update page title"}));
  await waitFor(()=>expect(state().pageTitles.admin).toBe("External admin"));expect(state().panelTitle).toBe("Server panel");
  expect(JSON.parse(String(fetch.mock.calls[2][1]?.body))).toMatchObject({panelTitle:"Server panel",pageTitleAdmin:"External admin"});
});

it("an initial load can establish server data after a failed concurrent edit",async()=>{
  let resolveGet!:(value:Response)=>void;const pending=new Promise<Response>(resolve=>{resolveGet=resolve});
  const fetch=vi.fn().mockReturnValueOnce(pending).mockRejectedValueOnce(new TypeError("fixture mutation failure"));vi.stubGlobal("fetch",fetch);mount();
  fireEvent.click(screen.getByRole("button",{name:"External update"}));await waitFor(()=>expect(fetch).toHaveBeenCalledTimes(2));
  await act(async()=>{resolveGet(new Response(JSON.stringify(wire),{status:200}));await pending});
  expect(state().panelTitle).toBe("Server panel");expect(state().pageTitles.admin).toBe("Server admin");
});
