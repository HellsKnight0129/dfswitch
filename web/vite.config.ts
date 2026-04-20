import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Wails embeds ./web/dist into the Go binary via main.go's //go:embed directive.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
