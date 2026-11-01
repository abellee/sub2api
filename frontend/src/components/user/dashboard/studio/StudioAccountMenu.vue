<template>
  <div ref="accountRoot" class="studio-account" :class="{ 'is-expanded': expanded }" :style="{ '--studio-tool-count': toolCount }" @mouseenter="cancelHoverClose" @mouseleave="scheduleHoverClose" @keydown.esc.stop="dismissPanels">
    <div class="studio-utilities" aria-label="常用工具">
      <StudioLocaleMenu :expanded="expanded" @opened="dismissPanels" />
      <AnnouncementBell v-if="user" class="studio-announcement" @click="closeDropdown(); communityOpen = null" />
      <button type="button" class="utility-button" :title="isDark ? t('nav.lightMode') : t('nav.darkMode')" :aria-label="isDark ? t('nav.lightMode') : t('nav.darkMode')" @click="toggleTheme"><Icon :name="isDark ? 'sun' : 'moon'" size="md" /></button>
      <a v-if="headerWidgetConfig" href="https://qm.qq.com/q/iX3TeHnckw" target="_blank" rel="noopener noreferrer" class="utility-button" title="加入 QQ 群" aria-label="加入 QQ 群" ref="qqTrigger" data-tour="join-qq-group" :aria-expanded="communityOpen === 'qq'" @mouseenter="openOnHover('qq', $event)"><QqBrandIcon /></a>
      <a v-if="headerWidgetConfig?.telegram" :href="headerWidgetConfig.telegram.link" target="_blank" rel="noopener noreferrer" class="utility-button" title="加入 Telegram" aria-label="加入 Telegram" :aria-expanded="communityOpen === 'telegram'" @mouseenter="openOnHover('telegram', $event)"><svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M9.78 18.65l.28-4.23 7.68-6.92c.34-.3-.07-.45-.52-.18L7.74 13.3l-4.1-1.28c-.89-.28-.9-.89.19-1.32l16-6.17c.74-.27 1.39.18 1.15 1.32l-2.72 12.81c-.2.91-.74 1.13-1.5.7l-4.14-3.06-2 1.93c-.22.22-.41.41-.84.41z" /></svg></a>
    </div>
    <section v-if="user" class="studio-user-card" :aria-label="t('common.userMenu')">
      <div class="studio-wallet" :inert="expanded ? undefined : true" :aria-hidden="!expanded" :title="`${t('common.availableBalance')}: ${formatMoney(availableBalance)} · ${t('common.frozenBalance')}: ${formatMoney(frozenBalance)}`">
        <div class="wallet-available"><span class="wallet-label">{{ t('common.availableBalance') }}</span><strong>{{ formatMoney(availableBalance) }}</strong></div>
        <div class="wallet-frozen"><span class="wallet-label">{{ t('common.frozenBalance') }}</span><strong>{{ formatMoney(frozenBalance) }}</strong></div>
      </div>
      <div class="studio-points" :inert="expanded ? undefined : true" :aria-hidden="!expanded"><span><img :src="pointsIcon" alt="" />{{ t('common.points') }}</span><strong>—</strong></div>
      <div class="studio-identity">
        <button ref="avatarButton" type="button" class="studio-avatar" :aria-label="expanded ? t('nav.profile') : t('common.userMenu')" :aria-expanded="expanded ? undefined : dropdownOpen" @mouseenter="!expanded && openOnHover('settings', $event)" @click="expanded ? router.push('/profile') : toggleDropdown($event)"><img v-if="avatarUrl" :src="avatarUrl" alt="" /><span v-else>{{ userInitials }}</span><i v-if="lotterySocketStatus !== 'idle'" class="studio-socket" :class="socketDotClass" role="status" :title="socketDotTitle" :aria-label="socketDotTitle"></i></button>
        <div class="studio-account-label" :inert="expanded ? undefined : true" :aria-hidden="!expanded"><strong>{{ displayName }}</strong><small>{{ t('admin.users.roles.' + (user?.role || 'user')) }}</small></div>
        <button ref="settingsButton" type="button" class="studio-settings utility-button" :inert="expanded ? undefined : true" :aria-hidden="!expanded" :aria-label="t('nav.settings')" :title="t('nav.settings')" :aria-expanded="dropdownOpen" aria-controls="studio-account-menu" @mouseenter="openOnHover('settings', $event)" @click="toggleDropdown"><Icon name="cog" size="md" /></button>
      </div>
    </section>
    <StudioPopover v-show="dropdownOpen" :anchor="panelAnchor" side="right" id="studio-account-menu" class="studio-flyout studio-settings-menu" :aria-label="t('common.userMenu')" @mouseenter="cancelHoverClose">
      <div class="studio-menu-section">
        <router-link to="/profile" class="studio-menu-item" @click="closeDropdown"><Icon name="user" size="sm" />{{ t('nav.profile') }}</router-link>
        <router-link to="/keys" class="studio-menu-item" @click="closeDropdown"><Icon name="key" size="sm" />{{ t('nav.apiKeys') }}</router-link>
        <router-link v-if="modelPlazaEnabled" :to="{ path: '/model-plaza', query: { embedded: '1' } }" class="studio-menu-item" @click="closeDropdown"><Icon name="grid" size="sm" />{{ t('nav.modelPlaza') }}</router-link>
        <div v-if="subscriptionFeatureEnabled" class="studio-subscription"><SubscriptionProgressMini /></div>
      </div>
      <div class="studio-menu-section"><PushNotificationToggle /></div>
      <div v-if="docUrl || authStore.isAdmin || showOnboardingButton || contactInfo" class="studio-menu-section">
        <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="studio-menu-item" @click="closeDropdown"><Icon name="book" size="sm" />{{ t('nav.docs') }}</a>
        <a v-if="authStore.isAdmin" href="https://github.com/Wei-Shaw/sub2api" target="_blank" rel="noopener noreferrer" class="studio-menu-item" @click="closeDropdown"><Icon name="externalLink" size="sm" />{{ t('nav.github') }}</a>
        <button v-if="showOnboardingButton" type="button" class="studio-menu-item" @click="handleReplayGuide"><Icon name="questionCircle" size="sm" />{{ t('onboarding.restartTour') }}</button>
        <div v-if="contactInfo" class="studio-contact"><span>{{ t('common.contactSupport') }}</span><span>{{ contactInfo }}</span></div>
      </div>
      <div class="studio-menu-section"><button type="button" class="studio-menu-item studio-logout" @click="handleLogout"><Icon name="arrowRight" size="sm" />{{ t('nav.logout') }}</button></div>
    </StudioPopover>
    <StudioPopover :anchor="panelAnchor" side="right" v-if="communityOpen && headerWidgetConfig" class="studio-flyout studio-community" role="dialog" :aria-label="communityOpen === 'qq' ? '加入 QQ 群' : '加入 Telegram'" @mouseenter="cancelHoverClose">
      <header><strong>{{ communityOpen === 'qq' ? headerWidgetConfig.qq.groupName : headerWidgetConfig.telegram.groupName }}</strong><button type="button" class="utility-button" :aria-label="t('common.close')" @click="communityOpen = null"><Icon name="x" size="sm" /></button></header>
      <img :src="communityOpen === 'qq' ? headerWidgetConfig.qq.qrUrl : headerWidgetConfig.telegram.qrUrl" :alt="communityOpen === 'qq' ? 'QQ群二维码' : 'Telegram 群组二维码'" />
      <div class="community-detail"><span>{{ communityOpen === 'qq' ? headerWidgetConfig.qq.groupNumber : headerWidgetConfig.telegram.link }}</span><button type="button" class="utility-button" :aria-label="communityOpen === 'qq' ? '复制QQ群号' : '复制Telegram群组链接'" @click="copyCommunity"><Icon name="copy" size="sm" /></button></div>
      <a :href="communityOpen === 'qq' ? 'https://qm.qq.com/q/iX3TeHnckw' : headerWidgetConfig.telegram.link" target="_blank" rel="noopener noreferrer" class="studio-community-link">{{ communityOpen === 'qq' ? '加入 QQ 群' : '加入 Telegram' }}<Icon name="externalLink" size="sm" /></a>
    </StudioPopover>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { CLOSE_QQ_GROUP_EVENT, OPEN_QQ_GROUP_EVENT, OPEN_USER_MENU_EVENT, PUSH_TOUR_UNLOCK_EVENT } from '@/composables/usePushNotificationTour'
