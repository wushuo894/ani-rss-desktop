<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref, watchEffect} from 'vue'
import {isMyGo, runtime} from 'mygo-runtime'
import {provideDesktopState} from './desktopState'
import JdkView from './views/JdkView.vue'
import LogsView from './views/LogsView.vue'
import OverviewView from './views/OverviewView.vue'
import SettingsView from './views/SettingsView.vue'
import brand from './brand.png'

const systemAppearance = window.matchMedia('(prefers-color-scheme: dark)')
const systemDark = ref(systemAppearance.matches)
// 只有 macOS 页面延伸到标题栏下方，并为原生窗口按钮预留空间。
const immersiveWindow = isMyGo() && runtime().platform === 'darwin'

// 系统外观变化时即时更新跟随系统模式。
function updateSystemAppearance(event: MediaQueryListEvent) {
  systemDark.value = event.matches
}

onMounted(() => systemAppearance.addEventListener('change', updateSystemAppearance))
onUnmounted(() => systemAppearance.removeEventListener('change', updateSystemAppearance))

// 自定义样式与 Vuetify 使用同一外观，根节点变量也能覆盖弹窗。
watchEffect(() => {
  document.documentElement.dataset.appearance = systemDark.value ? 'dark' : 'light'
})

const sections = [
  {id: 'overview', title: '概览', icon: 'mdi-view-dashboard-outline'},
  {id: 'logs', title: '日志', icon: 'mdi-text-box-outline'},
  {id: 'settings', title: '启动设置', icon: 'mdi-tune-variant'},
  {id: 'jdk', title: 'JDK 管理', icon: 'mdi-language-java'},
] as const

const stateLabels: Record<string, string> = {
  stopped: '已停止', starting: '启动中', running: '运行中', stopping: '停止中', failed: '启动失败',
}
const page = ref<(typeof sections)[number]['id']>('overview')
const {status, notification} = provideDesktopState()
const stateLabel = computed(() => stateLabels[status.value?.state ?? 'stopped'])
const progress = computed(() => status.value?.total ? (status.value.received / status.value.total) * 100 : undefined)
</script>

<template>
  <v-app :theme="systemDark ? 'desktopDark' : 'desktop'"
         :class="{'immersive-window': immersiveWindow}">
    <!-- 透明拖动区延伸到原生窗口按钮下方，按钮由系统叠加，不由网页重绘。 -->
    <header v-if="immersiveWindow" class="window-titlebar" aria-label="窗口标题栏"/>
    <div class="desktop">
      <aside class="sidebar">
        <div class="brand">
          <img :src="brand" width="35" height="35" alt=""/>
          <span>ani-rss<small>Desktop</small></span>
        </div>
        <nav aria-label="主导航">
          <v-btn
              v-for="section in sections" :key="section.id" type="button" variant="text"
              :class="{ selected: page === section.id }" :aria-current="page === section.id ? 'page' : undefined"
              :prepend-icon="section.icon" :ripple="false"
              @click="page = section.id"
          >
            {{ section.title }}
          </v-btn>
        </nav>
        <div class="app-status" :class="status?.state" role="status">
          <span class="status-dot" :class="status?.state"></span>
          {{ stateLabel }}
        </div>
      </aside>

      <main class="main">
        <div v-if="status?.operation" class="operation" role="status">
          <span>{{ status.operation }}</span>
          <span v-if="progress !== undefined">{{ Math.round(progress) }}%</span>
          <v-progress-linear :model-value="progress" :indeterminate="progress === undefined" color="primary" rounded/>
        </div>

        <!-- 保留打开过的页面，切换菜单时保留输入和滚动位置。 -->
        <KeepAlive>
          <OverviewView v-if="page === 'overview'" @show-jdks="page = 'jdk'"/>
          <LogsView v-else-if="page === 'logs'"/>
          <SettingsView v-else-if="page === 'settings'" @show-jdks="page = 'jdk'"/>
          <JdkView v-else/>
        </KeepAlive>
      </main>
    </div>

    <v-snackbar v-model="notification.open" :color="notification.color" :timeout="4800">{{
        notification.message
      }}
    </v-snackbar>
  </v-app>
</template>
