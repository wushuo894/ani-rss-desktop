<script setup lang="ts">
import {computed} from 'vue'
import {Desktop} from './mygo'
import {provideDesktopState} from './desktopState'
import brand from './brand.png'

const {status, busy, working, notification, act} = provideDesktopState()
const labels: Record<string, string> = {
  stopped: '已停止', starting: '正在启动', running: '运行中', stopping: '正在停止', failed: '启动失败',
}
const stateLabel = computed(() => status.value ? labels[status.value.state] ?? '状态未知' : '正在连接')
// 启动、重启和打开地址分别要求明确的服务阶段，首次读取状态前全部禁用。
const canStart = computed(() => !working.value && ['stopped', 'failed'].includes(status.value?.state ?? ''))
const canRestart = computed(() => !working.value && status.value?.state === 'running')
const canStop = computed(() => !working.value && ['starting', 'running'].includes(status.value?.state ?? ''))
const canOpenWeb = computed(() => canRestart.value && Boolean(status.value?.url))
const progress = computed(() => status.value?.total ? Math.min(100, status.value.received / status.value.total * 100) : undefined)
</script>

<template>
  <main class="tray-panel" aria-label="ani-rss 快捷面板">
    <header class="tray-header">
      <img :src="brand" width="36" height="36" alt=""/>
      <div class="tray-brand"><strong>ani-rss</strong><span>Desktop</span></div>
      <button class="icon-button" title="打开主窗口" aria-label="打开主窗口" @click="act(Desktop.showMainWindow)">
        <i class="mdi mdi-open-in-new" aria-hidden="true"/>
      </button>
    </header>

    <section class="tray-service" :class="status?.state" aria-label="服务状态">
      <div class="service-state" role="status"><span class="state-dot"/>{{ stateLabel }}</div>
      <span class="version">{{ status?.version ? 'v' + status.version : '尚未安装服务' }}</span>
      <p class="service-address">{{ status?.state === 'running' ? status.url : '你的本地订阅服务' }}</p>
      <button class="web-button" :disabled="!canOpenWeb" @click="act(() => Desktop.openWeb('ui'))">
        打开 Web UI <i class="mdi mdi-arrow-top-right" aria-hidden="true"/>
      </button>
    </section>

    <div class="service-buttons" aria-label="服务操作">
      <button :disabled="!canStart" @click="act(Desktop.start)"><i class="mdi mdi-play" aria-hidden="true"/>启动</button>
      <button :disabled="!canRestart" @click="act(Desktop.restart)"><i class="mdi mdi-restart" aria-hidden="true"/>重启</button>
      <button :disabled="!canStop" @click="act(Desktop.stop)"><i class="mdi mdi-stop" aria-hidden="true"/>停止</button>
    </div>

    <div v-if="status?.operation" class="tray-progress" role="status">
      <span>{{ status.operation }}</span><span v-if="progress !== undefined">{{ Math.round(progress) }}%</span>
      <progress :value="progress" max="100" aria-label="当前操作进度"/>
    </div>
    <p v-if="notification.open" class="tray-message" role="alert">{{ notification.message }}</p>
    <p v-else-if="status?.message" class="tray-message" role="alert">{{ status.message }}</p>

    <nav class="tray-links" aria-label="桌面端操作">
      <button @click="act(Desktop.showMainWindow)"><i class="mdi mdi-view-dashboard-outline" aria-hidden="true"/>显示主窗口<i class="mdi mdi-chevron-right trailing" aria-hidden="true"/></button>
      <button :disabled="working" @click="act(Desktop.checkDesktopUpdate)"><i class="mdi mdi-update" aria-hidden="true"/>检查桌面端更新<i class="mdi mdi-chevron-right trailing" aria-hidden="true"/></button>
    </nav>
    <footer class="tray-footer">
      <span>{{ busy ? '正在处理…' : 'ani-rss-desktop' }}</span>
      <button class="quit-button" @click="act(Desktop.quitDesktop)"><i class="mdi mdi-power" aria-hidden="true"/>退出桌面端</button>
    </footer>
  </main>
</template>
