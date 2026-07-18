import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

export default defineConfig({
  plugins: [vue()],
  build: {
    outDir: path.resolve(__dirname, '../server/web/dist'),
    // 构建前自动清空输出目录，避免旧文件堆积
    emptyOutDir: true,
  },
})
