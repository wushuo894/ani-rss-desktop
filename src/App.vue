<script setup lang="ts">
import {computed, ref} from 'vue'
import {provideDesktopState} from './desktopState'
import JdkView from './views/JdkView.vue'
import LogsView from './views/LogsView.vue'
import OverviewView from './views/OverviewView.vue'
import SettingsView from './views/SettingsView.vue'
import brand from './brand.png'

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
  <v-app>
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
