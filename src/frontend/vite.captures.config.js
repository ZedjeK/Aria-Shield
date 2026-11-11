// Compilation de l'interface seule, pour les captures d'écran (captures/) :
// les liaisons Wails (bindings/aria-shield, générées à la compilation de l'exe)
// sont remplacées par un faux service qui renvoie des données fictives.
// Sans effet sur l'exe : build.ps1 n'utilise pas ce fichier.
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

export default defineConfig({
  plugins: [svelte()],
  resolve: {
    alias: [
      {
        find: /^(\.\.\/)+bindings\/aria-shield$/,
        replacement: fileURLToPath(new URL('./captures/mock.js', import.meta.url)),
      },
    ],
  },
  build: { outDir: 'dist-captures', emptyOutDir: true },
  preview: { port: 4173, strictPort: true },
})
