<script setup lang="ts">
import {computed, onActivated, onDeactivated, ref} from 'vue'
import {useDesktopState} from '../desktopState'
import {Desktop, type JDK, type Settings} from '../mygo'

const emit = defineEmits<{ showJdks: [] }>()
const {status, busy, running, active, working, notify, act} = useDesktopState()
const settings = ref<Settings>()
const selectedJava = ref<JDK>()
const extraArgs = ref('')
const autoStart = ref(false)
const dialog = ref<'password' | 'command' | null>(null)
const newPassword = ref('')
const username = ref('')
const command = ref('')
const commandError = ref('')
const currentCommand = ref(false)
const savedSettings = ref('')
const canSave = computed(() => changed.value && !invalidSettings.value && !working.value)

// 保存与脏状态比较共用同一份参数，忽略输入结尾的空行和空格。
const draft = computed(() => {
  const current = settings.value
  if (!current) return undefined
  return {
    ...current,
    dataDir: current.dataDir.trim(),
    address: current.address.trim(),
    port: Number(current.port),
    minHeap: Number(current.minHeap),
    maxHeap: Number(current.maxHeap),
    jvmArgs: extraArgs.value.split('\n').map(line => line.trim()).filter(Boolean),
  }
})
const changed = computed(() => draft.value !== undefined && JSON.stringify(draft.value) !== savedSettings.value)

// 数字输入允许临时清空，只有完整且有效的参数才能保存。
const invalidSettings = computed(() => {
  const current = draft.value
  if (!current) return ''
  if (!current.dataDir || !current.address) return '请填写配置目录和绑定地址。'
  if (!Number.isInteger(current.port) || current.port < 1 || current.port > 65535) return '服务端口须在 1 到 65535 之间。'
  if (!Number.isInteger(current.minHeap) || !Number.isInteger(current.maxHeap) || current.minHeap < 1 || current.maxHeap < current.minHeap) {
    return '最大堆内存须大于或等于初始值，初始值至少为 1 MiB。'
  }
  return ''
})
const saveHint = computed(() => {
  if (invalidSettings.value) return invalidSettings.value
  if (!changed.value) return '所有更改已保存'
  return running.value ? '有未保存的更改，重启服务后生效。' : '有未保存的更改'
})

// 启动设置只在初始化或保存后读取，避免覆盖用户正在填写的表单。
async function loadSettings() {
  settings.value = await Desktop.getSettings()
  extraArgs.value = settings.value.jvmArgs?.join('\n') ?? ''
  savedSettings.value = JSON.stringify(draft.value)
}

// 展示当前或下次启动的命令。命令始终使用后端的实际启动参数。
async function showLaunchCommand(current = false) {
  currentCommand.value = current
  command.value = ''
  commandError.value = ''
  dialog.value = 'command'
  try {
    const result = await Desktop.getLaunchCommand(current)
    if (currentCommand.value === current) command.value = result
  } catch (error) {
    if (currentCommand.value === current) commandError.value = error instanceof Error ? error.message : String(error)
  }
}

// 将表单提交给 Go，可让运行中的服务立即应用新设置。
function saveSettings(restart: boolean) {
  const current = draft.value
  if (!current || !canSave.value) return
  return act(async () => {
    await Desktop.saveSettings(current, restart)
    await loadSettings()
  }, '启动设置已保存')
}

// 选择数据目录后保留在表单，统一通过保存操作应用。
function pickDataDirectory() {
  return act(async () => {
    const path = await Desktop.pickPath('data')
    if (path && settings.value) settings.value.dataDir = path
  })
}

// 系统登录项独立即时生效，并重新读取操作系统返回的状态。
function setAutoStart(enabled: boolean | null) {
  return act(async () => {
    await Desktop.setAutoStart(Boolean(enabled))
    autoStart.value = await Desktop.getAutoStart()
  })
}

