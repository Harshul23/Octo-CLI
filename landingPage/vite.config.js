import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import fs from "fs";
import path from "path";
import { fileURLToPath } from "url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// https://vite.dev/config/
export default defineConfig({
  darkMode: "class",
  plugins: [
    react(),
    tailwindcss(),
    // Copy index.html → 404.html after build so static hosts (e.g. GitHub Pages)
    // serve the SPA shell for every route instead of returning a 404
    {
      name: "spa-fallback",
      closeBundle() {
        const dist = path.resolve(__dirname, "dist");
        const index = path.join(dist, "index.html");
        const notFound = path.join(dist, "404.html");
        if (fs.existsSync(index)) {
          fs.copyFileSync(index, notFound);
        }
      },
    },
  ],
});
