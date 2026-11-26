<script setup lang="ts">
import {computed, onActivated, onDeactivated, ref, watch} from 'vue'
import {useDesktopState} from '../desktopState'
import {Desktop, type Overview, type UpdateInfo} from '../mygo'

const emit = defineEmits<{ showJdks: [] }>()
const {status, busy, running, active, working, notify, act} = useDesktopState()
const overview = ref<Overview>()
const overviewError = ref('')
const updateInfo = ref<UpdateInfo>()
const showUpdate = ref(false)
let timer: ReturnType<typeof setInterval> | undefined
let visible = false
let readingOverview = false

const taskLabels: Record<string, string> = {
  downloading: '下载中', forcedDL: '强制下载', stalledDL: '等待连接', queuedDL: '排队中',
  metaDL: '获取元数据', forcedMetaDL: '获取元数据', checkingDL: '校验中', allocating: '分配空间',
  pausedDL: '已暂停', stoppedDL: '已停止', error: '下载错误', missingFiles: '文件缺失',
}
const completedStates = new Set([
  'uploading', 'stalledUP', 'forcedUP', 'queuedUP', 'stoppedUP', 'pausedUP', 'checkingUP',
])
const tasks = computed(() => (overview.value?.downloads ?? []).filter(task => task.progress < 100 && !completedStates.has(task.state)))

// 读取后台指标，并丢弃实例在请求期间停止时返回的旧数据。
async function refreshOverview() {
  if (!running.value || !visible || readingOverview) return
  readingOverview = true
  const url = status.value?.url
  try {
    const result = await Desktop.getOverview()
    if (running.value && url === status.value?.url) {
      overview.value = result
      overviewError.value = ''
    }
  } catch (error) {
    if (running.value && url === status.value?.url) {
      overview.value = undefined
      overviewError.value = error instanceof Error ? error.message : String(error)
    }
  } finally {
    readingOverview = false
  }
}

// 获取最新版本，只有发行版可安装时才打开确认窗口。
function checkUpdate() {
  return act(async () => {
    updateInfo.value = await Desktop.checkUpdate()
    if (updateInfo.value.available) showUpdate.value = true
    else notify('已使用最新版本', 'success')
  })
}

// 启动前读取最新的 JDK 选用状态。
function startService() {
  return act(async () => {
    const [settings, jdks] = await Promise.all([Desktop.getSettings(), Desktop.getJDKs()])
    if (!jdks?.some(jdk => jdk.id === settings.jdkId && jdk.usable)) {
      emit('showJdks')
      notify('先下载或添加一个可用的 JDK')
      return
    }
    await Desktop.start()
  })
}

watch(running, (ready) => {
  if (ready) {
    void refreshOverview()
  } else {
    overview.value = undefined
    overviewError.value = ''
  }
})
onActivated(() => {
  visible = true
  void refreshOverview()
  timer = setInterval(refreshOverview, 7000)
})
onDeactivated(() => {
  visible = false
  clearInterval(timer)
  showUpdate.value = false
})
</script>

<template>
  <div class="page-content">
    <header class="page-header">
      <h1>概览</h1>
      <v-btn v-if="running" variant="outlined" color="primary" append-icon="mdi-open-in-new"
             @click="act(() => Desktop.openWeb('ui'))">打开 Web UI
      </v-btn>
    </header>
    <section class="panel service-panel">
      <div class="service-heading">
        <div class="service-icon">
          <v-icon icon="mdi-server-outline" size="24"/>
        </div>
        <div>
          <h2>ani-rss 服务</h2>
          <div class="service-meta">
            <span v-if="status?.version" class="mono">v{{ status.version }}</span>
            <span v-if="running" class="mono">{{ status?.url }}</span>
            <span v-else-if="!status?.version">尚未安装</span>
          </div>
        </div>
      </div>
      <v-alert v-if="status?.message" color="error" variant="tonal" density="compact" class="mt-4">{{
          status.message
        }}
      </v-alert>
      <div class="controls">
        <v-btn color="primary" prepend-icon="mdi-play" :disabled="active || working" @click="startService">启动</v-btn>
        <v-btn variant="tonal" prepend-icon="mdi-restart" :disabled="!running || working" @click="act(Desktop.restart)">
          重启
        </v-btn>
        <v-btn variant="tonal" prepend-icon="mdi-stop" :disabled="!active || working" @click="act(Desktop.stop)">停止
        </v-btn>
        <v-spacer/>
        <v-btn variant="text" prepend-icon="mdi-update" :disabled="working" @click="checkUpdate">检查更新</v-btn>
      </div>
    </section>
    <div class="metrics">
      <div class="panel metric"><span><v-icon icon="mdi-rss" size="19"/> 订阅</span><strong>{{
          running && overview && !overview.subscriptionError ? overview.subscriptions : '—'
        }}</strong></div>
      <div class="panel metric"><span><v-icon icon="mdi-download-outline" size="19"/> 下载任务</span><strong>{{
          running && overview && !overview.downloadError ? tasks.length : '—'
        }}</strong></div>
    </div>
    <section class="panel">
      <div class="panel-heading">
        <h2>下载概览</h2>
        <v-btn v-if="running" icon="mdi-refresh" variant="text" size="small" aria-label="刷新下载状态"
               @click="refreshOverview"/>
      </div>
      <p v-if="!running" class="empty">启动服务后查看下载任务。</p>
      <p v-else-if="overviewError" class="empty">{{ overviewError }}</p>
      <p v-else-if="overview?.downloadError" class="empty">{{ overview.downloadError }}</p>
      <p v-else-if="!overview" class="empty">正在读取下载任务…</p>
      <p v-else-if="!tasks.length" class="empty">暂无进行中的下载任务。</p>
      <v-table v-else density="comfortable">
        <thead>
        <tr>
          <th>名称</th>
          <th>状态</th>
          <th class="download-progress">进度</th>
          <th>大小</th>
        </tr>
        </thead>
        <tbody>
        <tr v-for="task in tasks" :key="task.id">
          <td>{{ task.name }}</td>
          <td>{{ taskLabels[task.state] ?? task.state }}</td>
          <td>
            <v-progress-linear :model-value="task.progress" color="primary" rounded/>
            <span class="mono">{{ task.progress }}%</span></td>
          <td>{{ task.formatSize }}</td>
        </tr>
        </tbody>
      </v-table>
      <p v-if="overview?.subscriptionError" class="form-note">{{ overview.subscriptionError }}</p>
    </section>
  </div>

  <v-dialog v-model="showUpdate" :persistent="busy" max-width="520">
    <v-card v-if="updateInfo" class="dialog-card">
      <v-card-title>{{ updateInfo.current ? '更新 ani-rss' : '安装 ani-rss' }}</v-card-title>
      <v-card-text>
        {{ updateInfo.current ? '当前版本 v' + updateInfo.current + '，' : '' }}可安装版本 v{{ updateInfo.latest }}。
        {{ running ? '下载并校验后将重启服务。' : '' }}
      </v-card-text>
      <v-card-actions>
        <v-spacer/>
        <v-btn @click="showUpdate = false">取消</v-btn>
        <v-btn color="primary" variant="flat" @click="showUpdate = false; act(Desktop.update, 'ani-rss 已更新')">下载安装
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
