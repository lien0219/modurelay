<template>
  <div v-if="phone" class="inline-flex items-center gap-1">
    <button
      type="button"
      class="btn btn-secondary btn-sm whitespace-nowrap"
      :title="t('sms.user.copyWithCallingCode')"
      @click="copy(fullNumber, t('sms.user.copyWithCallingCodeSuccess'))"
    >
      {{ t('sms.user.copyWithCallingCodeShort') }}
    </button>
    <button
      type="button"
      class="btn btn-secondary btn-sm whitespace-nowrap"
      :disabled="!canCopyLocal"
      :title="canCopyLocal ? t('sms.user.copyWithoutCallingCode') : t('sms.user.copyWithoutCallingCodeUnavailable')"
      @click="copy(localNumber, t('sms.user.copyWithoutCallingCodeSuccess'))"
    >
      {{ t('sms.user.copyWithoutCallingCodeShort') }}
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'

const props = defineProps<{
  phone?: string
  callingCode?: string
}>()

const { t } = useI18n()
const appStore = useAppStore()

function digits(value?: string) {
  return String(value || '').replace(/\D/g, '')
}

const fullNumber = computed(() => {
  const raw = String(props.phone || '').trim()
  if (!raw) return ''
  const phoneDigits = digits(raw)
  if (!phoneDigits) return raw

  const codeDigits = digits(props.callingCode)
  if (raw.startsWith('+')) return `+${phoneDigits}`
  if (codeDigits && phoneDigits.startsWith(codeDigits)) return `+${phoneDigits}`
  if (codeDigits) return `+${codeDigits}${phoneDigits}`
  return raw
})

const localNumber = computed(() => {
  const phoneDigits = digits(fullNumber.value)
  const codeDigits = digits(props.callingCode)
  if (!phoneDigits || !codeDigits || !phoneDigits.startsWith(codeDigits)) return ''
  const local = phoneDigits.slice(codeDigits.length)
  return local || ''
})

const canCopyLocal = computed(() => Boolean(localNumber.value))

async function copy(value: string, successMessage: string) {
  if (!value) return
  try {
    await navigator.clipboard.writeText(value)
    appStore.showSuccess(successMessage)
  } catch {
    appStore.showError(t('sms.user.copyFailed'))
  }
}
</script>
