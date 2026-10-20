import { writeFileSync } from 'node:fs'
import { defineConfig, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'

// Vite empties dist/ on every build; the Go binary embeds dist/ and needs at least one file to
// compile from a fresh clone, so keep the committed placeholder alive.
const keepDistPlaceholder = (): Plugin => ({
  name: 'keep-dist-placeholder',
  closeBundle: () => writeFileSync('dist/.gitkeep', ''),
})

export default defineConfig({ plugins: [react(), keepDistPlaceholder()] })
