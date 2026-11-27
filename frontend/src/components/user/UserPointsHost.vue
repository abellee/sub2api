<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useAuthStore } from '@/stores'
import { OPEN_USER_RESOURCE_EVENT } from '@/composables/userResourceEvents'
import { updateUserPoints } from '@/composables/useUserPoints'
import PointsModal from './PointsModal.vue'

const authStore = useAuthStore()
const user = computed(() => authStore.user)
const pointsOpen = ref(false)
function openResource(event: Event) {
  if ((event as CustomEvent).detail === 'points' && user.value) pointsOpen.value = true
}
watch(() => user.value?.id, () => { pointsOpen.value = false })
onMounted(() => window.addEventListener(OPEN_USER_RESOURCE_EVENT, openResource))
onBeforeUnmount(() => window.removeEventListener(OPEN_USER_RESOURCE_EVENT, openResource))
</script>

<template>
  <PointsModal v-if="user" :show="pointsOpen" :user-id="user.id" @close="pointsOpen = false" @loaded="updateUserPoints($event, user.id)" />
</template>
