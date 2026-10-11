import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, apiV1, users, setCsrfToken, getCsrfToken, setUnauthorizedHandler } from "./api";

afterEach(() => { setCsrfToken(null); setUnauthorizedHandler(null); vi.unstubAllGlobals(); });

function respond(body: BodyInit | null, init: ResponseInit) {
  const fetch = vi.fn().mockResolvedValue(new Response(body, init));
  vi.stubGlobal("fetch", fetch);
  return fetch;
}

describe("existing request/error boundary prerequisite", () => {
  it("keeps GET same-origin credentials and excludes the mutation CSRF header", async () => {
    setCsrfToken("fixture-token");
    const fetch = respond('{"data":[],"meta":{}}', { status: 200 });
    await apiV1.users();
    expect(fetch).toHaveBeenCalledWith(expect.any(String), { method: "GET", headers: {}, body: undefined, credentials: "same-origin" });
  });

  it("sends current CSRF and exact JSON bytes on a mutation", async () => {
    setCsrfToken("fixture-token");
    const fetch = respond('{"message":"subscription updated"}', { status: 200 });
    await users.updateSubscription(1, { subscription_name: null });
    expect(fetch).toHaveBeenCalledWith("/api/v1/users/1/subscription", { method: "PATCH", headers: { "Content-Type": "application/json", "X-CSRF-Token": "fixture-token" }, body: '{"subscription_name":null}', credentials: "same-origin" });
  });

  for (const body of ['{"code":"unauthorized","message":"sign in","request_id":"fixture-request"}', "", "<invalid>"]) {
    it("clears CSRF and notifies unauthorized state for 401 " + JSON.stringify(body), async () => {
      setCsrfToken("fixture-token");
      const unauthorized = vi.fn();
      setUnauthorizedHandler(unauthorized);
      respond(body, { status: 401 });
      await expect(apiV1.user(1)).rejects.toMatchObject({ status: 401 });
      expect(getCsrfToken()).toBeNull();
      expect(unauthorized).toHaveBeenCalledOnce();
    });
  }

  it("retains denied fields/request ID and leaves authentication intact", async () => {
    setCsrfToken("fixture-token");
    const unauthorized = vi.fn();
    setUnauthorizedHandler(unauthorized);
    respond(JSON.stringify({ code: "owner_only", message: "owner required", error: "compatibility text", field_errors: { panelTitle: ["denied"] }, request_id: "fixture-request" }), { status: 403 });
    const result = users.delete(1);
    await expect(result).rejects.toBeInstanceOf(ApiError);
    await expect(result).rejects.toMatchObject({ status: 403, code: "owner_only", message: "owner required", fieldErrors: { panelTitle: ["denied"] }, field_errors: { panelTitle: ["denied"] }, requestId: "fixture-request", request_id: "fixture-request" });
    expect(getCsrfToken()).toBe("fixture-token");
    expect(unauthorized).not.toHaveBeenCalled();
  });

  it("uses header request ID and status text for a malformed error body", async () => {
    respond("<invalid>", { status: 502, statusText: "Bad Gateway", headers: { "X-Request-ID": "header-request" } });
    await expect(apiV1.user(1)).rejects.toMatchObject({ status: 502, code: "http_502", message: "Bad Gateway", fieldErrors: {}, requestId: "header-request" });
  });

  it("propagates network rejection and retains authentication", async () => {
    const error = new TypeError("fixture network failure");
    setCsrfToken("fixture-token");
    const unauthorized = vi.fn();
    setUnauthorizedHandler(unauthorized);
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(error));
    await expect(apiV1.user(1)).rejects.toBe(error);
    expect(getCsrfToken()).toBe("fixture-token");
    expect(unauthorized).not.toHaveBeenCalled();
  });

  it("rejects malformed successful JSON with the response request ID", async () => {
    respond("{invalid", { status: 200, headers: { "X-Request-ID": "header-request" } });
    await expect(apiV1.user(1)).rejects.toMatchObject({ status: 200, code: "invalid_response", requestId: "header-request" });
  });

  for (const status of [200, 204]) {
    it("allows an empty successful response " + status, async () => {
      respond(null, { status });
      await expect(users.delete(1)).resolves.toBeUndefined();
    });
  }
});
