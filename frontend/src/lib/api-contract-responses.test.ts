import { afterEach, describe, expect, it, vi } from "vitest";
import fixture from "../../../testdata/api/contracts.json";
import { apiV1, keys, users, subscriptionSettings, ApiError, setCsrfToken, setUnauthorizedHandler } from "./api";

afterEach(() => { setCsrfToken(null); setUnauthorizedHandler(null); vi.unstubAllGlobals(); });

const calls = {
  "user-list": () => apiV1.users(),
  "user-detail": () => apiV1.user(1),
  "user-full-alias": () => users.list(),
  "users-empty": () => apiV1.users({ query: "absent" }),
  "source-list": () => apiV1.sources.list(),
  "source-detail": () => apiV1.sources.get(1),
  "sources-empty": () => apiV1.sources.list({ query: "absent" }),
  "settings": () => subscriptionSettings.get(),
  "key-detail": () => keys.get(1),
  "key-info": () => keys.get(2),
  "keys-empty": () => keys.listSummaries({ query: "absent" }),
};

describe("shared API responses through the actual request boundary", () => {
  for (const name of Object.keys(calls) as (keyof typeof calls)[]) {
    it(name, async () => {
      const c = fixture.cases[name];
      const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify(c.response), { status: c.status }));
      vi.stubGlobal("fetch", fetch);
      expect(await calls[name]()).toEqual(c.response);
      const [url, options] = fetch.mock.calls[0];
      expect(String(url).replace(/\?$/, "")).toBe(c.path);
      expect(options).toMatchObject({ method: c.method, credentials: "same-origin" });
      expect(options.body).toBeUndefined();
    });
  }

  for (const name of ["patch-omitted", "patch-null", "patch-empty", "patch-status-null", "patch-refresh-max", "patch-refresh-over-max"] as const) {
    it(name, async () => {
      const c = fixture.cases[name];
      const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify(c.response), { status: c.status }));
      vi.stubGlobal("fetch", fetch);
      setCsrfToken("fixture-csrf");
      const result = users.updateSubscription(1, c.body);
      if (c.status === 200) expect(await result).toEqual(c.response);
      else {
        if (!("code" in c.response)) throw new Error("Failed response fixture has no error code");
        await expect(result).rejects.toMatchObject({ status: c.status, code: c.response.code, fieldErrors: {}, requestId: "mutation-fixture" });
      }
      expect(fetch).toHaveBeenCalledWith(c.path, expect.objectContaining({ method: "PATCH", body: JSON.stringify(c.body), credentials: "same-origin", headers: { "Content-Type": "application/json", "X-CSRF-Token": "fixture-csrf" } }));
    });
  }

  it("retains the v1 error envelope and legacy error fallback", async () => {
    for (const name of ["missing-user", "invalid-user"] as const) {
      const c = fixture.cases[name];
      vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify(c.response), { status: c.status })));
      const result = apiV1.user(999);
      await expect(result).rejects.toBeInstanceOf(ApiError);
      await expect(result).rejects.toMatchObject({ status: c.status, code: name === "missing-user" ? "user_not_found" : "http_400", message: c.response.error, field_errors: {}, request_id: name === "missing-user" ? "mutation-fixture" : "" });
    }
  });
});
