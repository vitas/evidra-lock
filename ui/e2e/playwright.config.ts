import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: ".",
  webServer: {
    command: "npm run build && npm run preview",
    port: 4173,
    reuseExistingServer: !process.env.CI,
  },
  use: {
    baseURL: "http://localhost:4173",
  },
});
