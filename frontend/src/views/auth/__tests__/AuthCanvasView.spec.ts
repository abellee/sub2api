import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import AuthCanvasView from '@/views/auth/AuthCanvasView.vue'

enableAutoUnmount(afterEach)

vi.mock('@/mocks/adminAuth', () => ({ isAdminAuthMockEnabled: false }))

const { getPublicSettingsMock, pushMock } = vi.hoisted(() => ({
  getPublicSettingsMock: vi.fn(),
  pushMock: vi.fn()
}))

const cachedSettings = {
  site_name: 'TokenFree',
  site_subtitle: '无聊的人太多，暂停gmail、outlook邮箱注册',
  registration_enabled: true,
  backend_mode_enabled: false
}

const publicSettings = {
  site_name: 'TokenFree',
  registration_enabled: true,
  email_verify_enabled: true,
  turnstile_enabled: false,
  turnstile_site_key: '',
  tencent_captcha_enabled: false,
  tencent_captcha_app_id: '',
  aliyun_captcha_enabled: false,
  aliyun_captcha_scene_id: '',
  aliyun_captcha_prefix: '',
  linuxdo_oauth_enabled: false,
  dingtalk_oauth_enabled: false,
  wechat_oauth_enabled: false,
  backend_mode_enabled: false,
  oidc_oauth_enabled: false,
  oidc_oauth_provider_name: 'OIDC',
  github_oauth_enabled: false,
  google_oauth_enabled: false,
  password_reset_enabled: false,
  passkey_enabled: false,
  promo_code_enabled: false,
  invitation_code_enabled: false,
  affiliate_enabled: false,
  login_agreement_enabled: false,
  login_agreement_documents: []
}

vi.mock('vue-router', () => ({
  useRoute: () => currentRoute,
  useRouter: () => ({
    push: pushMock,
    currentRoute: { value: currentRoute }
  })
}))

vi.mock('vue-i18n', () => ({
  createI18n: () => ({
    global: {
      t: (key: string) => key
    }
  }),
  useI18n: () => ({
    t: (key: string) => key
  })
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => ({
    login: vi.fn(),
    loginWithPasskey: vi.fn(),
    login2FA: vi.fn(),
    register: vi.fn()
  }),
  useAppStore: () => ({
    siteName: 'TokenFree',
    siteLogo: '',
    publicSettingsLoaded: true,
    cachedPublicSettings: cachedSettings,
    fetchPublicSettings: vi.fn(),
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showWarning: vi.fn()
  })
}))

vi.mock('@/api/auth', () => ({
  buildOAuthLoginStartURL: vi.fn(),
  getPublicSettings: (...args: unknown[]) => getPublicSettingsMock(...args),
  isTotp2FARequired: vi.fn(() => false),
  isWeChatWebOAuthEnabled: vi.fn(() => false),
  startOAuthLogin: vi.fn(),
  validatePromoCode: vi.fn(),
  validateInvitationCode: vi.fn()
}))

const currentRoute = reactive({ path: '/login', query: {} })

function mountCanvas() {
  return mount(AuthCanvasView, {
    global: {
      stubs: {
        LoginCanvasPanel: true,
        RegisterCanvasPanel: true,
        RouterLink: { template: '<a><slot /></a>' },
        transition: false
      }
    }
  })
}

describe('AuthCanvasView', () => {
  beforeEach(() => {
    currentRoute.path = '/login'
    getPublicSettingsMock.mockReset()
    pushMock.mockReset()
    getPublicSettingsMock.mockResolvedValue(publicSettings)
  })

  it('renders login tab by default with brand text and announce pill', async () => {
    const wrapper = mountCanvas()
    await flushPromises()

    expect(wrapper.text()).toContain('auth.signInToSite')
    expect(wrapper.find('.ac-announce').text()).toContain(cachedSettings.site_subtitle)
    expect(wrapper.findAll('.ac-tabs button')).toHaveLength(2)
    expect(wrapper.find('.ac-tabs button.on').text()).toBe('auth.signIn')
    // 跑马灯：13 个品牌 × 两组
    expect(wrapper.findAll('.mq-item')).toHaveLength(26)
  })

  it('navigates to /register when the register tab is clicked', async () => {
    const wrapper = mountCanvas()
    await flushPromises()

    await wrapper.findAll('.ac-tabs button')[1].trigger('click')

    expect(pushMock).toHaveBeenCalledWith({ path: '/register', query: {} })
  })

  it('hides the announce pill when site_subtitle is empty', async () => {
    cachedSettings.site_subtitle = ''
    const wrapper = mountCanvas()
    await flushPromises()

    expect(wrapper.find('.ac-announce').exists()).toBe(false)
    cachedSettings.site_subtitle = '无聊的人太多，暂停gmail、outlook邮箱注册'
  })

  it('shows the register entry below the card only when registration is enabled', async () => {
    const wrapper = mountCanvas()
    await flushPromises()
    expect(wrapper.find('.ac-below').text()).toContain('auth.signUp')

    cachedSettings.registration_enabled = false
    const wrapper2 = mountCanvas()
    await flushPromises()
    expect(wrapper2.find('.ac-below').text()).not.toContain('auth.signUp')
    cachedSettings.registration_enabled = true
  })
})

