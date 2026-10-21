<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NButton, NInput, useMessage } from 'naive-ui'
import { LanShareService } from '../../../bindings/zashiki/internal/lanshare'

const props = defineProps<{ accessCode: string }>()
const emit = defineEmits<{ (event: 'saved'): void }>()
const message = useMessage()
const draft = ref(props.accessCode)
const saving = ref(false)
const valid = computed(() => draft.value === '' || /^[A-Za-z0-9_-]{1,32}$/.test(draft.value))
const changed = computed(() => draft.value !== props.accessCode)

watch(() => props.accessCode, value => { draft.value = value })

async function save() {
  if (saving.value || !valid.value || !changed.value) return
  saving.value = true
  try {
    await LanShareService.SetAccessCode(draft.value)
    emit('saved')
    message.success('访问码已保存，连接地址已更新')
  } catch (err) {
    message.error(`保存访问码失败：${err instanceof Error ? err.message : String(err)}`)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="access-code-settings">
    <label for="lan-access-code">自定义短访问码</label>
    <div class="access-code-row">
      <NInput
        id="lan-access-code"
        v-model:value="draft"
        size="small"
        placeholder="例如 abc；留空使用随机访问码"
        :status="valid ? undefined : 'error'"
        :disabled="saving"
        @keydown.enter.prevent="save"
      />
      <NButton size="small" :loading="saving" :disabled="!valid || !changed" @click="save">
        保存
      </NButton>
    </div>
    <div class="access-code-hint" :class="{ invalid: !valid }">
      {{ valid ? '支持 1–32 位英文字母、数字、_ 和 -，区分大小写。留空恢复随机访问码。保存后旧链接失效。' : '访问码只能包含 1–32 位英文字母、数字、_ 和 -。' }}
    </div>
  </div>
</template>

<style scoped>
.access-code-settings {
  margin-top: 16px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 12px;
}
.access-code-row {
  display: flex;
  gap: 8px;
}
.access-code-hint {
  color: var(--n-text-color-3, #888);
}
.access-code-hint.invalid {
  color: #d03050;
}
</style>
