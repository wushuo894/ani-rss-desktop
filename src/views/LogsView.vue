<script setup lang="ts">
import {computed, nextTick, onActivated, onDeactivated, ref} from 'vue'
import {useDesktopState} from '../desktopState'
import {Desktop} from '../mygo'

const {act, notify} = useDesktopState()
const logQuery = ref<string | null>('')
const logs = ref('')
const logPanel = ref<HTMLElement>()
let timer: ReturnType<typeof setInterval> | undefined
let loading = false

const filteredLogs = computed(() => {
  const query = (logQuery.value ?? '').trim().toLowerCase()
  return query ? logs.value.split('\n').filter(line => line.toLowerCase().includes(query)).join('\n') : logs.value
})

// 日志页打开时读取最近输出，避免重复请求。
async function refreshLogs() {
  if (loading) return
  loading = true
  try {
    logs.value = await Desktop.getLogs()
    // 等待日志渲染完成，再滚动到最后一行。
    await nextTick()
    const panel = logPanel.value
    if (timer !== undefined && panel) {
      panel.scrollTop = panel.scrollHeight
    }
  } catch (error) {
    if (timer !== undefined) {
      clearInterval(timer)
      timer = undefined
      notify(error instanceof Error ? error.message : String(error))
    }
  } finally {
    loading = false
  }
}

// 打开日志页时立即刷新，离开时停止轮询。
onActivated(() => {
  timer = setInterval(refreshLogs, 800)
  void refreshLogs()
})
onDeactivated(() => {
  clearInterval(timer)
  timer = undefined
})
</script>

<template>
  <div class="page-content">
    <header class="page-header"><h1>日志</h1></header>
    <section class="panel">
      <div class="log-controls">
        <v-text-field v-model="logQuery" label="搜索日志" prepend-inner-icon="mdi-magnify" clearable hide-details
                      density="compact" variant="outlined"/>
        <v-btn variant="tonal" prepend-icon="mdi-content-copy" :disabled="!filteredLogs"
               @click="act(() => Desktop.copyText(filteredLogs), '日志已复制')">复制
        </v-btn>
        <v-btn variant="text" prepend-icon="mdi-folder-open-outline" @click="act(Desktop.openLogDirectory)">
          打开日志目录
        </v-btn>
      </div>
      <pre ref="logPanel" class="log-panel">{{
          filteredLogs || (logQuery ? '未找到匹配的日志。' : '暂无进程输出。')
        }}</pre>
    </section>
  </div>
</template>
