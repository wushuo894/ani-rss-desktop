import {computed, inject, type InjectionKey, onMounted, onUnmounted, provide, ref} from 'vue'
import {Desktop, type Status} from './mygo'

const desktopStateKey: InjectionKey<ReturnType<typeof provideDesktopState>> = Symbol('desktopState')

// 在应用层维护进程状态及操作提示，让各个页面共用同一份桌面状态。
export function provideDesktopState() {
    const status = ref<Status>()
    const busy = ref(false)
    const notification = ref({open: false, message: '', color: 'error'})
    const running = computed(() => status.value?.state === 'running')
    const active = computed(() => ['starting', 'stopping', 'running'].includes(status.value?.state ?? ''))
    const working = computed(() => busy.value || Boolean(status.value?.operation))
    let polling = false
    let timer: ReturnType<typeof setInterval> | undefined

    // 显示后台操作结果或需要处理的错误。
    function notify(message: string, color = 'error') {
        notification.value = {open: true, message, color}
    }

    // 每次只读取一份进程状态，避免较慢的请求覆盖新状态。
    async function poll() {
        if (polling) return
        polling = true
        try {
            status.value = await Desktop.getStatus()
        } catch (error) {
            if (!status.value) notify(error instanceof Error ? error.message : String(error))
        } finally {
            polling = false
        }
    }

    // 串行处理页面提交，结束后刷新状态。
    async function act(task: () => Promise<unknown>, message = '') {
        if (busy.value) return
        busy.value = true
        try {
            await task()
            if (message) notify(message, 'success')
        } catch (error) {
            notify(error instanceof Error ? error.message : String(error))
        } finally {
            busy.value = false
            await poll()
        }
    }

    onMounted(() => {
        void poll()
        timer = setInterval(poll, 800)
    })
    onUnmounted(() => clearInterval(timer))

    const state = {status, busy, notification, running, active, working, notify, act}
    provide(desktopStateKey, state)
    return state
}

// 只由应用内的页面调用，读取桌面状态和操作方法。
export function useDesktopState() {
    const state = inject(desktopStateKey)
    if (!state) throw new Error('桌面状态尚未初始化')
    return state
}
