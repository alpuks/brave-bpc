import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { TanStackRouterVite } from "@tanstack/router-plugin/vite";
import tailwindcss from "@tailwindcss/vite";

const base = process.env.VITE_BACKEND_ORIGIN ?? "http://localhost:2727";
// https://vite.dev/config/
export default defineConfig({
  plugins: [
    TanStackRouterVite({
      target: "react",
      autoCodeSplitting: true,
    }),
    react(),
    tailwindcss(),
  ],
  server: {
    port: 3000,

    proxy: {
      "/api": {
        target: base,
        changeOrigin: true,
      },
      "/session": {
        target: base,
        changeOrigin: true,
      },
      "/logout": {
        target: base,
        changeOrigin: true,
      },
    },
  },
  ssr: {
    noExternal: ["framer-motion", "@heroui/react"],
  },
  optimizeDeps: {
    include: ["framer-motion"],
  },
});
