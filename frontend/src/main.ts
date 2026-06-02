import { createApp } from 'vue'
import App from './App.vue'
import { initSettings } from './composables/useSettings'

initSettings().then(() => {
  createApp(App).mount('#app')
})
