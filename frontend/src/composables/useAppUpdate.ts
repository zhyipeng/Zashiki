import { computed, ref, shallowRef } from 'vue'
import { Events } from '@wailsio/runtime'
import { UpdateService } from '../../bindings/zashiki/internal/appupdate'

const status = shallowRef({ currentVersion: '', enabled: false, reason: '', state: '', version: '', notes: '', size: 0 })
const visible = ref(false)
const busy = ref(false)
const error = ref('')
const feedback = ref('')
const percent = ref<number | null>(null)
let dismissedVersion = ''

async function refresh() {
  status.value = await UpdateService.GetStatus()
}

async function check(manual = false) {
  if (busy.value || !status.value.enabled) return
  if (status.value.state === 'ready') {
    if (manual) visible.value = true
    return
  }
  busy.value = true
  error.value = ''
  feedback.value = ''
  try {
    status.value = await UpdateService.Check()
    if (status.value.version && (manual || status.value.version !== dismissedVersion)) {
      visible.value = true
    } else if (manual) {
      feedback.value = '当前已是最新版本'
    }
  } catch (err) {
    if (manual) error.value = String(err)
  } finally {
    busy.value = false
  }
}

async function install() {
  if (busy.value) return
  busy.value = true
  error.value = ''
  percent.value = null
  try {
    await UpdateService.Download()
    await refresh()
    await UpdateService.Restart()
  } catch (err) {
    error.value = String(err)
    await refresh().catch(() => {})
  } finally {
    busy.value = false
  }
}

function dismiss() {
  if (busy.value) return
  dismissedVersion = status.value.version
  visible.value = false
}

// 由根组件挂载一次，设置弹窗复用状态，不重复注册定时器和事件。
function start() {
  let stopped = false
  const offProgress = Events.On('wails:updater:download-progress', event => {
    const progress = event.data as { written: number, total: number }
    percent.value = progress.total > 0 ? Math.min(100, Math.floor(progress.written / progress.total * 100)) : null
  })
  void refresh().then(() => { if (!stopped) void check() }).catch(() => {})
  const timer = window.setInterval(() => { void check() }, 6 * 60 * 60 * 1000)
  return () => {
    stopped = true
    window.clearInterval(timer)
    offProgress()
  }
}

export function useAppUpdate() {
  return {
    status, visible, busy, error, feedback, percent,
    ready: computed(() => status.value.state === 'ready'),
    check, install, dismiss, start,
  }
}