// 打开弹窗前读取当前用户名，并清空密码输入。
function openPassword() {
  return act(async () => {
    username.value = await Desktop.getLoginUsername()
    newPassword.value = ''
    dialog.value = 'password'
  })
}

// 通过 Go 将用户名和新密码写入服务配置文件。
function submitPassword() {
  if (!username.value.trim() || !newPassword.value.trim()) {
    notify('请输入用户名和新密码')
    return
  }
  return act(async () => {
    await Desktop.changePassword(username.value, newPassword.value)
    dialog.value = null
    newPassword.value = ''
  }, '登录信息已更新')
}

// 每次进入页面刷新外部状态，同时保留尚未保存的启动参数。
async function refreshSettingsPage() {
  try {
    if (!settings.value) await loadSettings()
    const [saved, jdks, openAtLogin] = await Promise.all([
      Desktop.getSettings(), Desktop.getJDKs(), Desktop.getAutoStart(),
    ])
    selectedJava.value = jdks?.find(jdk => jdk.id === saved.jdkId)
    autoStart.value = openAtLogin
  } catch (error) {
    notify(error instanceof Error ? error.message : String(error))
  }
}

onActivated(refreshSettingsPage)
onDeactivated(() => {
  dialog.value = null
  // 离开设置页时清空尚未提交的密码。
  newPassword.value = ''
})
</script>

