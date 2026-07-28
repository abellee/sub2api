<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { setLocale } from '@/i18n'
import ModelPlazaContent from '@/components/model-plaza/ModelPlazaContent.vue'

type ThemeChoice = 'system' | 'light' | 'dark'

const props = withDefaults(defineProps<{ page?: 'home' | 'models' }>(), { page: 'home' })

const THEME_KEY = 'llmfree-home-theme'
const BUSINESS_QQ = '751077517'
const authStore = useAuthStore()
const { locale } = useI18n()
const pageRef = ref<HTMLElement | null>(null)
const copyToastVisible = ref(false)
const systemDark = ref(window.matchMedia('(prefers-color-scheme: dark)').matches)
const savedTheme = localStorage.getItem(THEME_KEY)
const themeChoice = ref<ThemeChoice>(savedTheme === 'light' || savedTheme === 'dark' ? savedTheme : 'system')
let copyToastTimer: number | undefined
const systemThemeQuery = window.matchMedia('(prefers-color-scheme: dark)')

const language = ref<'zh' | 'en'>(locale.value.startsWith('zh') ? 'zh' : 'en')
const resolvedTheme = computed<'light' | 'dark'>(() => themeChoice.value === 'system' ? (systemDark.value ? 'dark' : 'light') : themeChoice.value)
const isAuthenticated = computed(() => authStore.isAuthenticated)
const dashboardPath = computed(() => authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
const primaryActionPath = computed(() => isAuthenticated.value ? dashboardPath.value : '/register')

function closeDropdown(event: Event) {
  ;(event.currentTarget as HTMLElement | null)?.closest('details')?.removeAttribute('open')
}

async function selectLanguage(code: 'zh' | 'en', event: Event) {
  const dropdown = (event.currentTarget as HTMLElement | null)?.closest('details')
  language.value = code
  dropdown?.removeAttribute('open')
  await setLocale(code)
}

function selectTheme(theme: ThemeChoice, event: Event) {
  themeChoice.value = theme
  localStorage.setItem(THEME_KEY, theme)
  closeDropdown(event)
}

function handleDropdownToggle(event: Event) {
  const current = event.currentTarget as HTMLDetailsElement
  if (!current.open || !pageRef.value) return
  pageRef.value.querySelectorAll<HTMLDetailsElement>('[data-dropdown][open]').forEach((dropdown) => {
    if (dropdown !== current) dropdown.removeAttribute('open')
  })
}

function handleDocumentClick(event: MouseEvent) {
  if (!pageRef.value) return
  pageRef.value.querySelectorAll<HTMLDetailsElement>('[data-dropdown][open]').forEach((dropdown) => {
    if (!dropdown.contains(event.target as Node)) dropdown.removeAttribute('open')
  })
}

function handleSystemThemeChange(event: MediaQueryListEvent) {
  systemDark.value = event.matches
}

function fallbackCopy(value: string) {
  const textarea = document.createElement('textarea')
  textarea.value = value
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.select()
  document.execCommand('copy')
  textarea.remove()
}

async function copyBusinessQq() {
  try {
    if (navigator.clipboard?.writeText) await navigator.clipboard.writeText(BUSINESS_QQ)
    else fallbackCopy(BUSINESS_QQ)
  } catch {
    fallbackCopy(BUSINESS_QQ)
  }

  copyToastVisible.value = true
  window.clearTimeout(copyToastTimer)
  copyToastTimer = window.setTimeout(() => {
    copyToastVisible.value = false
  }, 1800)
}

const structuredData = {
  "@context": "https://schema.org",
  "@graph": [
    {
      "@type": "Organization",
      "@id": "https://llmfree.work/#organization",
      "name": "LLM Free",
      "url": "https://llmfree.work/",
      "logo": "https://llmfree.work/llmfree/llmfree-logo-transparent.png",
      "sameAs": ["https://t.me/+nqFUXD_y66liYTll"]
    },
    {
      "@type": "WebSite",
      "@id": "https://llmfree.work/#website",
      "url": "https://llmfree.work/",
      "name": "LLM Free AI API Gateway",
      "publisher": { "@id": "https://llmfree.work/#organization" },
      "inLanguage": ["zh-CN", "en"]
    },
    {
      "@type": "Service",
      "name": "LLM Free AI API Gateway",
      "serviceType": "AI API Gateway",
      "provider": { "@id": "https://llmfree.work/#organization" },
      "url": "https://llmfree.work/",
      "description": "OpenAI-compatible AI API gateway providing unified access to Claude, GPT, Gemini and other leading models."
    },
    {
      "@type": "FAQPage",
      "mainEntity": [
        {
          "@type": "Question",
          "name": "What is LLM Free?",
          "acceptedAnswer": { "@type": "Answer", "text": "LLM Free is an OpenAI-compatible AI API gateway with unified model access, API key management, usage statistics, and cost visibility." }
        },
        {
          "@type": "Question",
          "name": "Which AI models are supported?",
          "acceptedAnswer": { "@type": "Answer", "text": "LLM Free supports Claude, GPT, Gemini, and other leading AI models. Current availability is listed in the model plaza." }
        },
        {
          "@type": "Question",
          "name": "How can users get trial credit?",
          "acceptedAnswer": { "@type": "Answer", "text": "Register and join the LLM Free QQ or Telegram group to request trial credit." }
        }
      ]
    }
  ]
}

onMounted(() => {
  document.addEventListener('click', handleDocumentClick)
  systemThemeQuery.addEventListener('change', handleSystemThemeChange)

  const schema = document.createElement('script')
  schema.id = 'llmfree-home-structured-data'
  schema.type = 'application/ld+json'
  schema.textContent = JSON.stringify(structuredData)
  document.head.appendChild(schema)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleDocumentClick)
  systemThemeQuery.removeEventListener('change', handleSystemThemeChange)
  window.clearTimeout(copyToastTimer)
  document.getElementById('llmfree-home-structured-data')?.remove()
})
</script>

