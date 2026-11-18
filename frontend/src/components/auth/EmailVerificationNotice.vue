<template>
  <aside
    class="flex items-start gap-3 rounded-xl border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-700/60 dark:bg-amber-900/20 dark:text-amber-200"
  >
    <Icon name="mail" size="md" class="mt-0.5 shrink-0" aria-hidden="true" />
    <div class="min-w-0 break-words">
      <p class="font-semibold">{{ t('auth.emailVerificationSpamNotice') }}</p>
      <p v-if="supportContact" class="mt-1">
        {{ t('auth.emailVerificationSupport', { contact: supportContact }) }}
      </p>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{ contactInfo: string }>()
const { t } = useI18n()
const supportContact = computed(() => {
  const contact = props.contactInfo.trim()
  return /^\d+$/.test(contact) ? `QQ ${contact}` : contact
})
</script>