<template>
  <div class="settings-page">
    <header class="page-header settings-header">
      <h1>启动设置</h1>
      <v-btn variant="outlined" color="primary" prepend-icon="mdi-console-line" @click="showLaunchCommand()">
        查看启动命令
      </v-btn>
    </header>

    <template v-if="settings">
      <div class="settings-grid">
        <section class="panel settings-card settings-wide">
          <h2>
            <v-icon icon="mdi-server-network-outline" size="20"/>
            服务配置
          </h2>
          <div class="settings-fields">
            <div class="field-with-button settings-wide">
              <v-text-field v-model="settings.dataDir" label="配置与数据目录" variant="outlined" density="comfortable"
                            hide-details :disabled="working"/>
              <v-btn variant="tonal" prepend-icon="mdi-folder-open-outline" :disabled="working"
                     @click="pickDataDirectory">浏览
              </v-btn>
            </div>
            <v-text-field v-model="settings.address" label="绑定 IP 地址" variant="outlined" density="comfortable"
                          hint="127.0.0.1 仅限本机访问，0.0.0.0 监听所有 IPv4 网卡。" persistent-hint
                          :disabled="working"/>
            <v-number-input v-model="settings.port" label="服务端口" :min="1" :max="65535" :precision="0"
                            control-variant="stacked" variant="outlined" density="comfortable" hide-details
                            :disabled="working"/>
          </div>
        </section>

        <section class="panel settings-card settings-wide">
          <h2>
            <v-icon icon="mdi-language-java" size="20"/>
            Java 虚拟机
          </h2>
          <div class="runtime-row">
            <div>
              <div class="setting-title">下次启动的 JDK</div>
              <p class="setting-detail">
                {{ selectedJava ? 'Java ' + selectedJava.version + ' · ' + selectedJava.arch : '尚未选用 JDK' }}</p>
            </div>
            <v-btn variant="text" color="primary" append-icon="mdi-chevron-right" @click="emit('showJdks')">JDK 管理
            </v-btn>
          </div>
          <div class="settings-fields">
            <v-number-input v-model="settings.minHeap" label="初始堆内存" suffix="MiB" :min="1" :precision="0"
                            control-variant="stacked" variant="outlined" density="comfortable" hide-details
                            :disabled="working"/>
            <v-number-input v-model="settings.maxHeap" label="最大堆内存" suffix="MiB" :min="1" :precision="0"
                            control-variant="stacked" variant="outlined" density="comfortable" hide-details
                            :disabled="working"/>
          </div>
          <details class="extra-arguments">
            <summary>
              <span>额外 JVM 参数</span>
              <span v-if="draft?.jvmArgs.length" class="setting-detail">{{ draft.jvmArgs.length }} 项</span>
              <v-icon icon="mdi-chevron-down" size="19"/>
            </summary>
            <v-textarea v-model="extraArgs" aria-label="额外 JVM 参数" placeholder="每行填写一个 JVM 参数" rows="3"
                        variant="outlined" density="comfortable" hide-details :disabled="working"/>
          </details>
        </section>

        <section class="panel settings-card">
          <h2>
            <v-icon icon="mdi-connection" size="20"/>
            服务接口
          </h2>
          <div class="preference-row">
            <div>
              <div class="setting-title">Swagger</div>
              <p class="setting-detail">接口文档</p>
            </div>
            <v-btn v-if="running && status?.swagger" size="small" icon="mdi-open-in-new" variant="text"
                   aria-label="打开 Swagger 接口文档" @click="act(() => Desktop.openWeb('swagger'))"/>
            <v-switch v-model="settings.swagger" aria-label="启用 Swagger" color="primary" density="compact"
                      hide-details :disabled="working"/>
          </div>
          <div class="preference-row">
            <div>
              <div class="setting-title">MCP</div>
              <p class="setting-detail">工具接入</p>
            </div>
            <v-btn v-if="running && status?.mcp" size="small" icon="mdi-content-copy" variant="text"
                   aria-label="复制 MCP 接入地址"
                   @click="act(() => Desktop.copyText(status?.url + '/api/mcp'), 'MCP 接入地址已复制')"/>
            <v-switch v-model="settings.mcp" aria-label="启用 MCP" color="primary" density="compact" hide-details
                      :disabled="working"/>
          </div>
        </section>

        <section class="panel settings-card">
          <h2>
            <v-icon icon="mdi-account-cog-outline" size="20"/>
            桌面与账户
          </h2>
          <div class="preference-row">
            <div>
              <div class="setting-title">登录时启动</div>
              <p class="setting-detail">登录系统时启动桌面端和 ani-rss</p>
            </div>
            <v-switch :model-value="autoStart" aria-label="登录系统时启动 ani-rss" color="primary" density="compact"
                      hide-details :disabled="working" @update:model-value="setAutoStart"/>
          </div>
          <div class="preference-row">
            <span class="setting-title">Web UI 密码</span>
            <v-btn variant="tonal" size="small" :disabled="working" @click="openPassword">重设密码</v-btn>
          </div>
        </section>
      </div>

      <footer class="settings-footer">
        <span class="save-hint" :class="{ 'invalid-hint': invalidSettings }" role="status">{{ saveHint }}</span>
        <div class="heading-actions">
          <v-btn :color="running ? undefined : 'primary'" :variant="running ? 'tonal' : 'flat'" :disabled="!canSave"
                 @click="saveSettings(false)">保存设置
          </v-btn>
          <v-btn v-if="running" color="primary" :disabled="!canSave" @click="saveSettings(true)">保存并重启</v-btn>
        </div>
      </footer>
    </template>
  </div>

  <v-dialog :model-value="dialog !== null" :persistent="busy" :max-width="dialog === 'command' ? 720 : 520"
            @update:model-value="value => { if (!value) dialog = null }">
    <v-card v-if="dialog === 'password'" class="dialog-card">
      <v-card-title>修改 Web UI 登录信息</v-card-title>
      <v-form @submit.prevent="submitPassword">
        <v-card-text>
          <v-text-field v-model="username" label="用户名" autocomplete="username"
                        variant="outlined" density="compact"/>
          <v-text-field v-model="newPassword" label="新密码" autocomplete="new-password"
                        type="text" variant="outlined" density="compact" hide-details/>
          <p class="form-note">写入服务数据目录中的 config.v2.json。{{ active ? '保存时将重启当前服务。' : '' }}</p>
        </v-card-text>
        <v-card-actions>
          <v-spacer/>
          <v-btn :disabled="busy" @click="dialog = null">取消</v-btn>
          <v-btn type="submit" color="primary" variant="flat" :loading="busy"
                 :disabled="!username.trim() || !newPassword.trim()">确定
          </v-btn>
        </v-card-actions>
      </v-form>
    </v-card>
    <v-card v-else-if="dialog === 'command'" class="dialog-card">
      <v-card-title>启动命令</v-card-title>
      <v-card-text>
        <div class="heading-actions mb-3">
          <v-btn :variant="currentCommand ? 'text' : 'tonal'" color="primary" @click="showLaunchCommand(false)">
            下次启动
          </v-btn>
          <v-btn v-if="active" :variant="currentCommand ? 'tonal' : 'text'" color="primary"
                 @click="showLaunchCommand(true)">当前进程
          </v-btn>
        </div>
        <pre v-if="command" class="command-output mono">{{ command }}</pre>
        <p v-else>{{ commandError || '正在读取启动命令…' }}</p>
        <p v-if="!currentCommand" class="form-note">使用已保存的启动设置。</p>
      </v-card-text>
      <v-card-actions>
        <v-spacer/>
        <v-btn @click="dialog = null">关闭</v-btn>
        <v-btn color="primary" variant="flat" prepend-icon="mdi-content-copy" :disabled="!command"
               @click="act(() => Desktop.copyText(command), '启动命令已复制')">复制命令
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.settings-page {
  container-type: inline-size;
}