<template>
<div ref="pageRef" class="llmf-page" :data-theme="resolvedTheme" :data-theme-choice="themeChoice" :data-auth="isAuthenticated ? 'authenticated' : 'guest'" :data-language="language">
  <header class="llmf-header">
    <div class="llmf-wrap llmf-nav">
      <RouterLink class="llmf-brand" to="/home" aria-label="LLM Free">
        <span class="llmf-mark"><img src="/llmfree/llmfree-logo-transparent.png" alt="LLM Free Logo"></span>
        <span class="llmf-brand-copy">
          <span class="llmf-brand-name">LLM Free</span>
          <span class="llmf-brand-note"><span data-lang="zh">AI API 中转站</span><span data-lang="en">AI API Gateway</span></span>
        </span>
      </RouterLink>

      <div class="llmf-nav-right">
        <nav class="llmf-nav-links" aria-label="Primary">
          <RouterLink v-if="props.page === 'models'" to="/home" title="首页 / Home">
            <span class="llmf-nav-icon" aria-hidden="true">
              <svg viewBox="0 0 24 24"><path d="m3 10 9-7 9 7"></path><path d="M5 9.5V21h14V9.5"></path><path d="M9 21v-7h6v7"></path></svg>
            </span>
            <span class="llmf-nav-link-label"><span data-lang="zh">首页</span><span data-lang="en">Home</span></span>
          </RouterLink>

          <RouterLink to="/models" title="模型广场 / Model Plaza">
            <span class="llmf-nav-icon" aria-hidden="true">
              <svg viewBox="0 0 24 24"><rect x="3" y="3" width="7" height="7" rx="1"></rect><rect x="14" y="3" width="7" height="7" rx="1"></rect><rect x="3" y="14" width="7" height="7" rx="1"></rect><rect x="14" y="14" width="7" height="7" rx="1"></rect></svg>
            </span>
            <span class="llmf-nav-link-label"><span data-lang="zh">模型广场</span><span data-lang="en">Models</span></span>
          </RouterLink>

          <span class="llmf-nav-community llmf-nav-tooltip" data-tooltip-zh="加入 LLM Free QQ群" data-tooltip-en="Join the LLM Free QQ group">
            <span class="llmf-nav-icon" aria-hidden="true">
              <svg viewBox="0 0 24 24"><path d="M8.4 10.1c.2-3.1 1.5-5.2 3.6-5.2s3.4 2.1 3.6 5.2c1.4 1.2 2.2 3 2.2 5 0 1-.2 1.9-.6 2.7-.5-.6-1.1-1-1.8-1.3-.8 1.1-2 1.8-3.4 1.8s-2.6-.7-3.4-1.8c-.7.3-1.3.7-1.8 1.3-.4-.8-.6-1.7-.6-2.7 0-2 .8-3.8 2.2-5Z"></path><path d="M9.2 20.2c.8.4 1.8.7 2.8.7s2-.3 2.8-.7"></path><path d="M9.8 10.6h.1M14.1 10.6h.1"></path></svg>
            </span>
            <span class="llmf-nav-link-label"><span data-lang="zh">QQ群</span><span data-lang="en">QQ Group</span></span>
            <a class="llmf-official-overlay" target="_blank" href="https://qm.qq.com/cgi-bin/qm/qr?k=4m4LsIHtnUmdqd-G3vWiPNLVq9g_fx-a&amp;jump_from=webapi&amp;authKey=PAe3IwrmVErvv6/3dyjOj7t2gHCXz2GfmlG9nS2PuUWcaAJ3iQ8lqit6nM3q9EgR">
              <img border="0" src="//pub.idqqimg.com/wpa/images/group.png" alt="LLM-Free售后" title="LLM-Free售后">
            </a>
          </span>

          <a class="llmf-nav-tooltip" href="https://t.me/+nqFUXD_y66liYTll" target="_blank" rel="noopener noreferrer" data-tooltip-zh="加入 LLM Free Telegram群" data-tooltip-en="Join the LLM Free Telegram group">
            <span class="llmf-nav-icon llmf-nav-icon-telegram" aria-hidden="true">
              <svg viewBox="0 0 24 24"><path d="M21.6 3.1 18.4 20c-.2 1.2-.9 1.5-1.9.9l-4.9-3.6-2.4 2.3c-.3.3-.5.5-1 .5l.4-5 9.1-8.2c.4-.4-.1-.6-.6-.2L5.8 13.8 1 12.3c-1-.3-1.1-1 .2-1.5L20 3.6c.9-.3 1.7.2 1.6-.5Z"></path></svg>
            </span>
            <span class="llmf-nav-link-label">Telegram</span>
          </a>
        </nav>

        <div class="llmf-switches">
          <details class="llmf-dropdown" data-dropdown @toggle="handleDropdownToggle">
            <summary aria-label="语言 / Language"><span class="llmf-language-current"><span data-lang="zh">中</span><span data-lang="en">EN</span></span></summary>
            <div class="llmf-dropdown-menu">
              <button class="llmf-dropdown-option" :class="{ 'is-active': language === 'zh' }" type="button" @click="selectLanguage('zh', $event)">中文</button>
              <button class="llmf-dropdown-option" :class="{ 'is-active': language === 'en' }" type="button" @click="selectLanguage('en', $event)">English</button>
            </div>
          </details>

          <details class="llmf-dropdown" data-dropdown @toggle="handleDropdownToggle">
            <summary aria-label="主题 / Theme">
              <span class="llmf-theme-current">
                <span class="llmf-theme-current-system"><svg viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="13" rx="2"></rect><path d="M8 21h8M12 17v4"></path></svg></span>
                <span class="llmf-theme-current-light"><svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="4"></circle><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"></path></svg></span>
                <span class="llmf-theme-current-dark"><svg viewBox="0 0 24 24"><path d="M20.5 14.5A8.4 8.4 0 0 1 9.5 3.5 8.5 8.5 0 1 0 20.5 14.5Z"></path></svg></span>
              </span>
            </summary>
            <div class="llmf-dropdown-menu">
              <button class="llmf-dropdown-option" :class="{ 'is-active': themeChoice === 'system' }" type="button" @click="selectTheme('system', $event)"><svg viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="13" rx="2"></rect><path d="M8 21h8M12 17v4"></path></svg><span data-lang="zh">跟随系统</span><span data-lang="en">System</span></button>
              <button class="llmf-dropdown-option" :class="{ 'is-active': themeChoice === 'light' }" type="button" @click="selectTheme('light', $event)"><svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="4"></circle><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"></path></svg><span data-lang="zh">浅色</span><span data-lang="en">Light</span></button>
              <button class="llmf-dropdown-option" :class="{ 'is-active': themeChoice === 'dark' }" type="button" @click="selectTheme('dark', $event)"><svg viewBox="0 0 24 24"><path d="M20.5 14.5A8.4 8.4 0 0 1 9.5 3.5 8.5 8.5 0 1 0 20.5 14.5Z"></path></svg><span data-lang="zh">深色</span><span data-lang="en">Dark</span></button>
            </div>
          </details>
        </div>

        <div class="llmf-auth-actions">
          <RouterLink v-if="!isAuthenticated" class="llmf-header-action llmf-header-action-primary" to="/register"><span data-lang="zh">注册</span><span data-lang="en">SIGN UP</span></RouterLink>
          <RouterLink v-if="!isAuthenticated" class="llmf-header-action" to="/login"><span data-lang="zh">登录</span><span data-lang="en">SIGN IN</span></RouterLink>
          <RouterLink v-else class="llmf-header-action llmf-header-action-primary" :to="dashboardPath"><span data-lang="zh">进入控制台</span><span data-lang="en">OPEN CONSOLE</span></RouterLink>
        </div>
      </div>
    </div>
  </header>

  <main v-if="props.page === 'home'">
    <section class="llmf-hero">
      <div class="llmf-wrap">
        <div class="llmf-hero-grid">
          <div class="llmf-hero-copy">
            <div class="llmf-kicker">
              <span data-lang="zh">专业 AI API 中转站</span>
              <span data-lang="en">Professional AI API Gateway</span>
            </div>

            <h1 data-lang="zh">让每一次模型调用，<br>都有<em>可靠的路径</em>。</h1>
            <h1 data-lang="en">A reliable path<br>for every <em>model call</em>.</h1>

            <p class="llmf-lead" data-lang="zh">
              LLM Free 是面向开发者与团队的专业 AI API 中转站，提供统一、稳定、可观测的大模型接口。通过一个兼容主流协议的入口连接 Claude、GPT、Gemini 等模型，把复杂的上游接入留在平台，把清晰可靠的 API 交给你。
            </p>
            <p class="llmf-lead" data-lang="en">
              LLM Free is a professional AI API gateway for developers and teams, providing unified, stable, and observable access to leading models. Connect Claude, GPT, Gemini, and more through one compatible endpoint while we manage the upstream complexity.
            </p>

            <div class="llmf-actions">
              <RouterLink class="llmf-button llmf-button-primary" :to="primaryActionPath">
                <span data-lang="zh">{{ isAuthenticated ? '进入控制台' : '开始使用' }}</span><span data-lang="en">{{ isAuthenticated ? 'OPEN CONSOLE' : 'GET STARTED' }}</span><span class="llmf-arrow">→</span>
              </RouterLink>
            </div>
          </div>

          <aside class="llmf-gateway-demo" aria-label="LLM Free API gateway routing example">
            <div class="llmf-terminal-bar">
              <span class="llmf-terminal-dots" aria-hidden="true"><i></i><i></i><i></i></span>
              <span>gateway.route</span>
              <span>HTTPS</span>
            </div>
            <pre class="llmf-gateway-code"><span class="llmf-code-method">POST</span> <span class="llmf-code-path">https://api.llmfree.work/v1/chat/completions</span>

