import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Dev: the Go backend runs on :8080; proxy REST + WebSocket so the app is same-origin.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/api": "http://localhost:8080",
      "/healthz": "http://localhost:8080",
      "/ws": { target: "ws://localhost:8080", ws: true },
    },
  },
});
