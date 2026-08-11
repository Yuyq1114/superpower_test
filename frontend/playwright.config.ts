import { defineConfig, devices } from "@playwright/test";

/**
 * Tests always exercise a real Compose or Kubernetes deployment of the
 * complete stack (frontend + gateway + services), never a Vite dev server,
 * so `webServer` is intentionally omitted. `PLAYWRIGHT_BASE_URL` selects
 * which same-origin entrypoint to hit: `http://127.0.0.1:8088` for Compose
 * (also the default when running `npm run e2e` directly without the
 * variable set) or `http://127.0.0.1:30080` for Kubernetes. `make
 * frontend-e2e` requires the variable explicitly so CI/local runs never
 * silently fall back to a stale default.
 */
const baseURL = process.env.PLAYWRIGHT_BASE_URL ?? "http://127.0.0.1:8088";

export default defineConfig({
  testDir: "./e2e",
  timeout: 60_000,
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure"
  },
  projects: [
    {
      name: "desktop",
      use: { ...devices["Desktop Chrome"], viewport: { width: 1280, height: 800 } }
    },
    {
      name: "mobile",
      // "Pixel 5" (rather than "Desktop Chrome") supplies a real mobile
      // profile -- isMobile/hasTouch flags and a mobile Chrome user agent --
      // while the viewport override keeps the exact 390x844 dimensions this
      // suite's layout assertions are written against.
      use: { ...devices["Pixel 5"], viewport: { width: 390, height: 844 } }
    }
  ]
});
