import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  base: './',
  server: {
    port: 5173,
    host: '127.0.0.1',
    proxy: {
      // 开发模式把 /api 转发到本地 Go 服务
      '/api': 'http://127.0.0.1:45678'
    }
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true
  }
});