<span class="llmf-code-key">model</span>: <span class="llmf-code-value">"your model"</span>
<span class="llmf-code-key">authorization</span>: <span class="llmf-code-value">"Bearer sk-llmf••••"</span></pre>
            <div class="llmf-route-trace">
              <div class="llmf-route-row">
                <span class="llmf-route-index">01</span>
                <span><strong data-lang="zh">客户端请求</strong><strong data-lang="en">Client request</strong><small>OpenAI-compatible protocol</small></span>
                <span class="llmf-route-badge">IN</span>
              </div>
              <div class="llmf-route-row">
                <span class="llmf-route-index">02</span>
                <span><strong>LLM Free Gateway</strong><small data-lang="zh">鉴权 · 计费 · 智能路由</small><small data-lang="en">Auth · billing · smart routing</small></span>
                <span class="llmf-route-badge">ROUTE</span>
              </div>
              <div class="llmf-route-row">
                <span class="llmf-route-index">03</span>
                <span><strong data-lang="zh">上游模型</strong><strong data-lang="en">Upstream models</strong><small data-lang="zh">统一接口，按需切换</small><small data-lang="en">One API, switch as needed</small></span>
                <span class="llmf-route-providers">CLAUDE<br>OPENAI<br>GEMINI</span>
              </div>
            </div>
            <div class="llmf-route-status"><span data-lang="zh">路由正常</span><span data-lang="en">Route healthy</span><span>48 ms</span></div>
          </aside>
        </div>

        <div class="llmf-proof">
          <div class="llmf-proof-item">
            <div class="llmf-proof-label"><span data-lang="zh">协议</span><span data-lang="en">Protocol</span></div>
            <div class="llmf-proof-value" data-lang="zh">兼容 OpenAI API</div>
            <div class="llmf-proof-value" data-lang="en">OpenAI Compatible</div>
          </div>
          <div class="llmf-proof-item">
            <div class="llmf-proof-label"><span data-lang="zh">模型接入</span><span data-lang="en">Model Access</span></div>
            <div class="llmf-proof-value" data-lang="zh">主流大模型统一入口</div>
            <div class="llmf-proof-value" data-lang="en">One gateway, leading models</div>
          </div>
          <div class="llmf-proof-item">
            <div class="llmf-proof-label"><span data-lang="zh">可观测性</span><span data-lang="en">Visibility</span></div>
            <div class="llmf-proof-value" data-lang="zh">用量与成本可观测</div>
            <div class="llmf-proof-value" data-lang="en">Observable by design</div>
          </div>
        </div>

        <div class="llmf-model-wall" aria-label="Supported model providers">
          <div class="llmf-model-brand">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 2v20M4.9 4.9l14.2 14.2M2 12h20M4.9 19.1 19.1 4.9"></path></svg>
            <span>Claude</span>
          </div>
          <div class="llmf-model-brand">
            <img src="/llmfree/openai-logo.svg" alt="OpenAI Logo">
            <span>OpenAI</span>
          </div>
          <div class="llmf-model-brand llmf-model-brand-gemini">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 2c.8 5.8 4.2 9.2 10 10-5.8.8-9.2 4.2-10 10-.8-5.8-4.2-9.2-10-10 5.8-.8 9.2-4.2 10-10Z"></path></svg>
            <span>Gemini</span>
          </div>
        </div>
      </div>
    </section>

    <section class="llmf-section" id="llmf-about">
      <div class="llmf-wrap">
        <div class="llmf-section-head">
          <div class="llmf-index"><span data-lang="zh">01 / 平台使命</span><span data-lang="en">01 / MISSION</span></div>
          <div>
            <h2 class="llmf-section-title" data-lang="zh">一个入口，连接主流大模型。</h2>
            <h2 class="llmf-section-title" data-lang="en">One gateway to leading AI models.</h2>
            <p class="llmf-section-copy" data-lang="zh">统一管理模型接入、API 密钥、请求用量与账户成本。</p>
            <p class="llmf-section-copy" data-lang="en">Manage model access, API keys, usage, and cost from one place.</p>
          </div>
        </div>

        <div class="llmf-capabilities">
          <article class="llmf-capability">
            <div class="llmf-capability-icon"><svg viewBox="0 0 24 24"><path d="M4 7h16M4 17h16M8 3v8M16 13v8"></path></svg></div>
            <h3 data-lang="zh">统一接口</h3><h3 data-lang="en">One endpoint</h3>
            <p data-lang="zh">兼容主流协议，快速切换不同模型。</p>
            <p data-lang="en">Switch models through familiar API conventions.</p>
          </article>
          <article class="llmf-capability">
            <div class="llmf-capability-icon"><svg viewBox="0 0 24 24"><path d="M4 19V9M10 19V5M16 19v-7M22 19H2"></path></svg></div>
            <h3 data-lang="zh">透明用量</h3><h3 data-lang="en">Clear usage</h3>
            <p data-lang="zh">请求、Token 与成本清晰可查。</p>
            <p data-lang="en">Track requests, tokens, and cost clearly.</p>
          </article>
          <article class="llmf-capability">
            <div class="llmf-capability-icon"><svg viewBox="0 0 24 24"><path d="M12 3 4 6v5c0 5 3.4 8.5 8 10 4.6-1.5 8-5 8-10V6l-8-3Z"></path><path d="m9 12 2 2 4-4"></path></svg></div>
            <h3 data-lang="zh">持续运行</h3><h3 data-lang="en">Operational focus</h3>
            <p data-lang="zh">面向稳定性、可观测性与故障恢复。</p>
            <p data-lang="en">Built for stability, visibility, and recovery.</p>
          </article>
        </div>
      </div>
    </section>

    <section class="llmf-section" id="llmf-technology">
      <div class="llmf-wrap">
        <div class="llmf-section-head">
          <div class="llmf-index"><span data-lang="zh">02 / 技术架构</span><span data-lang="en">02 / SYSTEM</span></div>
          <div>
            <h2 class="llmf-section-title llmf-section-title-compact" data-lang="zh">熟悉的协议，明确的边界，足够简单的接入。</h2>
            <h2 class="llmf-section-title llmf-section-title-compact" data-lang="en">Familiar protocols, clear boundaries, and a simpler integration.</h2>
          </div>
        </div>

        <div class="llmf-architecture">
          <div class="llmf-flow" aria-label="API request flow">
            <div class="llmf-flow-node">
              <small><span data-lang="zh">01 / 客户端</span><span data-lang="en">01 / Client</span></small>
              <strong data-lang="zh">你的应用</strong><strong data-lang="en">Your app</strong>
              <span>SDK · CLI · Agent</span>
            </div>
            <div class="llmf-flow-arrow">→</div>
            <div class="llmf-flow-node llmf-flow-node-main">
              <small><span data-lang="zh">02 / API 网关</span><span data-lang="en">02 / Gateway</span></small>
              <strong>LLM Free</strong>
              <span data-lang="zh">认证 · 路由 · 计量</span><span data-lang="en">Auth · Route · Meter</span>
            </div>
            <div class="llmf-flow-arrow">→</div>
            <div class="llmf-flow-node">
              <small><span data-lang="zh">03 / 模型厂商</span><span data-lang="en">03 / Models</span></small>
              <strong>AI Providers</strong>
              <span>Claude · GPT · Gemini</span>
            </div>
          </div>

          <div class="llmf-code">
            <div class="llmf-code-head"><span><span data-lang="zh">请求示例</span><span data-lang="en">request.sh</span></span><span>HTTPS / JSON</span></div>
            <pre><span class="llmf-comment"># OpenAI-compatible request</span>