import { useLotterySocketStatus } from '@/composables/useLotterySocketStatus'
import { useAppStore, useAuthStore, useOnboardingStore, useRemoteWidgetsStore } from '@/stores'
import { useTheme } from '@/composables/useTheme'
import { useClipboard } from '@/composables/useClipboard'
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import SubscriptionProgressMini from '@/components/common/SubscriptionProgressMini.vue'
import AnnouncementBell from '@/components/common/AnnouncementBell.vue'
import PushNotificationToggle from '@/components/common/PushNotificationToggle.vue'
import Icon from '@/components/icons/Icon.vue'
import QqBrandIcon from '@/components/icons/QqBrandIcon.vue'
import pointsIcon from '@/assets/icons/points-image-color.svg'
import StudioLocaleMenu from './StudioLocaleMenu.vue'
import StudioPopover from './StudioPopover.vue'

const props = defineProps<{ expanded: boolean }>()
const emit = defineEmits<{ holdOpen: [open: boolean] }>()
const router = useRouter()
const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const onboardingStore = useOnboardingStore()
const { headerConfig: headerWidgetConfig } = storeToRefs(useRemoteWidgetsStore())
const { copyToClipboard } = useClipboard()
const { isDark, setTheme } = useTheme()
const { status: lotterySocketStatus } = useLotterySocketStatus()
const socketDotTitle = computed(() => ({ open: '实时连接已建立', connecting: '正在连接', closed: '实时连接已断开', idle: '' })[lotterySocketStatus.value])
const socketDotClass = computed(() => lotterySocketStatus.value === 'open' ? 'socket-open' : lotterySocketStatus.value === 'connecting' ? 'socket-connecting' : 'socket-closed')
const accountRoot = ref<HTMLElement | null>(null)
const settingsButton = ref<HTMLButtonElement | null>(null)
const avatarButton = ref<HTMLButtonElement | null>(null)
const qqTrigger = ref<HTMLAnchorElement | null>(null)
const panelAnchor = ref<HTMLElement | null>(null)
const dropdownOpen = ref(false)
const dropdownLocked = ref(false)
const communityOpen = ref<'qq' | 'telegram' | null>(null)
let hoverCloseTimer: ReturnType<typeof setTimeout> | undefined
const user = computed(() => authStore.user)
const toolCount = computed(() => 2 + Number(!!headerWidgetConfig.value) + Number(!!headerWidgetConfig.value?.telegram) + Number(!!user.value))
const avatarUrl = computed(() => user.value?.avatar_url?.trim() || '')
const displayName = computed(() => user.value?.username || user.value?.email?.split('@')[0] || '')
const userInitials = computed(() => displayName.value.substring(0, 2).toUpperCase())
const availableBalance = computed(() => Number(user.value?.balance || 0))
const frozenBalance = computed(() => Number(user.value?.frozen_balance || 0))
const docUrl = computed(() => sanitizeUrl(appStore.docUrl))
const contactInfo = computed(() => appStore.contactInfo)
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))
const subscriptionFeatureEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.subscription))
const showOnboardingButton = computed(() => !authStore.isSimpleMode && authStore.isAdmin)
function formatMoney(value: number) { return `$${(Number.isFinite(value) ? value : 0).toFixed(2)}` }
function toggleDropdown(event?: Event) {
  cancelHoverClose()
  if (dropdownLocked.value) return
  panelAnchor.value = event?.currentTarget as HTMLElement || settingsButton.value || avatarButton.value
  dropdownOpen.value = !dropdownOpen.value
  communityOpen.value = null
}
function toggleTheme() {
  cancelHoverClose()
  closeDropdown()
  communityOpen.value = null
  setTheme(isDark.value ? 'light' : 'dark')
}
function closeDropdown() { if (!dropdownLocked.value) dropdownOpen.value = false }
function dismissPanels() {
  if (dropdownLocked.value) return
  const wasOpen = dropdownOpen.value
  dropdownOpen.value = false
  communityOpen.value = null
  if (wasOpen) (props.expanded ? settingsButton.value : avatarButton.value)?.focus()
}
function cancelHoverClose() { clearTimeout(hoverCloseTimer) }
watch(() => props.expanded, (expanded) => {
  if (expanded) return
  cancelHoverClose()
  dropdownOpen.value = false
  communityOpen.value = null
}, { flush: 'sync' })
function scheduleHoverClose() {
  cancelHoverClose()
  hoverCloseTimer = setTimeout(() => {
    if (dropdownLocked.value) return
    dropdownOpen.value = false
    communityOpen.value = null
  }, 180)
}
function openOnHover(panel: 'settings' | 'qq' | 'telegram', event: Event) {
  if (!window.matchMedia('(hover: hover) and (pointer: fine)').matches || dropdownLocked.value) return
  cancelHoverClose()
  panelAnchor.value = event.currentTarget as HTMLElement
  dropdownOpen.value = panel === 'settings'
  communityOpen.value = panel === 'qq' || panel === 'telegram' ? panel : null
}
async function copyCommunity() {
  if (!headerWidgetConfig.value) return
  if (communityOpen.value === 'qq') await copyToClipboard(headerWidgetConfig.value.qq.groupNumber, '群号已复制')
  else await copyToClipboard(headerWidgetConfig.value.telegram.link, 'Telegram 群组链接已复制')
}
function openUserMenuForTour() {
  emit('holdOpen', true)
  if (window.matchMedia('(max-width: 1023px)').matches) appStore.setMobileOpen(true)
  communityOpen.value = null
  dropdownOpen.value = true
  panelAnchor.value = settingsButton.value || avatarButton.value
  dropdownLocked.value = true
}
function unlockUserMenu() { dropdownLocked.value = false; emit('holdOpen', false) }
function openQqGroupForTour() {
  emit('holdOpen', true)
  if (window.matchMedia('(max-width: 1023px)').matches) appStore.setMobileOpen(true)
  dropdownLocked.value = false
  dropdownOpen.value = false
  communityOpen.value = 'qq'
  panelAnchor.value = qqTrigger.value
}
function closeQqGroupForTour() { communityOpen.value = null; emit('holdOpen', false) }
async function handleLogout() {
  closeDropdown()
  try { await authStore.logout() } catch (error) { console.error('Logout error:', error) }
  await router.push('/login')
}
function handleReplayGuide() { closeDropdown(); onboardingStore.replay() }
function handleClickOutside(event: MouseEvent) {
  if (!accountRoot.value?.contains(event.target as Node) && !dropdownLocked.value) {
    dropdownOpen.value = false
    communityOpen.value = null
  }
}
onMounted(() => {
  document.addEventListener('click', handleClickOutside)
  window.addEventListener(OPEN_USER_MENU_EVENT, openUserMenuForTour)
  window.addEventListener(PUSH_TOUR_UNLOCK_EVENT, unlockUserMenu)
  window.addEventListener(OPEN_QQ_GROUP_EVENT, openQqGroupForTour)
  window.addEventListener(CLOSE_QQ_GROUP_EVENT, closeQqGroupForTour)
})
onBeforeUnmount(() => {
  cancelHoverClose()
  document.removeEventListener('click', handleClickOutside)
  window.removeEventListener(OPEN_USER_MENU_EVENT, openUserMenuForTour)
  window.removeEventListener(PUSH_TOUR_UNLOCK_EVENT, unlockUserMenu)
  window.removeEventListener(OPEN_QQ_GROUP_EVENT, openQqGroupForTour)
  window.removeEventListener(CLOSE_QQ_GROUP_EVENT, closeQqGroupForTour)
})
</script>