.settings-header {
  margin-bottom: 20px;
}

.settings-grid, .settings-fields {
  display: grid;
  gap: 16px;
}

.settings-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.settings-wide {
  grid-column: 1 / -1;
}

.settings-fields {
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  align-items: start;
}

.settings-card h2 {
  display: flex;
  align-items: center;
  gap: 9px;
  margin-bottom: 17px;
}

.settings-card h2 .v-icon {
  color: var(--accent);
}

.settings-card .field-with-button .v-input {
  min-width: 0;
}

.runtime-row, .preference-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.runtime-row {
  justify-content: space-between;
  margin-bottom: 16px;
  padding: 9px 10px 9px 13px;
  border-radius: 9px;
  background: #f8efe6;
}

.preference-row {
  min-height: 52px;
  padding-block: 6px;
}

.preference-row + .preference-row {
  border-top: 1px solid var(--line);
  margin-top: 4px;
}

.preference-row > :first-child {
  flex: 1 1 auto;
  min-width: 0;
}

.preference-row .v-switch {
  flex: 0 0 auto;
}

.setting-title {
  font-size: 14px;
  font-weight: 630;
}

.setting-detail {
  color: var(--muted);
  font-size: 12px;
  margin-top: 2px;
}

.extra-arguments {
  margin-top: 16px;
  border-top: 1px solid var(--line);
}

.extra-arguments summary {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 14px 0 1px;
  cursor: pointer;
  font-size: 14px;
  font-weight: 630;
}

.extra-arguments summary::-webkit-details-marker {
  display: none;
}

.extra-arguments summary .v-icon {
  margin-left: auto;
  color: var(--muted);
}

.extra-arguments[open] summary {
  margin-bottom: 12px;
}

.extra-arguments[open] summary .v-icon {
  transform: rotate(180deg);
}

/* 表单滚动时保留保存入口，键盘焦点仍按文档顺序移动。 */
.settings-footer {
  position: sticky;
  bottom: 0;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 16px;
  padding: 12px 14px;
  border: 1px solid var(--line);
  border-radius: 12px;
  background: var(--paper);
  box-shadow: 0 -3px 12px #4a2e1d0d;
}

.save-hint {
  color: var(--muted);
  font-size: 12px;
}

.invalid-hint {
  color: rgb(var(--v-theme-error));
}

@container (max-width: 620px) {
  .settings-grid {
    grid-template-columns: 1fr;
  }
}

@container (max-width: 460px) {
  .settings-fields {
    grid-template-columns: 1fr;
  }

  .settings-footer {
    flex-wrap: wrap;
  }
}
</style>
