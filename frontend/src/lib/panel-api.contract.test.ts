import { afterEach, expect, it, vi } from "vitest";
import fixture from "../../../testdata/api/contracts.json";
import { panelSettings, ApiError, setCsrfToken, getCsrfToken, setUnauthorizedHandler } from "./api";

const wire = fixture.cases["panel-settings"].response;
const calls = { GET: () => panelSettings.get(), PUT: () => panelSettings.update(wire) };
afterEach(() => { setCsrfToken(null); setUnauthorizedHandler(null); vi.unstubAllGlobals(); });

it("loads the public shared branding response through GET without mutation CSRF", async () => {
  setCsrfToken("fixture-csrf");
  const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify(wire), { status: 200 }));
  vi.stubGlobal("fetch", fetch);
  expect(await panelSettings.get()).toEqual(wire);
  expect(fetch).toHaveBeenCalledWith(fixture.cases["panel-settings"].path, { method: "GET", headers: {}, body: undefined, credentials: "same-origin" });
});

it("saves exact flat branding JSON with credentials and current CSRF", async () => {
  setCsrfToken("fixture-csrf");
  const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify(wire), { status: 200 }));
  vi.stubGlobal("fetch", fetch);
  expect(await panelSettings.update(wire)).toEqual(wire);
  expect(fetch).toHaveBeenCalledWith(fixture.cases["panel-settings"].path, { method: "PUT", headers: { "Content-Type": "application/json", "X-CSRF-Token": "fixture-csrf" }, body: JSON.stringify(wire), credentials: "same-origin" });
});

for (const method of ["GET", "PUT"] as const) {
  it(method + " keeps canonical 401 clearing and notification", async () => {
    setCsrfToken("fixture-csrf");
    const unauthorized = vi.fn();
    setUnauthorizedHandler(unauthorized);
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response('{"code":"unauthorized","message":"sign in"}', { status: 401 })));
    await expect(calls[method]()).rejects.toMatchObject({ status: 401, code: "unauthorized" });
    expect(getCsrfToken()).toBeNull();
    expect(unauthorized).toHaveBeenCalledOnce();
  });

  it(method + " propagates network failure through the existing boundary", async () => {
    const error = new TypeError("fixture network failure");
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(error));
    await expect(calls[method]()).rejects.toBe(error);
  });
}

it("preserves denied PUT fields/request ID and authentication", async () => {
  setCsrfToken("fixture-csrf");
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response('{"code":"owner_only","message":"owner required","field_errors":{"panelTitle":["denied"]},"request_id":"fixture-request"}', { status: 403 })));
  const result = panelSettings.update(wire);
  await expect(result).rejects.toBeInstanceOf(ApiError);
  await expect(result).rejects.toMatchObject({ code: "owner_only", status: 403, fieldErrors: { panelTitle: ["denied"] }, requestId: "fixture-request" });
  expect(getCsrfToken()).toBe("fixture-csrf");
});

it("keeps invalid successful JSON as a structured response error", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("{invalid", { status: 200, headers: { "X-Request-ID": "fixture-request" } })));
  await expect(panelSettings.get()).rejects.toMatchObject({ status: 200, code: "invalid_response", requestId: "fixture-request" });
});
