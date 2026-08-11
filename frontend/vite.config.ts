import { defineConfig } from "vitest/config";
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
    css: true
  }
});