curl https://api.llmfree.work/v1/chat/completions \
  -H <span class="llmf-value">"Authorization: Bearer $API_KEY"</span> \
  -H <span class="llmf-value">"Content-Type: application/json"</span> \
  -d '{
    <span class="llmf-key">"model"</span>: <span class="llmf-value">"your-model"</span>,
    <span class="llmf-key">"messages"</span>: [{
      <span class="llmf-key">"role"</span>: <span class="llmf-value">"user"</span>,
      <span class="llmf-key">"content"</span>: <span class="llmf-value">"Hello"</span>
    }]
  }'</pre>
          </div>
        </div>
      </div>
    </section>

    <section class="llmf-section" id="llmf-principles">
      <div class="llmf-wrap llmf-principles">
        <blockquote class="llmf-quote" data-lang="zh">技术服务的可信度，不来自夸张承诺，而来自清晰、克制与持续可验证。</blockquote>
        <blockquote class="llmf-quote" data-lang="en">Trust in infrastructure comes not from bold claims, but from clarity, restraint, and verifiable operation.</blockquote>

        <div class="llmf-principle-list">
          <div class="llmf-principle">
            <strong><span data-lang="zh">清晰</span><span data-lang="en">Clarity</span></strong>
            <span data-lang="zh">清晰展示接口、用量与运行状态，减少不可解释的黑盒。</span>
            <span data-lang="en">Make interfaces, usage, and operational state understandable.</span>
          </div>
          <div class="llmf-principle">
            <strong><span data-lang="zh">可控</span><span data-lang="en">Control</span></strong>
            <span data-lang="zh">密钥、额度与访问路径由你掌握，适配真实开发流程。</span>
            <span data-lang="en">Keep keys, budgets, and access paths under your control.</span>
          </div>
          <div class="llmf-principle">
            <strong><span data-lang="zh">持续</span><span data-lang="en">Continuity</span></strong>
            <span data-lang="zh">以持续可用为目标，把稳定运行视为基础能力而非附加功能。</span>
            <span data-lang="en">Treat dependable operation as a foundation, not an optional feature.</span>
          </div>
        </div>
      </div>
    </section>

    <section class="llmf-section" id="llmf-faq">
      <div class="llmf-wrap llmf-faq">
        <div>
          <div class="llmf-index"><span data-lang="zh">04 / 常见问题</span><span data-lang="en">04 / FAQ</span></div>
          <h2 class="llmf-section-title" data-lang="zh">关于 LLM Free</h2>
          <h2 class="llmf-section-title" data-lang="en">About LLM Free</h2>
          <p class="llmf-section-copy" data-lang="zh">关于 AI API 中转、模型支持与试用额度的简要说明。</p>
          <p class="llmf-section-copy" data-lang="en">Direct answers about the API gateway, supported models, and trial credit.</p>
        </div>

        <div class="llmf-faq-list">
          <details open>
            <summary><span data-lang="zh">LLM Free 是什么？</span><span data-lang="en">What is LLM Free?</span></summary>
            <p class="llmf-faq-answer" data-lang="zh">LLM Free 是一个兼容 OpenAI API 格式的 AI API 中转站，为开发者和团队提供统一的大模型访问入口、API 密钥管理、用量统计与成本查看。</p>
            <p class="llmf-faq-answer" data-lang="en">LLM Free is an OpenAI-compatible AI API gateway with unified model access, API key management, usage statistics, and cost visibility.</p>
          </details>
          <details>
            <summary><span data-lang="zh">支持哪些大模型？</span><span data-lang="en">Which models are supported?</span></summary>
            <p class="llmf-faq-answer" data-lang="zh">平台可接入 Claude、GPT、Gemini 等主流模型，实际可用模型以模型广场展示为准。</p>
            <p class="llmf-faq-answer" data-lang="en">The gateway supports Claude, GPT, Gemini, and other leading models. Current availability is listed in the model plaza.</p>
          </details>
          <details>
            <summary><span data-lang="zh">如何领取试用额度？</span><span data-lang="en">How do I get trial credit?</span></summary>
            <p class="llmf-faq-answer" data-lang="zh">注册后加入 LLM Free QQ群或 Telegram 群，按照群内说明即可申请试用额度。</p>
            <p class="llmf-faq-answer" data-lang="en">Register, then join the LLM Free QQ or Telegram group and follow the community instructions to request trial credit.</p>
          </details>
          <details>
            <summary><span data-lang="zh">如何联系商务支持？</span><span data-lang="en">How can I contact business support?</span></summary>
            <p class="llmf-faq-answer" data-lang="zh">商务合作、企业接入与定制需求请联系 QQ：751077517。</p>
            <p class="llmf-faq-answer" data-lang="en">For business cooperation, enterprise access, or custom requirements, contact QQ: 751077517.</p>
          </details>
        </div>
      </div>
    </section>
  </main>

  <main v-else class="llmf-model-plaza-page">
    <div class="llmf-wrap">
      <section class="llmf-model-plaza-intro">
        <div class="llmf-model-plaza-copy">
          <div class="llmf-kicker"><span data-lang="zh">模型目录</span><span data-lang="en">MODEL DIRECTORY</span></div>
          <h1><span data-lang="zh">模型广场</span><span data-lang="en">Model Plaza</span></h1>
          <p data-lang="zh">按分组浏览可用模型，对比输入、缓存与输出价格。</p>
          <p data-lang="en">Browse available models by group and compare input, cache, and output prices.</p>
        </div>

        <figure class="llmf-model-plaza-visual" aria-label="OpenAI、Claude 与 Gemini 通过 LLM Free 统一 API 接入">
          <div class="llmf-visual-grid" aria-hidden="true"></div>
          <div class="llmf-visual-flow" aria-hidden="true">
            <span></span><span></span><span></span>
          </div>

          <div class="llmf-model-node llmf-model-node-openai">
            <span class="llmf-model-logo llmf-model-logo-openai">
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M21.55 10.004a5.416 5.416 0 0 0-.478-4.501c-1.217-2.09-3.662-3.166-6.05-2.66A5.59 5.59 0 0 0 10.831 1C8.39.995 6.224 2.546 5.473 4.838A5.553 5.553 0 0 0 1.76 7.496a5.487 5.487 0 0 0 .691 6.5 5.416 5.416 0 0 0 .477 4.502c1.217 2.09 3.662 3.165 6.05 2.66A5.586 5.586 0 0 0 13.168 23c2.443.006 4.61-1.546 5.361-3.84a5.553 5.553 0 0 0 3.715-2.66 5.488 5.488 0 0 0-.693-6.497v.001Zm-8.381 11.558a4.199 4.199 0 0 1-2.675-.954l.132-.074 4.44-2.53a.71.71 0 0 0 .364-.623v-6.176l1.877 1.069c.02.01.033.029.036.05v5.115c-.003 2.274-1.87 4.118-4.174 4.123ZM4.192 17.78a4.059 4.059 0 0 1-.498-2.763l.131.078 4.44 2.53c.225.13.504.13.73 0l5.42-3.088v2.138a.068.068 0 0 1-.027.057L9.9 19.288c-1.999 1.136-4.552.46-5.707-1.51ZM3.023 8.216A4.15 4.15 0 0 1 5.198 6.41l-.002.151v5.06c0 .26.139.5.364.624l5.42 3.087-1.876 1.07a.067.067 0 0 1-.063.005l-4.489-2.559c-1.995-1.14-2.679-3.658-1.53-5.63Zm15.417 3.54-5.42-3.088L14.896 7.6a.067.067 0 0 1 .063-.006l4.489 2.557c1.998 1.14 2.683 3.662 1.529 5.633a4.163 4.163 0 0 1-2.174 1.807V12.38a.71.71 0 0 0-.363-.623Zm1.867-2.773-.132-.078-4.44-2.53a.731.731 0 0 0-.729 0l-5.42 3.088V7.325c0-.023.01-.044.027-.057L14.1 4.713c2-1.137 4.555-.46 5.707 1.513.487.833.664 1.809.499 2.757ZM8.566 12.793l-1.877-1.068a.065.065 0 0 1-.036-.051V6.559c.001-2.277 1.873-4.122 4.181-4.12.976 0 1.92.338 2.671.954l-.131.073-4.44 2.53a.71.71 0 0 0-.365.623l-.003 6.174Zm1.02-2.168L12 9.25l2.414 1.375v2.75L12 14.75l-2.415-1.375v-2.75Z"/></svg>
            </span>
            <span><strong>OpenAI</strong><small>GPT · API</small></span>
          </div>

          <div class="llmf-model-node llmf-model-node-claude">
            <span class="llmf-model-logo llmf-model-logo-claude">
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4.709 15.955l4.72-2.647.08-.23-.08-.128H9.2l-.79-.048-2.698-.073-2.339-.097-2.266-.122-.571-.121L0 11.784l.055-.352.48-.321.686.06 1.52.103 2.278.158 1.652.097 2.449.255h.389l.055-.157-.134-.098-.103-.097-2.358-1.596-2.552-1.688-1.336-.972-.724-.491-.364-.462-.158-1.008.656-.722.881.06.225.061.893.686 1.908 1.476 2.491 1.833.365.304.145-.103.019-.073-.164-.274-1.355-2.446-1.446-2.49-.644-1.032-.17-.619a2.97 2.97 0 0 1-.104-.729L6.283.134 6.696 0l.996.134.42.364.62 1.414 1.002 2.229 1.555 3.03.456.898.243.832.091.255h.158V9.01l.128-1.706.237-2.095.23-2.695.08-.76.376-.91.747-.492.584.28.48.685-.067.444-.286 1.851-.559 2.903-.364 1.942h.212l.243-.242.985-1.306 1.652-2.064.73-.82.85-.904.547-.431h1.033l.76 1.129-.34 1.166-1.064 1.347-.881 1.142-1.264 1.7-.79 1.36.073.11.188-.02 2.856-.606 1.543-.28 1.841-.315.833.388.091.395-.328.807-1.969.486-2.309.462-3.439.813-.042.03.049.061 1.549.146.662.036h1.622l3.02.225.79.522.474.638-.079.485-1.215.62-1.64-.389-3.829-.91-1.312-.329h-.182v.11l1.093 1.068 2.006 1.81 2.509 2.33.127.578-.322.455-.34-.049-2.205-1.657-.851-.747-1.926-1.62h-.128v.17l.444.649 2.345 3.521.122 1.08-.17.353-.608.213-.668-.122-1.374-1.925-1.415-2.167-1.143-1.943-.14.08-.674 7.254-.316.37-.729.28-.607-.461-.322-.747.322-1.476.389-1.924.315-1.53.286-1.9.17-.632-.012-.042-.14.018-1.434 1.967-2.18 2.945-1.726 1.845-.414.164-.717-.37.067-.662.401-.589 2.388-3.036 1.44-1.882.93-1.086-.006-.158h-.055L4.132 18.56l-1.13.146-.487-.456.061-.746.231-.243 1.908-1.312-.006.006Z"/></svg>
            </span>
            <span><strong>Claude</strong><small>Anthropic · API</small></span>
          </div>

          <div class="llmf-model-node llmf-model-node-gemini">
            <span class="llmf-model-logo llmf-model-logo-gemini">
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M20.616 10.835a14.147 14.147 0 0 1-4.45-3.001 14.111 14.111 0 0 1-3.678-6.452.503.503 0 0 0-.975 0 14.134 14.134 0 0 1-3.679 6.452 14.155 14.155 0 0 1-4.45 3.001c-.65.28-1.318.505-2.002.678a.502.502 0 0 0 0 .975c.684.172 1.35.397 2.002.677a14.147 14.147 0 0 1 4.45 3.001 14.112 14.112 0 0 1 3.679 6.453.502.502 0 0 0 .975 0c.172-.685.397-1.351.677-2.003a14.145 14.145 0 0 1 3.001-4.45 14.113 14.113 0 0 1 6.453-3.678.503.503 0 0 0 0-.975 13.245 13.245 0 0 1-2.003-.678Z"/></svg>
            </span>
            <span><strong>Gemini</strong><small>Google · API</small></span>
          </div>

          <div class="llmf-gateway-node">
            <span class="llmf-gateway-pulse" aria-hidden="true"></span>
            <img src="/llmfree/llmfree-logo-transparent.png" alt="">
            <span><strong>LLM Free</strong><small>Unified API Gateway</small></span>
          </div>
          <div class="llmf-visual-endpoint"><span>POST</span><code>/v1/chat/completions</code></div>
        </figure>
      </section>
      <ModelPlazaContent variant="home" :language="language" />
    </div>
  </main>

  <footer class="llmf-footer">
    <div v-if="props.page === 'home'" class="llmf-wrap llmf-community-cta">
      <div>
        <h2 data-lang="zh">加入社群，领取试用额度</h2>
        <h2 data-lang="en">Join the community and claim trial credit</h2>
        <p data-lang="zh">注册后加入 QQ群或 Telegram 群申请试用；商务支持请联系 QQ：751077517。</p>
        <p data-lang="en">Join our QQ or Telegram group after registration. Business support QQ: 751077517.</p>
      </div>
      <div class="llmf-community-actions">
        <span class="llmf-button llmf-button-primary" style="position:relative"><span data-lang="zh">加入QQ群</span><span data-lang="en">Join QQ group</span><a class="llmf-official-overlay" target="_blank" href="https://qm.qq.com/cgi-bin/qm/qr?k=4m4LsIHtnUmdqd-G3vWiPNLVq9g_fx-a&amp;jump_from=webapi&amp;authKey=PAe3IwrmVErvv6/3dyjOj7t2gHCXz2GfmlG9nS2PuUWcaAJ3iQ8lqit6nM3q9EgR"><img border="0" src="//pub.idqqimg.com/wpa/images/group.png" alt="LLM-Free售后" title="LLM-Free售后"></a></span>
        <a class="llmf-button" target="_blank" rel="noopener noreferrer" href="https://t.me/+nqFUXD_y66liYTll">Telegram</a>
        <button class="llmf-button llmf-copy-button" type="button" aria-label="复制商务支持QQ号" @click="copyBusinessQq">
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8.4 10.1c.2-3.1 1.5-5.2 3.6-5.2s3.4 2.1 3.6 5.2c1.4 1.2 2.2 3 2.2 5 0 1-.2 1.9-.6 2.7-.5-.6-1.1-1-1.8-1.3-.8 1.1-2 1.8-3.4 1.8s-2.6-.7-3.4-1.8c-.7.3-1.3.7-1.8 1.3-.4-.8-.6-1.7-.6-2.7 0-2 .8-3.8 2.2-5Z"></path><path d="M9.2 20.2c.8.4 1.8.7 2.8.7s2-.3 2.8-.7"></path></svg>
          <span>751077517</span>
        </button>
      </div>
    </div>

    <div class="llmf-wrap llmf-footer-inner">
      <div>
        <RouterLink class="llmf-brand" to="/home">
          <span class="llmf-mark"><img src="/llmfree/llmfree-logo-transparent.png" alt="LLM Free Logo"></span>
          <span class="llmf-brand-copy">
            <span class="llmf-brand-name">LLM Free</span>
            <span class="llmf-footer-note"><span data-lang="zh">专业 AI API 中转站</span><span data-lang="en">PROFESSIONAL AI API GATEWAY</span></span>
          </span>
        </RouterLink>
      </div>
      <div class="llmf-footer-links">
        <RouterLink v-if="isAuthenticated" :to="dashboardPath"><span data-lang="zh">控制台</span><span data-lang="en">CONSOLE</span></RouterLink>
        <RouterLink v-if="!isAuthenticated" to="/register"><span data-lang="zh">注册</span><span data-lang="en">SIGN UP</span></RouterLink>
        <RouterLink v-if="!isAuthenticated" to="/login"><span data-lang="zh">登录</span><span data-lang="en">SIGN IN</span></RouterLink>
        <span>© 2026 LLM Free</span>
      </div>
    </div>
  </footer>

  <div class="llmf-copy-toast" :class="{ 'is-visible': copyToastVisible }" role="status" aria-live="polite">
    <span data-lang="zh">复制成功</span><span data-lang="en">Copied</span>
  </div>
