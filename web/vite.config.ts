import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The Go BFF serves the built app and exposes /api/*. In dev we proxy /api to it.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/api": { target: "http://localhost:8080", changeOrigin: true },
    },
  },
  build: { outDir: "dist", emptyOutDir: true },
});
