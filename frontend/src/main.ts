import { createApp } from 'vue'
import App from './App.vue'
import { initSettings } from './composables/useSettings'
import { Service as MediaStreamService } from '../bindings/zashiki/internal/mediastream'
import { setMediaStreamBase } from './components/assetUrl'

initSettings()
  .then(() => MediaStreamService.GetBaseURL())
  .then((base) => setMediaStreamBase(base || ''))
  .catch((err) => console.error('media stream init failed:', err))
  .finally(() => {
    createApp(App).mount('#app')
  })
