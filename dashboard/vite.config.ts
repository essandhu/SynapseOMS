/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { fileURLToPath, URL } from "node:url";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  test: {
    globals: true,
    environment: "jsdom",
    setupFiles: ["./src/test-setup.ts"],
    css: false,
    env: {
      VITE_API_URL: "http://localhost:8080/api/v1",
      VITE_RISK_API_URL: "http://localhost:8081/",
      VITE_WS_URL: "ws://localhost:8080",
    },
  },
  server: {
    port: 3000,
    proxy: {
      "/api/v1": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
      "/risk-api": {
        target: "http://localhost:8081",
        changeOrigin: true,
        rewrite: (path: string) => path.replace(/^\/risk-api/, ""),
      },
      "/ws": {
        target: "ws://localhost:8080",
        ws: true,
      },
    },
  },
});