</div>
</template>

<style scoped>
.llmf-control {
    position: fixed;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip: rect(0 0 0 0);
    clip-path: inset(50%);
    white-space: nowrap;
  }

  .llmf-page {
    --paper: #fbfcfa;
    --surface: #ffffff;
    --ink: #171918;
    --muted: #636965;
    --line: #dfe3df;
    --line-strong: #b8c0ba;
    --forest: #285943;
    --forest-soft: #e8f0eb;
    --burgundy: #7c2d3e;
    --code: #f1f3f1;
    --shadow: rgba(21, 29, 24, 0.08);
    min-height: 100vh;
    overflow: hidden;
    color: var(--ink);
    background-color: var(--paper);
    font-family: Inter, ui-sans-serif, -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif;
    transition: color 180ms ease, background-color 180ms ease;
  }

  .llmf-page[data-theme="dark"] {
    --paper: #111412;
    --surface: #181c19;
    --ink: #f2f4f2;
    --muted: #a8b0aa;
    --line: #252b27;
    --line-strong: #414a44;
    --forest: #9cc9af;
    --forest-soft: #203229;
    --burgundy: #d998a7;
    --code: #151916;
    --shadow: rgba(0, 0, 0, 0.24);
  }

  .llmf-page[data-language="zh"] [data-lang="en"],
  .llmf-page[data-language="en"] [data-lang="zh"] {
    display: none !important;
  }

  .llmf-page[data-language="zh"] .llmf-kicker,
  .llmf-page[data-language="zh"] .llmf-brand-note,
  .llmf-page[data-language="zh"] .llmf-proof-label,
  .llmf-page[data-language="zh"] .llmf-index,
  .llmf-page[data-language="zh"] .llmf-flow-node small,
  .llmf-page[data-language="zh"] .llmf-footer-note {
    font-size: 12px;
  }

  .llmf-page *,
  .llmf-page *::before,
  .llmf-page *::after {
    box-sizing: border-box;
  }

  .llmf-page a {
    color: inherit;
    text-decoration: none;
  }

  .llmf-wrap {
    width: min(1120px, calc(100% - 48px));
    margin: 0 auto;
  }

  .llmf-header {
    position: relative;
    z-index: 10;
    border-bottom: 1px solid var(--line);
    background: color-mix(in srgb, var(--paper) 92%, transparent);
    backdrop-filter: blur(14px);
  }

  .llmf-nav {
    display: flex;
    min-height: 72px;
    align-items: center;
    justify-content: space-between;
    gap: 24px;
  }

  .llmf-brand {
    display: inline-flex;
    min-width: 0;
    align-items: center;
    gap: 11px;
  }

  .llmf-mark {
    display: block;
    width: 42px;
    height: 42px;
    flex: 0 0 42px;
    overflow: hidden;
    padding: 3px;
    border: 1px solid var(--line-strong);
    border-radius: 6px;
    background: transparent;
  }

  .llmf-mark img {
    display: block;
    width: 100%;
    height: 100%;
    object-fit: contain;
    border-radius: 3px;
  }

  .llmf-page[data-theme="dark"] .llmf-mark img {
    filter: invert(1);
  }

  .llmf-brand-copy {
    display: grid;
    gap: 1px;
  }

  .llmf-brand-name {
    font-family: Georgia, "Times New Roman", serif;
    font-size: 18px;
    font-weight: 700;
  }

  .llmf-brand-note {
    color: var(--muted);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 9px;
    text-transform: uppercase;
  }

  .llmf-nav-right,
  .llmf-nav-links,
  .llmf-switches,
  .llmf-lang-switch,
  .llmf-auth-actions {
    display: flex;
    align-items: center;
  }

  .llmf-nav-right {
    gap: 24px;
  }

  .llmf-nav-links {
    gap: 8px;
    color: var(--muted);
    font-size: 13px;
  }

  .llmf-nav-links > a,
  .llmf-nav-community {
    position: relative;
    display: inline-flex;
    min-height: 36px;
    align-items: center;
    gap: 7px;
    padding: 0 10px;
    border: 1px solid transparent;
    border-radius: 6px;
    transition: color 150ms ease, border-color 150ms ease, background 150ms ease;
  }

  .llmf-nav-links > a:hover,
  .llmf-nav-community:hover {
    border-color: var(--line);
    background: var(--surface);
    color: var(--ink);
  }

  .llmf-nav-tooltip::after {
    position: absolute;
    z-index: 5;
    top: calc(100% + 9px);
    left: 50%;
    width: max-content;
    max-width: 180px;
    padding: 6px 8px;
    border: 1px solid var(--line);
    border-radius: 5px;
    background: var(--surface);
    box-shadow: 0 8px 20px var(--shadow);
    color: var(--ink);
    content: attr(data-tooltip-zh);
    font-size: 11px;
    line-height: 1.4;
    opacity: 0;
    pointer-events: none;
    transform: translate(-50%, -3px);
    transition: opacity 140ms ease, transform 140ms ease;
  }

  .llmf-page[data-language="en"] .llmf-nav-tooltip::after {
    content: attr(data-tooltip-en);
  }

  .llmf-nav-tooltip:hover::after {
    opacity: 1;
    transform: translate(-50%, 0);
  }

  .llmf-nav-icon {
    display: grid;
    width: 17px;
    height: 17px;
    flex: 0 0 17px;
    place-items: center;
    color: var(--forest);
  }

  .llmf-nav-icon svg {
    display: block;
    width: 17px;
    height: 17px;
    fill: none;
    stroke: currentColor;
    stroke-linecap: round;
    stroke-linejoin: round;
    stroke-width: 1.8;
  }

  .llmf-nav-icon-telegram svg {
    fill: currentColor;
    stroke: none;
  }

  .llmf-official-overlay {
    position: absolute;
    z-index: 2;
    inset: 0;
    display: block;
    overflow: hidden;
    opacity: 0;
  }

  .llmf-official-overlay img {
    width: 100%;
    height: 100%;
  }

  .llmf-switches {
    gap: 8px;
  }

  .llmf-dropdown {
    position: relative;
  }

  .llmf-dropdown summary {
    display: grid;
    width: 34px;
    height: 34px;
    place-items: center;
    border: 1px solid var(--line);
    border-radius: 6px;
    background: var(--surface);
    color: var(--muted);
    cursor: pointer;
    list-style: none;
    user-select: none;
  }

  .llmf-dropdown summary::-webkit-details-marker { display: none; }

  .llmf-dropdown[open] summary,
  .llmf-dropdown summary:hover {
    border-color: var(--forest);
    color: var(--forest);
  }

  .llmf-dropdown svg {
    width: 15px;
    height: 15px;
    fill: none;
    stroke: currentColor;
    stroke-linecap: round;
    stroke-linejoin: round;
    stroke-width: 1.7;
  }

  .llmf-language-current {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 11px;
    font-weight: 700;
  }

  .llmf-dropdown-menu {
    position: absolute;
    z-index: 20;
    top: calc(100% + 8px);
    right: 0;
    display: grid;
    min-width: 142px;
    padding: 5px;
    border: 1px solid var(--line);
    border-radius: 6px;
    background: var(--surface);
    box-shadow: 0 12px 30px var(--shadow);
  }

  .llmf-dropdown-option {
    display: flex;
    min-height: 34px;
    align-items: center;
    gap: 9px;
    padding: 0 9px;
    border: 0;
    border-radius: 4px;
    background: transparent;
    color: var(--ink);
    cursor: pointer;
    font: inherit;
    font-size: 12px;
    text-align: left;
  }

  .llmf-dropdown-option:hover,
  .llmf-dropdown-option.is-active {
    background: var(--forest-soft);
    color: var(--forest);
  }

  .llmf-theme-current > span {
    display: none;
  }

  .llmf-page[data-theme-choice="system"] .llmf-theme-current-system,
  .llmf-page[data-theme-choice="light"] .llmf-theme-current-light,
  .llmf-page[data-theme-choice="dark"] .llmf-theme-current-dark {
    display: grid;
  }

  .llmf-auth-actions {
    gap: 7px;
  }

  .llmf-page .llmf-header-action {
    display: inline-flex;
    min-height: 34px;
    align-items: center;
    justify-content: center;
    padding: 0 11px;
    border: 1px solid var(--line);
    border-radius: 6px;
    background: var(--surface);
    color: var(--ink);
    font-size: 12px;
    font-weight: 650;
  }

  .llmf-page .llmf-header-action-primary {
    border-color: var(--forest);
    background: var(--forest);
    color: #ffffff;
  }

  .llmf-page[data-theme="dark"] .llmf-header-action-primary {
    color: #111412;
  }

  .llmf-auth-guest {
    display: inline-flex;
  }

  .llmf-auth-logged-in {
    display: none;
  }

  .llmf-page[data-auth="authenticated"] .llmf-auth-guest {
    display: none;
  }

  .llmf-page[data-auth="authenticated"] .llmf-auth-logged-in {
    display: inline-flex;
  }

  .llmf-hero {
    display: flex;
    min-height: 620px;
    flex-direction: column;
    justify-content: center;
    padding: 84px 0 68px;
    border-bottom: 1px solid var(--line);
    background: color-mix(in srgb, var(--paper) 90%, transparent);
  }

  .llmf-hero-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(300px, 400px);
    align-items: center;
    gap: clamp(42px, 6vw, 82px);
  }

  .llmf-hero-copy {
    min-width: 0;
  }

  .llmf-gateway-demo {
    width: min(100%, 400px);
    min-width: 0;
    margin: 0;
    justify-self: end;
    overflow: hidden;
    border: 1px solid color-mix(in srgb, var(--burgundy) 58%, #202925);
    border-radius: 8px;
    background: #111715;
    color: #e8efe9;
    box-shadow: 12px 14px 0 color-mix(in srgb, var(--burgundy) 24%, transparent);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  }

  .llmf-terminal-bar {
    display: flex;
    min-height: 42px;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 0 14px;
    border-bottom: 1px solid #29332f;
    background: #171e1b;
    color: #8e9c94;
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
  }

  .llmf-terminal-dots {
    display: flex;
    gap: 5px;
  }

  .llmf-terminal-dots i {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: #445049;
  }

  .llmf-terminal-dots i:first-child { background: #d998a7; }
  .llmf-terminal-dots i:nth-child(2) { background: #d8bd77; }
  .llmf-terminal-dots i:last-child { background: #82b99a; }

  .llmf-gateway-code {
    display: block;
    margin: 0;
    padding: 21px 20px 19px;
    border-bottom: 1px solid #29332f;
    color: #b8c5bd;
    font-size: 11px;
    line-height: 1.85;
    white-space: pre-wrap;
  }

  .llmf-code-method { color: #d998a7; }
  .llmf-code-path { color: #f2f5f3; }
  .llmf-code-key { color: #82b99a; }
  .llmf-code-value { color: #d8bd77; }

  .llmf-route-trace {
    padding: 0 20px;
  }

  .llmf-route-row {
    display: grid;
    grid-template-columns: 26px minmax(0, 1fr) auto;
    min-height: 64px;
    align-items: center;
    gap: 12px;
    border-bottom: 1px solid #29332f;
  }

  .llmf-route-index {
    color: #637169;
    font-size: 10px;
  }

  .llmf-route-row strong {
    display: block;
    color: #edf3ef;
    font-size: 11px;
    font-weight: 700;
  }

  .llmf-route-row small {
    display: block;
    margin-top: 4px;
    color: #7f8d85;
    font-size: 9px;
  }

  .llmf-route-badge {
    min-width: 36px;
    padding: 4px 6px;
    border: 1px solid #3b4b42;
    border-radius: 4px;
    color: #9cc9af;
    font-size: 9px;
    text-align: center;
  }

  .llmf-route-providers {
    color: #d8bd77;
    font-size: 9px;
    text-align: right;
  }

  .llmf-route-status {
    display: flex;
    min-height: 42px;
    align-items: center;
    gap: 8px;
    padding: 0 20px;
    color: #84928a;
    font-size: 9px;
    font-weight: 700;
    text-transform: uppercase;
  }

  .llmf-route-status::before {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: #82b99a;
    box-shadow: 0 0 0 4px rgba(130, 185, 154, 0.12);
    content: "";
  }

  .llmf-route-status span:last-child {
    margin-left: auto;
    color: #d998a7;
  }

  .llmf-page[data-language="zh"] .llmf-gateway-demo [data-lang="zh"] {
    font-family: Inter, ui-sans-serif, -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif;
    -webkit-font-smoothing: antialiased;
    text-rendering: optimizeLegibility;
  }

  .llmf-page[data-language="zh"] .llmf-route-row strong[data-lang="zh"] {
    font-size: 13px;
  }

  .llmf-page[data-language="zh"] .llmf-route-row small[data-lang="zh"] {
    margin-top: 5px;
    font-size: 11px;
    line-height: 1.45;
  }

  .llmf-page[data-language="zh"] .llmf-route-status [data-lang="zh"] {
    font-size: 11px;
  }

  .llmf-kicker {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 26px;
    color: var(--forest);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 11px;
    font-weight: 700;
    text-transform: uppercase;
  }

  .llmf-kicker::before {
    width: 28px;
    height: 1px;
    background: var(--forest);
    content: "";
  }

  .llmf-hero h1 {
    max-width: 720px;
    margin: 0;
    font-family: Georgia, "Noto Serif SC", "Songti SC", serif;
    font-size: 58px;
    font-weight: 500;
    line-height: 1.18;
  }

  .llmf-hero h1 em {
    color: var(--forest);
    font-style: normal;
  }

  .llmf-lead {
    max-width: 660px;
    margin: 28px 0 0;
    color: var(--muted);
    font-size: 17px;
    line-height: 1.9;
  }

  .llmf-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
    margin-top: 34px;
  }

  .llmf-button {
    display: inline-flex;
    min-height: 44px;
    align-items: center;
    justify-content: center;
    gap: 9px;
    padding: 0 18px;
    border: 1px solid var(--line-strong);
    border-radius: 6px;
    background: var(--surface);
    color: var(--ink);
    font-size: 13px;
    font-weight: 650;
    transition: transform 150ms ease, border-color 150ms ease, background 150ms ease;
  }

  .llmf-button:hover {
    border-color: var(--forest);
    transform: translateY(-1px);
  }

  .llmf-page .llmf-button-primary {
    border-color: var(--forest);
    background: var(--forest);
    color: #ffffff;
  }

  .llmf-page[data-theme="dark"] .llmf-button-primary {
    color: #111412;
  }

  .llmf-arrow {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  }

  .llmf-proof {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    margin-top: 68px;
    border-top: 1px solid var(--line);
  }

  .llmf-proof-item {
    padding: 18px 22px 0 0;
  }

  .llmf-proof-item + .llmf-proof-item {
    padding-left: 22px;
    border-left: 1px solid var(--line);
  }

  .llmf-proof-label {
    color: var(--muted);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 10px;
    text-transform: uppercase;
  }

  .llmf-proof-value {
    margin-top: 7px;
    font-family: Georgia, "Times New Roman", serif;
    font-size: 18px;
    font-weight: 700;
  }

  .llmf-model-wall {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    margin-top: 34px;
    border-top: 1px solid var(--line);
    border-bottom: 1px solid var(--line);
    background: var(--surface);
  }

  .llmf-model-brand {
    display: flex;
    min-height: 78px;
    align-items: center;
    justify-content: center;
    gap: 10px;
    color: var(--ink);
    font-family: Georgia, "Times New Roman", serif;
    font-size: 17px;
    font-weight: 700;
  }

  .llmf-model-brand + .llmf-model-brand {
    border-left: 1px solid var(--line);
  }

  .llmf-model-brand svg {
    width: 25px;
    height: 25px;
    color: var(--forest);
    fill: none;
    stroke: currentColor;
    stroke-linecap: round;
    stroke-linejoin: round;
    stroke-width: 1.7;
  }

  .llmf-model-brand img {
    display: block;
    width: 25px;
    height: 25px;
  }

  .llmf-page[data-theme="dark"] .llmf-model-brand img {
    filter: invert(1);
  }

  .llmf-model-brand-gemini svg {
    fill: currentColor;
    stroke: none;
  }

  .llmf-section {
    padding: 100px 0;
    border-bottom: 1px solid var(--line);
    background: color-mix(in srgb, var(--paper) 94%, transparent);
  }

  .llmf-section-head {
    display: grid;
    grid-template-columns: 160px minmax(0, 1fr);
    gap: 48px;
    margin-bottom: 58px;
  }

  .llmf-index {
    color: var(--burgundy);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 11px;
    font-weight: 700;
  }

  .llmf-section-title {
    max-width: 730px;
    margin: 0;
    font-family: Georgia, "Noto Serif SC", "Songti SC", serif;
    font-size: 38px;
    font-weight: 500;
    line-height: 1.3;
  }

  .llmf-section-copy {
    max-width: 680px;
    margin: 18px 0 0;
    color: var(--muted);
    font-size: 15px;
    line-height: 1.9;
  }

  .llmf-capabilities {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    border-top: 1px solid var(--line-strong);
  }

  .llmf-capability {
    min-width: 0;
    padding: 28px 28px 10px 0;
  }

  .llmf-capability + .llmf-capability {
    padding-left: 28px;
    border-left: 1px solid var(--line);
  }

  .llmf-capability-number {
    color: var(--forest);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 11px;
    font-weight: 700;
  }

  .llmf-capability-icon {
    display: grid;
    width: 38px;
    height: 38px;
    place-items: center;
    border: 1px solid var(--line);
    border-radius: 6px;
    background: var(--surface);
    color: var(--forest);
  }

  .llmf-capability-icon svg {
    width: 19px;
    height: 19px;
    fill: none;
    stroke: currentColor;
    stroke-linecap: round;
    stroke-linejoin: round;
    stroke-width: 1.8;
  }

  .llmf-capability h3 {
    margin: 22px 0 12px;
    font-family: Georgia, "Noto Serif SC", "Songti SC", serif;
    font-size: 21px;
    font-weight: 600;
  }

  .llmf-capability p {
    margin: 0;
    color: var(--muted);
    font-size: 13px;
    line-height: 1.8;
  }

  .llmf-architecture {
    display: grid;
    grid-template-columns: minmax(0, 0.9fr) minmax(420px, 1.1fr);
    gap: 72px;
    align-items: center;
  }

  .llmf-flow {
    display: grid;
    grid-template-columns: 1fr 38px 1.15fr 38px 1fr;
    align-items: stretch;
  }

  .llmf-flow-node {
    display: flex;
    min-height: 152px;
    flex-direction: column;
    justify-content: space-between;
    padding: 18px;
    border: 1px solid var(--line-strong);
    border-radius: 6px;
    background: var(--surface);
    box-shadow: 0 10px 26px var(--shadow);
  }

  .llmf-flow-node-main {
    border-color: var(--forest);
  }

  .llmf-flow-node small {
    color: var(--muted);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 9px;
    text-transform: uppercase;
  }

  .llmf-flow-node strong {
    font-family: Georgia, "Times New Roman", serif;
    font-size: 18px;
  }

  .llmf-flow-node span {
    color: var(--muted);
    font-size: 11px;
    line-height: 1.5;
  }

  .llmf-flow-arrow {
    display: grid;
    place-items: center;
    color: var(--forest);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  }

  .llmf-code {
    overflow: hidden;
    border: 1px solid var(--line-strong);
    border-radius: 6px;
    background: var(--code);
    box-shadow: 0 16px 40px var(--shadow);
  }

  .llmf-code-head {
    display: flex;
    min-height: 42px;
    align-items: center;
    justify-content: space-between;
    padding: 0 16px;
    border-bottom: 1px solid var(--line);
    color: var(--muted);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 10px;
  }

  .llmf-code pre {
    margin: 0;
    overflow-x: auto;
    padding: 22px;
    color: var(--ink);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 12px;
    line-height: 1.8;
  }

  .llmf-code .llmf-comment { color: var(--muted); }
  .llmf-code .llmf-key { color: var(--burgundy); }
  .llmf-code .llmf-value { color: var(--forest); }

  .llmf-principles {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 72px;
    align-items: start;
  }

  .llmf-quote {
    margin: 0;
    padding-left: 28px;
    border-left: 3px solid var(--burgundy);
    font-family: Georgia, "Noto Serif SC", "Songti SC", serif;
    font-size: 30px;
    line-height: 1.5;
  }

  .llmf-principle-list {
    border-top: 1px solid var(--line-strong);
  }

  .llmf-principle {
    display: grid;
    grid-template-columns: 94px 1fr;
    gap: 20px;
    padding: 22px 0;
    border-bottom: 1px solid var(--line);
  }

  .llmf-principle strong {
    color: var(--forest);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 11px;
    text-transform: uppercase;
  }

  .llmf-principle span {
    color: var(--muted);
    font-size: 13px;
    line-height: 1.7;
  }

  .llmf-section-title-compact {
    max-width: none;
    font-size: 35px;
    white-space: nowrap;
  }

  .llmf-faq {
    display: grid;
    grid-template-columns: 0.8fr 1.2fr;
    gap: 72px;
  }

  .llmf-faq-list {
    border-top: 1px solid var(--line-strong);
  }

  .llmf-faq > div:first-child .llmf-section-title {
    margin-top: 18px;
  }

  .llmf-faq details {
    border-bottom: 1px solid var(--line);
  }

  .llmf-faq summary {
    display: flex;
    min-height: 62px;
    align-items: center;
    justify-content: space-between;
    gap: 18px;
    cursor: pointer;
    font-family: Georgia, "Noto Serif SC", "Songti SC", serif;
    font-size: 17px;
    list-style: none;
  }

  .llmf-faq summary::-webkit-details-marker { display: none; }
  .llmf-faq summary::after { color: var(--forest); content: "+"; font-family: ui-monospace, monospace; }
  .llmf-faq details[open] summary::after { content: "−"; }

  .llmf-faq-answer {
    max-width: 680px;
    margin: -4px 0 20px;
    color: var(--muted);
    font-size: 13px;
    line-height: 1.8;
  }

  .llmf-community-cta {
    display: grid;
    grid-template-columns: 1fr auto;
    gap: 32px;
    align-items: center;
    padding: 34px 0;
    border-bottom: 1px solid var(--line);
  }

  .llmf-community-cta h2 {
    margin: 0;
    font-family: Georgia, "Noto Serif SC", "Songti SC", serif;
    font-size: 25px;
    font-weight: 600;
  }

  .llmf-community-cta p {
    margin: 8px 0 0;
    color: var(--muted);
    font-size: 13px;
  }

  .llmf-community-actions {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 10px;
  }

  .llmf-copy-button {
    cursor: pointer;
    font: inherit;
  }

  .llmf-copy-button svg {
    width: 16px;
    height: 16px;
    fill: none;
    stroke: currentColor;
    stroke-linecap: round;
    stroke-linejoin: round;
    stroke-width: 1.8;
  }

  .llmf-copy-toast {
    position: fixed;
    z-index: 100;
    right: 20px;
    bottom: 20px;
    padding: 10px 14px;
    border: 1px solid var(--forest);
    border-radius: 6px;
    background: var(--surface);
    box-shadow: 0 12px 30px var(--shadow);
    color: var(--forest);
    font-size: 12px;
    font-weight: 700;
    opacity: 0;
    pointer-events: none;
    transform: translateY(8px);
    transition: opacity 160ms ease, transform 160ms ease;
  }

.llmf-copy-toast.is-visible {
  opacity: 1;
  transform: translateY(0);
}

.llmf-model-plaza-page {
  min-height: calc(100vh - 176px);
  padding: 42px 0 84px;
}

.llmf-model-plaza-intro {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(360px, 430px);
  align-items: center;
  gap: clamp(44px, 7vw, 92px);
  margin-bottom: 38px;
}

.llmf-model-plaza-copy {
  max-width: 660px;
}

.llmf-model-plaza-intro h1 {
  margin: 12px 0 14px;
  color: var(--ink);
  font-family: Georgia, "Times New Roman", "Noto Serif SC", serif;
  font-size: 46px;
  font-weight: 500;
  letter-spacing: 0;
}

.llmf-model-plaza-intro p {
  margin: 0;
  color: var(--muted);
  font-size: 15px;
  line-height: 1.8;
  white-space: nowrap;
}

.llmf-model-plaza-visual {
  position: relative;
  width: 100%;
  height: 258px;
  margin: 0;
  overflow: hidden;
  isolation: isolate;
  -webkit-mask-image: linear-gradient(to right, #000 0%, #000 68%, transparent 100%);
  mask-image: linear-gradient(to right, #000 0%, #000 68%, transparent 100%);
}

.llmf-visual-grid {
  position: absolute;
  inset: 8px 0 12px 0;
  z-index: -2;
  background-image:
    linear-gradient(to right, color-mix(in srgb, var(--line) 55%, transparent) 1px, transparent 1px),
    linear-gradient(to bottom, color-mix(in srgb, var(--line) 55%, transparent) 1px, transparent 1px);
  background-size: 28px 28px;
  opacity: .7;
}

.llmf-visual-flow {
  position: absolute;
  inset: 0;
  z-index: -1;
}

.llmf-visual-flow::before,
.llmf-visual-flow::after,
.llmf-visual-flow span {
  content: "";
  position: absolute;
  height: 1px;
  transform-origin: left center;
  background: linear-gradient(90deg, var(--line), var(--forest));
}

.llmf-visual-flow::before { top: 48px; left: 127px; width: 126px; transform: rotate(25deg); }
.llmf-visual-flow::after { top: 200px; left: 127px; width: 126px; transform: rotate(-25deg); }
.llmf-visual-flow span:nth-child(1) { top: 126px; left: 127px; width: 118px; }
.llmf-visual-flow span:nth-child(2) { top: 126px; left: 300px; width: 116px; }
.llmf-visual-flow span:nth-child(3) { top: 126px; left: 232px; width: 7px; height: 7px; border-radius: 50%; background: var(--forest); box-shadow: 0 0 0 5px color-mix(in srgb, var(--forest) 14%, transparent); }

.llmf-model-node,
.llmf-gateway-node {
  position: absolute;
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--ink);
}

.llmf-model-node { left: 0; }
.llmf-model-node-openai { top: 22px; }
.llmf-model-node-claude { top: 101px; }
.llmf-model-node-gemini { top: 180px; }

.llmf-model-logo {
  display: grid;
  width: 42px;
  height: 42px;
  flex: 0 0 42px;
  place-items: center;
  border: 1px solid var(--line);
  border-radius: 50%;
  background: color-mix(in srgb, var(--surface) 92%, transparent);
  box-shadow: 0 8px 24px color-mix(in srgb, var(--shadow) 55%, transparent);
}

.llmf-model-logo svg { width: 21px; height: 21px; fill: currentColor; }
.llmf-model-logo-openai { color: var(--ink); }
.llmf-model-logo-claude { color: #c66b3d; }
.llmf-model-logo-gemini { color: #4285f4; }

.llmf-model-node strong,
.llmf-gateway-node strong {
  display: block;
  font-size: 13px;
  font-weight: 750;
}

.llmf-model-node small,
.llmf-gateway-node small {
  display: block;
  margin-top: 2px;
  color: var(--muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 9px;
}

.llmf-gateway-node {
  top: 91px;
  left: 242px;
  width: 148px;
  min-height: 72px;
  padding: 12px;
  border: 1px solid color-mix(in srgb, var(--forest) 42%, var(--line));
  border-radius: 6px;
  background: color-mix(in srgb, var(--surface) 94%, transparent);
  box-shadow: 0 16px 38px color-mix(in srgb, var(--shadow) 80%, transparent);
}

.llmf-gateway-node img { width: 36px; height: 36px; object-fit: contain; }
.llmf-gateway-node small { color: var(--forest); }

.llmf-gateway-pulse {
  position: absolute;
  top: 8px;
  right: 8px;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #39a96b;
  box-shadow: 0 0 0 4px color-mix(in srgb, #39a96b 15%, transparent);
}

.llmf-visual-endpoint {
  position: absolute;
  top: 178px;
  left: 270px;
  display: flex;
  align-items: center;
  gap: 7px;
  white-space: nowrap;
  color: var(--muted);
  font-size: 9px;
}

.llmf-visual-endpoint span {
  color: var(--forest);
  font-size: 8px;
  font-weight: 800;
}

.llmf-visual-endpoint code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

.llmf-footer {
    padding: 42px 0;
    background: var(--surface);
  }

  .llmf-footer-inner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 28px;
    padding-top: 32px;
  }

  .llmf-footer-note {
    color: var(--muted);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 10px;
  }

  .llmf-footer-links {
    display: flex;
    flex-wrap: wrap;
    gap: 20px;
    color: var(--muted);
    font-size: 12px;
  }

  @media (max-width: 900px) {
    .llmf-nav-link-label { display: none; }
    .llmf-nav-links > a,
    .llmf-nav-community { width: 36px; justify-content: center; padding: 0; }
    .llmf-hero { min-height: 590px; }
    .llmf-hero-grid { grid-template-columns: minmax(0, 1fr) 320px; gap: 34px; }
    .llmf-hero h1 { font-size: 45px; }
    .llmf-section-head { grid-template-columns: 110px minmax(0, 1fr); gap: 30px; }
    .llmf-architecture { grid-template-columns: 1fr; gap: 48px; }
    .llmf-principles { grid-template-columns: 1fr; gap: 48px; }
    .llmf-section-title-compact { white-space: normal; }
    .llmf-faq { grid-template-columns: 1fr; gap: 36px; }
    .llmf-model-plaza-intro { grid-template-columns: minmax(0, 1fr) 350px; gap: 34px; }
  }

  @media (max-width: 680px) {
    .llmf-wrap { width: min(100% - 32px, 1120px); }
    .llmf-nav { min-height: 64px; gap: 12px; }
    .llmf-brand-copy { display: none; }
    .llmf-mark { width: 36px; height: 36px; flex-basis: 36px; }
    .llmf-nav-right { gap: 8px; }
    .llmf-hero { min-height: 610px; padding: 66px 0 52px; }
    .llmf-hero-grid { grid-template-columns: 1fr; gap: 46px; }
    .llmf-hero h1 { font-size: 39px; line-height: 1.22; }
    .llmf-gateway-demo { width: min(100%, 420px); margin: 0 auto; justify-self: center; }
    .llmf-lead { font-size: 15px; line-height: 1.8; }
    .llmf-actions { align-items: stretch; }
    .llmf-button { flex: 1 1 150px; padding: 0 12px; }
    .llmf-proof { grid-template-columns: 1fr; margin-top: 50px; }
    .llmf-proof-item { padding: 15px 0; }
    .llmf-proof-item + .llmf-proof-item { padding-left: 0; border-top: 1px solid var(--line); border-left: 0; }
    .llmf-model-wall { grid-template-columns: repeat(3, 1fr); }
    .llmf-section { padding: 72px 0; }
    .llmf-section-head { grid-template-columns: 1fr; gap: 18px; margin-bottom: 40px; }
    .llmf-section-title { font-size: 31px; }
    .llmf-capabilities { grid-template-columns: 1fr; }
    .llmf-capability { padding: 25px 0; }
    .llmf-capability + .llmf-capability { padding-left: 0; border-top: 1px solid var(--line); border-left: 0; }
    .llmf-flow { grid-template-columns: 1fr; gap: 10px; }
    .llmf-flow-node { min-height: 118px; }
    .llmf-flow-arrow { height: 22px; transform: rotate(90deg); }
    .llmf-code pre { padding: 18px; font-size: 11px; }
    .llmf-quote { padding-left: 20px; font-size: 25px; }
    .llmf-principle { grid-template-columns: 78px 1fr; gap: 14px; }
    .llmf-footer-inner { align-items: flex-start; flex-direction: column; }
    .llmf-community-cta { grid-template-columns: 1fr; }
    .llmf-model-plaza-page { padding: 44px 0 64px; }
    .llmf-model-plaza-intro { grid-template-columns: 1fr; gap: 24px; margin-bottom: 28px; }
    .llmf-model-plaza-intro h1 { font-size: 36px; }
    .llmf-model-plaza-intro p { white-space: normal; }
    .llmf-model-plaza-visual { width: min(100%, 430px); height: 232px; }
    .llmf-model-node-openai { top: 10px; }
    .llmf-model-node-claude { top: 88px; }
    .llmf-model-node-gemini { top: 166px; }
    .llmf-gateway-node { top: 78px; }
    .llmf-visual-flow::before { top: 36px; }
    .llmf-visual-flow::after { top: 187px; }
    .llmf-visual-flow span:nth-child(1),
    .llmf-visual-flow span:nth-child(2),
    .llmf-visual-flow span:nth-child(3) { top: 113px; }
    .llmf-visual-endpoint { top: 165px; }
    .llmf-community-actions { justify-content: flex-start; }
    .llmf-nav { flex-wrap: wrap; padding: 10px 0; }
    .llmf-nav-right { width: 100%; justify-content: space-between; }
    .llmf-auth-actions { margin-left: auto; }
  }
</style>
