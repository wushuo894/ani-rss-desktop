<script setup lang="ts">
import {onActivated, onDeactivated, ref} from 'vue'
import {useDesktopState} from '../desktopState'
import {Desktop, type JDK, type JDKRelease} from '../mygo'

const {status, busy, working, notify, act} = useDesktopState()
const jdks = ref<JDK[]>([])
const configuredJDK = ref('')
const dialog = ref<'addJDK' | 'removeJDK' | null>(null)
const jdkPath = ref('')
const jdkRelease = ref<string | null>(null)
const availableJDKs = ref<JDKRelease[]>([])
const loadingJDKs = ref(false)
const jdkSourceError = ref('')
const selectedForRemoval = ref<JDK>()

// 刷新本机 JDK 列表和下次启动选中的条目。
async function refreshJDKs() {
  const [items, current] = await Promise.all([Desktop.getJDKs(), Desktop.getSettings()])
  jdks.value = items ?? []
  configuredJDK.value = current.jdkId
}

// 从 Adoptium 加载可下载版本，优先选中列表内的 Java 25。
async function loadJDKVersions() {
  if (loadingJDKs.value) return
  loadingJDKs.value = true
  jdkSourceError.value = ''
  try {
    availableJDKs.value = (await Desktop.getJDKVersions()) ?? []
    if (!availableJDKs.value.some(release => release.id === jdkRelease.value)) {
      jdkRelease.value = (availableJDKs.value.find(release => release.major === 25) ?? availableJDKs.value[0])?.id ?? null
    }
    if (!availableJDKs.value.length) jdkSourceError.value = '此系统暂无可下载的版本。'
  } catch (error) {
    jdkSourceError.value = error instanceof Error ? error.message : String(error)
  } finally {
    loadingJDKs.value = false
  }
}

// 下载选中的 API 发行版，并刷新本机安装列表。
function installJDK() {
  const release = jdkRelease.value
  if (!release) return
  return act(async () => {
    await Desktop.downloadJDK(release)
    await refreshJDKs()
  }, 'JDK 已安装')
}

// 根据 JDK 来源显示相应的删除确认文案。
function confirmRemove(jdk: JDK) {
  selectedForRemoval.value = jdk
  dialog.value = 'removeJDK'
}

onActivated(() => {
  void refreshJDKs().catch(error => notify(error instanceof Error ? error.message : String(error)))
  if (!availableJDKs.value.length) void loadJDKVersions()
})
onDeactivated(() => {
  dialog.value = null
})
</script>

