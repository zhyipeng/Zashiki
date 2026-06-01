<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { NLayout, NLayoutSider, NLayoutContent, NMessageProvider, NSpin } from 'naive-ui'
import Sidebar from './components/Sidebar.vue'
import FileTable from './components/FileTable.vue'
import { FileService } from '../bindings/zashiki'

const currentPath = ref('')
const homeDir = ref('')
const loading = ref(true)
const error = ref('')

onMounted(async () => {
  try {
    homeDir.value = await FileService.GetHomeDir()
    currentPath.value = homeDir.value
  } catch (err) {
    console.error('GetHomeDir failed:', err)
    error.value = String(err)
    currentPath.value = '/'
  } finally {
    loading.value = false
  }
})

function onNavigate(path: string) {
  currentPath.value = path
}
</script>

<template>
  <NMessageProvider>
    <div v-if="loading" class="app-loading">
      <NSpin />
    </div>
    <NLayout v-else has-sider class="app-layout">
      <NLayoutSider width="260" bordered :native-scrollbar="false">
        <Sidebar
          :currentPath="currentPath"
          :homeDir="homeDir"
          @navigate="onNavigate"
        />
      </NLayoutSider>
      <NLayoutContent class="main-content">
        <FileTable
          :path="currentPath"
          @navigate="onNavigate"
        />
      </NLayoutContent>
    </NLayout>
  </NMessageProvider>
</template>

<style>
html, body, #app {
  margin: 0;
  padding: 0;
  width: 100vw;
  height: 100vh;
  overflow: hidden;
  user-select: none;
  -webkit-user-select: none;
}

.app-layout {
  width: 100vw;
  height: 100vh;
}

.app-loading {
  width: 100vw;
  height: 100vh;
  display: flex;
  justify-content: center;
  align-items: center;
}

.main-content {
  flex: 1;
  min-width: 0;
  overflow: hidden;
}
</style>
