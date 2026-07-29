import { defineConfig, devices } from "@playwright/test";

const localChannel = process.env.PLAYWRIGHT_CHANNEL === "chrome" ? "chrome" as const : undefined;

export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  fullyParallel: true,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? "github" : "list",
  use: {
    baseURL: "http://127.0.0.1:3100",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  webServer: {
    command: "node node_modules/next/dist/bin/next dev --hostname 127.0.0.1 --port 3100",
    env: { ...process.env, NEXT_DIST_DIR: ".next-e2e" },
    url: "http://127.0.0.1:3100/admin/login",
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
  projects: [
    { name: "desktop-chromium", use: { ...devices["Desktop Chrome"], ...(localChannel ? { channel: localChannel } : {}) } },
    { name: "mobile-chromium", use: { ...devices["Pixel 5"], ...(localChannel ? { channel: localChannel } : {}) } },
  ],
});
