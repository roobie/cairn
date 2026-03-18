import { defineConfig } from 'tsdown'

export default defineConfig({
  entry: ['./src/index.ts'],
  format: ['esm', 'cjs'],
  dts: true,
  outExtensions({ format }) {
    if (format === 'cjs') return { js: '.cjs' }
    if (format === 'es') return { js: '.mjs' }
    return {}
  },
})