<template>
  <div class="page-content">
    <header class="page-header"><h1>JDK 管理</h1></header>
    <section class="panel">
      <div class="panel-heading">
        <h2>已添加的 JDK</h2>
        <div class="heading-actions">
          <v-btn variant="text" prepend-icon="mdi-magnify" :disabled="working"
                 @click="act(async () => { await Desktop.discoverJDKs(); await refreshJDKs() }, '本机扫描完成')">扫描本机
          </v-btn>
          <v-btn variant="tonal" color="primary" prepend-icon="mdi-plus" :disabled="working"
                 @click="jdkPath = ''; dialog = 'addJDK'">手动添加
          </v-btn>
        </div>
      </div>
      <v-table v-if="jdks.length" density="comfortable" class="jdk-table">
        <thead>
        <tr>
          <th>版本与架构</th>
          <th>来源与路径</th>
          <th>状态</th>
          <th>操作</th>
        </tr>
        </thead>
        <tbody>
        <tr v-for="jdk in jdks" :key="jdk.id">
          <td><strong>Java {{ jdk.version }}</strong>
            <div class="muted mono">{{ jdk.arch }}</div>
          </td>
          <td><span>{{ jdk.managed ? 'Temurin 下载' : jdk.discovered ? '系统发现' : '手动添加' }}</span>
            <div class="jdk-path mono muted" :title="jdk.java">{{ jdk.java }}</div>
          </td>
          <td>
            <v-chip v-if="status?.runningJdk === jdk.id" size="small" color="success">使用中</v-chip>
            <v-chip v-if="configuredJDK === jdk.id" size="small" color="primary">下次启动</v-chip>
            <v-chip v-if="!jdk.usable" size="small" color="error">{{
                jdk.major < 25 ? '版本不兼容' : '路径不可用'
              }}
            </v-chip>
          </td>
          <td class="jdk-actions">
            <v-btn size="small" variant="text" color="primary"
                   :disabled="working || !jdk.usable || configuredJDK === jdk.id"
                   @click="act(async () => { await Desktop.selectJDK(jdk.id); await refreshJDKs() }, '已用于下次启动')">
              选用
            </v-btn>
            <v-btn size="small" variant="text" color="error" :disabled="working || status?.runningJdk === jdk.id"
                   @click="confirmRemove(jdk)">{{ jdk.managed ? '删除' : '移除' }}
            </v-btn>
          </td>
        </tr>
        </tbody>
      </v-table>
      <p v-else class="empty">添加现有 JDK，或下载 Temurin JDK 25 来启动 ani-rss。</p>
    </section>
    <section class="panel">
      <h2>下载 Eclipse Temurin</h2>
      <div class="download-controls">
        <v-select v-model="jdkRelease" :items="availableJDKs" item-title="version" item-value="id" label="JDK 发行版本"
                  :loading="loadingJDKs" :disabled="loadingJDKs || !availableJDKs.length || working"
                  density="comfortable" variant="outlined" hide-details/>
        <v-btn icon="mdi-refresh" variant="text" aria-label="刷新可下载的 JDK" :disabled="loadingJDKs || working"
               @click="loadJDKVersions"/>
        <v-btn color="primary" prepend-icon="mdi-download" :disabled="working || !jdkRelease" @click="installJDK">下载
          JDK
        </v-btn>
      </div>
      <p v-if="jdkSourceError" role="alert" class="form-note">{{ jdkSourceError }}</p>
    </section>
  </div>

  <v-dialog :model-value="dialog !== null" :persistent="busy" max-width="520"
            @update:model-value="value => { if (!value) dialog = null }">
    <v-card v-if="dialog === 'addJDK'" class="dialog-card">
      <v-card-title>手动添加 JDK</v-card-title>
      <v-form
          @submit.prevent="act(async () => { await Desktop.addJDK(jdkPath); await refreshJDKs(); dialog = null }, 'JDK 已添加')">
        <v-card-text>
          <v-text-field v-model="jdkPath" label="JDK 目录或 java 可执行文件" variant="outlined" density="compact"/>
          <div class="controls mt-0">
            <v-btn variant="tonal"
                   @click="act(async () => { const path = await Desktop.pickPath('jdk'); if (path) jdkPath = path })">
              选择目录
            </v-btn>
            <v-btn variant="tonal"
                   @click="act(async () => { const path = await Desktop.pickPath('java'); if (path) jdkPath = path })">
              选择可执行文件
            </v-btn>
          </div>
        </v-card-text>
        <v-card-actions>
          <v-spacer/>
          <v-btn :disabled="busy" @click="dialog = null">取消</v-btn>
          <v-btn type="submit" color="primary" variant="flat" :disabled="!jdkPath" :loading="busy">添加</v-btn>
        </v-card-actions>
      </v-form>
    </v-card>
    <v-card v-else-if="dialog === 'removeJDK' && selectedForRemoval" class="dialog-card">
      <v-card-title>{{ selectedForRemoval.managed ? '删除 JDK' : '移除 JDK' }}</v-card-title>
      <v-card-text>Java {{ selectedForRemoval.version }}
        将从列表移除。{{ selectedForRemoval.managed ? '对应的安装文件也将删除。' : '' }}
      </v-card-text>
      <v-card-actions>
        <v-spacer/>
        <v-btn :disabled="busy" @click="dialog = null">取消</v-btn>
        <v-btn color="error" variant="flat" :loading="busy"
               @click="act(async () => { if (selectedForRemoval) await Desktop.deleteJDK(selectedForRemoval.id); await refreshJDKs(); dialog = null })">
          确定
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
