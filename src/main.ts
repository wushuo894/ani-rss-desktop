import '@mdi/font/css/materialdesignicons.css'
import 'vuetify/styles'
import './style.css'

import {createApp} from 'vue'
import {createVuetify} from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import App from './App.vue'

// 用同一组颜色定义 Vuetify 控件与页面状态。
const vuetify = createVuetify({
    components,
    directives,
    theme: {
        defaultTheme: 'desktop',
        themes: {
            desktop: {
                dark: false,
                colors: {
                    background: '#faf7f2',
                    surface: '#fffdf9',
                    primary: '#a03e25',
                    secondary: '#725e54',
                    success: '#1b785a',
                    error: '#b62e47',
                    warning: '#8b5b0a',
                },
            },
        },
    },
})

createApp(App).use(vuetify).mount('#app')
