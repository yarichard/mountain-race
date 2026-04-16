import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./specs",
  timeout: 30_000,
  use: {
    baseURL: process.env.BASE_URL ?? "http://localhost:8003",
    headless: true,
  },
  reporter: "list",
});
