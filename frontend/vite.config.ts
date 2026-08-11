import { configDefaults, defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api/v1": "http://127.0.0.1:30080",
      "/api-healthz": {
        target: "http://127.0.0.1:30080",
        rewrite: () => "/healthz"
      },
      "/api-readyz": {
        target: "http://127.0.0.1:30080",
        rewrite: () => "/readyz"
      }
    }
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    css: true,
    // Playwright's e2e/**/*.spec.ts files use test.describe()/expect() from
    // @playwright/test, which crashes if Vitest's own collector picks them
    // up; extend (rather than replace) Vitest's default exclude list so
    // unrelated defaults like node_modules/dist/.git aren't lost.
    exclude: [...configDefaults.exclude, "e2e/**"]
  }
});
