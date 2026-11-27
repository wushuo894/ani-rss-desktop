import vue from '@vitejs/plugin-vue'
import {defineConfig} from 'vite'

export default defineConfig({
    plugins: [vue()],
    server: {host: '127.0.0.1', port: 5173, strictPort: true},
    // 两个 HTML 入口都生成正式资源，托盘页面不依赖开发服务器的回退行为。
    build: {rollupOptions: {input: {main: 'index.html', tray: 'tray.html'}}},
})
