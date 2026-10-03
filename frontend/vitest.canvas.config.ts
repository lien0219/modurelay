import { resolve } from 'node:path'
import { defineConfig } from 'vitest/config'

const canvasRoot = resolve(__dirname, '../third_party/infinite-canvas/web')

export default defineConfig({
  resolve: {
    alias: {
      '@': resolve(canvasRoot, 'src'),
    },
  },
  test: {
    environment: 'jsdom',
    include: ['canvas-tests/**/*.spec.ts'],
  },
})
