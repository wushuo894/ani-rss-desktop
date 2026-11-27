import '@mdi/font/css/materialdesignicons.css'
import './tray.css'
import {createApp} from 'vue'
import TrayApp from './TrayApp.vue'

// 独立入口只加载面板及已有的服务桥接，避免主窗口导航与样式干扰内容测量。
createApp(TrayApp).mount('#app')
