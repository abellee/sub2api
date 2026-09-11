<template>
  <ChannelStatusV1View v-if="isV1" />
  <ChannelStatusV2StudioView v-else />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { isChannelMonitorV1Mode } from '@/utils/featureFlags'
import ChannelStatusV1View from './ChannelStatusV1View.vue'
import ChannelStatusV2StudioView from './ChannelStatusV2StudioView.vue'

const isV1 = computed(() => isChannelMonitorV1Mode())
</script>
