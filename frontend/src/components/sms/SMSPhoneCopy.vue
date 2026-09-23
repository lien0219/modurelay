<template>
  <div v-if="phone" class="inline-flex items-center gap-0.5">
    <CopyButton
      :text="fullNumber"
      :label="t('sms.user.copyWithCallingCode')"
      :success-message="t('sms.user.copyWithCallingCodeSuccess')"
    />
    <CopyButton
      v-if="canCopyLocal"
      :text="localNumber"
      :label="t('sms.user.copyWithoutCallingCode')"
      :success-message="t('sms.user.copyWithoutCallingCodeSuccess')"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import CopyButton from '@/components/common/CopyButton.vue'

const props = defineProps<{
  phone?: string
  callingCode?: string
}>()

const { t } = useI18n()

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
</script>