describe('AuthCanvasView active form agreements', () => {
  function mountForms(realCaptcha = false) {
    return mount(AuthCanvasView, {
      attachTo: document.body,
      global: {
        stubs: {
          Icon: true,
          CaptchaChallenge: !realCaptcha,
          RouterLink: { template: '<a><slot /></a>' },
          transition: true
        }
      }
    })
  }

  function dialogs() {
    return document.body.querySelectorAll('[class~="z-[140]"]')
  }

  beforeEach(() => {
    currentRoute.path = '/login'
    localStorage.clear()
    sessionStorage.clear()
    getPublicSettingsMock.mockReset()
    getPublicSettingsMock.mockResolvedValue({
      ...publicSettings,
      login_agreement_enabled: true,
      login_agreement_mode: 'modal',
      login_agreement_revision: 'test-revision',
      login_agreement_documents: [{ id: 'terms', title: 'Terms' }]
    })
  })

  afterEach(() => {
    document.body.innerHTML = ''
    document.head.querySelectorAll('script[src*="turnstile"]').forEach(script => script.remove())
    delete window.turnstile
    delete window.onTurnstileLoad
    localStorage.clear()
  })

  it.each(['/login', '/register'])('shows only one modal on %s and closes it once', async path => {
    currentRoute.path = path
    mountForms()
    await flushPromises()

    expect(dialogs()).toHaveLength(1)
    const rejectButton = dialogs()[0].querySelector('button') as HTMLButtonElement
    rejectButton.click()
    await flushPromises()

    expect(dialogs()).toHaveLength(0)
  })

  it('keeps one modal when switching between login and register', async () => {
    mountForms()
    await flushPromises()
    expect(dialogs()).toHaveLength(1)

    currentRoute.path = '/register'
    await flushPromises()
    expect(dialogs()).toHaveLength(1)

    currentRoute.path = '/login'
    await flushPromises()
    expect(dialogs()).toHaveLength(1)
  })

  it('does not ask again after accepting the same agreement and switching forms', async () => {
    const wrapper = mountForms()
    await flushPromises()
    const buttons = dialogs()[0].querySelectorAll('button')
    buttons[buttons.length - 1].click()
    await flushPromises()
    expect(dialogs()).toHaveLength(0)

    currentRoute.path = '/register'
    await flushPromises()

    expect(dialogs()).toHaveLength(0)
    expect(wrapper.find('#email').attributes('disabled')).toBeUndefined()
  })

  it('lets the register agreement label check the visible checkbox', async () => {
    currentRoute.path = '/register'
    const settings = await getPublicSettingsMock()
    getPublicSettingsMock.mockResolvedValue({ ...settings, login_agreement_mode: 'checkbox' })
    const wrapper = mountForms()
    await flushPromises()

    expect(wrapper.findAll('#login-agreement-consent')).toHaveLength(1)
    await wrapper.find('label[for="login-agreement-consent"]').trigger('click')

    expect((wrapper.find('#login-agreement-consent').element as HTMLInputElement).checked).toBe(true)
    expect(wrapper.find('#email').attributes('disabled')).toBeUndefined()
  })

  it('initializes the visible login captcha when the script loads for the first time', async () => {
    getPublicSettingsMock.mockResolvedValue({
      ...publicSettings,
      turnstile_enabled: true,
      turnstile_site_key: 'test-site-key'
    })
    const wrapper = mountForms(true)
    await flushPromises()
    const rendered: HTMLElement[] = []
    window.turnstile = {
      render: vi.fn((container: HTMLElement, options: { callback: (token: string) => void }) => {
        rendered.push(container)
        options.callback('test-token')
        return String(rendered.length)
      }),
      remove: vi.fn(),
      reset: vi.fn()
    }
    window.onTurnstileLoad?.()
    await flushPromises()

    expect(rendered).toHaveLength(1)
    expect(wrapper.find('.ac-pane').element.contains(rendered[0])).toBe(true)
    expect(wrapper.find('button[type="submit"]').attributes('disabled')).toBeUndefined()
  })
})