<style scoped>
.studio-account{--studio-account-width:calc(var(--studio-sidebar-width,248px) - 36px);position:relative;color:var(--studio-ink);font-size:13px}
.studio-utilities{position:relative;height:calc(var(--studio-tool-count)*36px - 2px);margin-bottom:12px;transition:height var(--studio-motion,240ms) var(--studio-ease,ease),margin-bottom var(--studio-motion,240ms) var(--studio-ease,ease)}
.studio-account .studio-utilities> *{--tool-index:0;position:absolute;top:0;left:0;width:36px;height:34px;transform:translate(8px,calc(var(--tool-index)*36px));transition:transform var(--studio-motion,240ms) var(--studio-ease,ease),background-color .15s,color .15s}
.studio-utilities> :nth-child(2){--tool-index:1}.studio-utilities> :nth-child(3){--tool-index:2}.studio-utilities> :nth-child(4){--tool-index:3}.studio-utilities> :nth-child(5){--tool-index:4}
.utility-button{display:flex;align-items:center;justify-content:center;width:36px;height:34px;flex-shrink:0;border:0;border-radius:10px;background:transparent;color:var(--studio-muted);cursor:pointer;transition:background .15s,color .15s}
.utility-button:hover,.utility-button[aria-expanded=true]{background:var(--tf-console-hover);color:var(--studio-ink)}
.utility-button:focus-visible,.studio-avatar:focus-visible,.studio-menu-item:focus-visible{outline:2px solid #8796db;outline-offset:2px}
.utility-button svg{width:19px;height:19px}
.studio-announcement :deep(button){width:36px;height:34px;color:var(--studio-muted);border-radius:10px;transition:background-color .15s,color .15s}
.studio-announcement :deep(button:hover),.studio-announcement :deep(button:focus-visible){background:var(--tf-console-hover);color:var(--studio-ink);transform:none}
.studio-announcement :deep(button:active){background:color-mix(in srgb,var(--studio-muted) 22%,transparent);transform:none}
.studio-announcement :deep(button:focus-visible){outline:2px solid #8796db;outline-offset:2px}
.studio-user-card{position:relative;height:40px;overflow:hidden;background:transparent;border-radius:14px;transition:height var(--studio-motion,240ms) var(--studio-ease,ease),background-color var(--studio-motion,240ms)}
.studio-wallet{position:absolute;top:14px;left:12px;width:calc(var(--studio-account-width) - 24px);display:flex;justify-content:space-between;gap:8px;font-variant-numeric:tabular-nums}
.wallet-available{min-width:0;text-align:left}.wallet-frozen{text-align:right}.wallet-label{display:block;margin-bottom:4px;font-size:12px;color:var(--studio-muted);white-space:nowrap}
.studio-wallet strong{display:block;font-size:16px;font-weight:500;line-height:24px;letter-spacing:normal;white-space:nowrap}
.studio-points{position:absolute;top:66px;left:12px;width:calc(var(--studio-account-width) - 24px);display:flex;align-items:center;justify-content:space-between;font-size:12px;color:var(--studio-muted)}
.studio-points>span{display:flex;align-items:center;gap:6px}.studio-points img{width:15px;height:15px}
.studio-identity{position:absolute;bottom:2px;left:8px;width:calc(var(--studio-account-width) - 24px);display:flex;align-items:center;gap:10px;height:36px;transition:left var(--studio-motion,240ms) var(--studio-ease,ease),bottom var(--studio-motion,240ms) var(--studio-ease,ease)}
.studio-identity::before{content:'';position:absolute;top:-11px;left:0;right:0;height:1px;background:color-mix(in srgb,var(--studio-muted) 12%,transparent);opacity:0;transition:opacity .15s}
.studio-wallet,.studio-points,.studio-account-label,.studio-settings{visibility:hidden;opacity:0;pointer-events:none;transition:opacity .12s,visibility .12s}
.is-expanded .studio-wallet,.is-expanded .studio-points,.is-expanded .studio-account-label,.is-expanded .studio-settings{visibility:visible;opacity:1;pointer-events:auto;transition-delay:60ms}
.studio-avatar{position:relative;display:grid;place-items:center;width:36px;height:36px;flex-shrink:0;border:0;border-radius:12px;background:#6577bd;color:#fff;cursor:pointer;font-size:12px;font-weight:600}
.studio-avatar>img{width:100%;height:100%;object-fit:cover;border-radius:inherit}
.studio-account-label{min-width:0;flex:1}.studio-account-label strong{display:block;font-size:13px;font-weight:600;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.studio-account-label small{display:block;font-size:12px;color:var(--studio-muted);margin-top:2px}
.studio-settings{width:30px;height:32px}.studio-socket{position:absolute;right:-1px;bottom:0;width:7px;height:7px;border:2px solid var(--tf-console-surface);border-radius:50%;box-sizing:content-box}.socket-open{background:#75b4a0}.socket-connecting{background:#c6a46b}.socket-closed{background:#c77989}
.is-expanded .studio-utilities{height:34px;margin-bottom:10px}
.studio-account.is-expanded .studio-utilities> *{transform:translate(calc(var(--tool-index)*(var(--studio-account-width) - 36px)/(var(--studio-tool-count) - 1)),0)}
.studio-utilities> :has(.studio-popover){z-index:3}
.is-expanded .studio-user-card{height:154px;background:var(--studio-menu-surface,var(--tf-console-hover))}
.wallet-frozen strong{color:var(--studio-muted)}
.is-expanded .studio-identity{left:12px;bottom:10px}.is-expanded .studio-identity::before{opacity:1}
.studio-flyout{width:280px;max-height:calc(100dvh - 32px);overflow-y:auto;padding:8px;border-radius:16px;background:var(--tf-console-surface);color:var(--studio-ink);scrollbar-width:thin}
.studio-menu-section+.studio-menu-section{border-top:1px solid color-mix(in srgb,var(--studio-muted) 12%,transparent);margin-top:5px;padding-top:5px}
.studio-menu-item,.studio-settings-menu :deep(.dropdown-item){display:flex;align-items:center;gap:10px;width:100%;padding:10px;border:0;border-radius:9px;background:transparent;text-align:left;color:var(--studio-ink);font-size:13px;cursor:pointer}
.studio-menu-item:hover,.studio-settings-menu :deep(.dropdown-item:hover){background:var(--tf-console-hover)}
.studio-menu-item svg{flex-shrink:0}.studio-logout{color:#bc657b}
.studio-contact{display:flex;flex-direction:column;gap:5px;padding:10px;color:var(--studio-muted);font-size:12px;overflow-wrap:anywhere}
.studio-subscription{padding:4px 10px}.studio-subscription :deep(.absolute){position:static;width:100%;margin-top:8px;box-shadow:none}.studio-subscription :deep([class*="text-[10px]"]){font-size:12px}
.studio-community{padding:16px}.studio-community header{display:flex;align-items:center;justify-content:space-between;gap:8px;margin-bottom:12px;font-size:14px}.studio-community>img{width:100%;max-height:250px;object-fit:contain;border-radius:10px}.community-detail{display:flex;align-items:center;gap:6px;margin-top:8px;font-size:12px;color:var(--studio-muted)}.community-detail>span{flex:1;min-width:0;overflow-wrap:anywhere}.studio-community-link{display:flex;align-items:center;justify-content:center;gap:8px;margin-top:10px;padding:10px;border-radius:10px;background:var(--tf-console-hover);font-size:13px}

</style>
