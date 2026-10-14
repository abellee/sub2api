<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { RouterLink } from "vue-router";
import { useI18n } from "vue-i18n";
import { useAuthStore } from "@/stores/auth";
import { setLocale } from "@/i18n";
import { useTheme, type ThemeChoice } from "@/composables/useTheme";
import { tokenFreeEnglish } from "./tokenfreeCopy";

const authStore = useAuthStore();
const { locale } = useI18n();
const isEnglish = computed(() => locale.value === "en");
const copy = (zh: string) =>
  isEnglish.value ? (tokenFreeEnglish[zh] ?? zh) : zh;
const dashboardPath = computed(() =>
  authStore.isAdmin ? "/admin/dashboard" : "/dashboard",
);
const primaryPath = computed(() =>
  authStore.isAuthenticated ? dashboardPath.value : "/register",
);
const primaryLabel = computed(() =>
  copy(authStore.isAuthenticated ? "进入控制台" : "开始使用"),
);
const root = ref<HTMLElement | null>(null);
const hasScrolled = ref(false);
function syncScroll() {
  hasScrolled.value = window.scrollY > 0;
}
const copyResult = ref<"success" | "error" | null>(null);
const copyStatus = computed(() =>
  copyResult.value === "success"
    ? copy("QQ号已复制。")
    : copyResult.value === "error"
      ? copy("请手动复制：751077517")
      : "",
);

const { theme, resolvedTheme, setTheme } = useTheme();
function closeMenus(restoreFocus = false) {
  root.value
    ?.querySelectorAll<HTMLDetailsElement>(".tf-nav-menu[open]")
    .forEach((menu) => {
      menu.open = false;
      if (restoreFocus) menu.querySelector("summary")?.focus();
    });
}
function onMenuToggle(event: Event) {
  const current = event.target as HTMLDetailsElement;
  if (!current.open) return;
  root.value
    ?.querySelectorAll<HTMLDetailsElement>(".tf-nav-menu")
    .forEach((menu) => {
      if (menu !== current) menu.open = false;
    });
}
function onDocumentClick(event: MouseEvent) {
  if (
    !(event.target instanceof Element) ||
    !event.target.closest(".tf-nav-menu")
  )
    closeMenus();
}
function onKeyDown(event: KeyboardEvent) {
  if (event.key === "Escape") closeMenus(true);
}
async function changeLanguage(language: "zh" | "en") {
  await setLocale(language);
  copyResult.value = null;
  closeMenus(true);
}
function changeTheme(value: ThemeChoice) {
  setTheme(value);
  closeMenus(true);
}
async function copySupport() {
  try {
    await navigator.clipboard.writeText("751077517");
    copyResult.value = "success";
  } catch {
    copyResult.value = "error";
  }
}
onMounted(() => {
  syncScroll();
  window.addEventListener("scroll", syncScroll, { passive: true });
  document.addEventListener("click", onDocumentClick);
  document.addEventListener("keydown", onKeyDown);
});
onBeforeUnmount(() => {
  window.removeEventListener("scroll", syncScroll);
  document.removeEventListener("click", onDocumentClick);
  document.removeEventListener("keydown", onKeyDown);
});
</script>

<template>
  <div
    class="tf-page tokenfree-home"
    ref="root"
    :lang="isEnglish ? 'en' : 'zh-CN'"
    :data-theme="resolvedTheme"
  >
    <div class="tf-nav-bar" :class="{ 'tf-nav-scrolled': hasScrolled }">
      <nav class="tf-nav">
        <a class="tf-brand tf-brand-lockup" href="#top" aria-label="TokenFree"
          ><img src="/tokenfree/logo.svg" alt="" /><span class="tf-brand-word"
            >oken<span class="tf-brand-free">Free</span></span
          ></a
        >
        <div class="tf-nav-right">
          <div class="tf-nav-destinations">
            <RouterLink to="/models"
              ><svg
                fill="none"
                stroke="currentColor"
                stroke-width="1.6"
                stroke-linecap="round"
                stroke-linejoin="round"
                aria-hidden="true"
                viewBox="0 0 24 24"
              >
                <rect x="3" y="3" width="6" height="6" rx="1"></rect>
                <rect x="15" y="3" width="6" height="6" rx="1"></rect>
                <rect x="3" y="15" width="6" height="6" rx="1"></rect>
                <rect x="15" y="15" width="6" height="6" rx="1"></rect></svg
              ><span>{{ copy("模型广场") }}</span></RouterLink
            >
            <a
              href="https://qm.qq.com/cgi-bin/qm/qr?k=4m4LsIHtnUmdqd-G3vWiPNLVq9g_fx-a&amp;jump_from=webapi&amp;authKey=PAe3IwrmVErvv6/3dyjOj7t2gHCXz2GfmlG9nS2PuUWcaAJ3iQ8lqit6nM3q9EgR"
              target="_blank"
              rel="noopener noreferrer"
              :title="copy('加入 QQ 群')"
              ><svg
                fill="none"
                stroke="currentColor"
                stroke-width="1.6"
                stroke-linecap="round"
                stroke-linejoin="round"
                aria-hidden="true"
                viewBox="0 0 24 24"
              >
                <path
                  d="M21 11.5a8.4 8.4 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.4 8.4 0 0 1-3.8-.9L3 21l1.9-5.7a8.4 8.4 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.4 8.4 0 0 1 3.8-.9H13a8.5 8.5 0 0 1 8 8v.5Z"
                ></path></svg
              ><span>{{ copy("QQ群") }}</span></a
            >
            <a
              href="https://t.me/+nqFUXD_y66liYTll"
              target="_blank"
              rel="noopener noreferrer"
              ><svg
                fill="none"
                stroke="currentColor"
                stroke-width="1.6"
                stroke-linecap="round"
                stroke-linejoin="round"
                aria-hidden="true"
                viewBox="0 0 24 24"
              >
                <path d="m22 2-7 20-4-9-9-4 20-7ZM22 2 11 13"></path></svg
              ><span>Telegram</span></a
            >
          </div>
          <div class="tf-nav-tools">
            <details class="tf-nav-menu" @toggle="onMenuToggle">
              <summary
                :aria-label="copy('语言 / Language')"
                :title="copy('语言 / Language')"
              >
                <span id="nav-language-label">{{
                  isEnglish ? "EN" : "中"
                }}</span>
              </summary>
              <div class="tf-nav-options">
                <button
                  type="button"
                  :aria-pressed="locale === 'zh'"
                  @click="changeLanguage('zh')"
                >
                  简体中文</button
                ><button
                  type="button"
                  :aria-pressed="locale === 'en'"
                  @click="changeLanguage('en')"
                >
                  English
                </button>
              </div>
            </details>

            <details class="tf-nav-menu" @toggle="onMenuToggle">
              <summary
                :aria-label="copy('主题 / Theme')"
                :title="copy('主题 / Theme')"
              >
                <svg
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.6"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  aria-hidden="true"
                  viewBox="0 0 24 24"
                >
                  <rect x="3" y="3" width="18" height="13" rx="2"></rect>
                  <path d="M8 21h8M12 16v5"></path>
                </svg>
              </summary>
              <div class="tf-nav-options">
                <button
                  type="button"
                  :aria-pressed="theme === 'dark'"
                  @click="changeTheme('dark')"
                >
                  {{ copy("深色") }}</button
                ><button
                  type="button"
                  :aria-pressed="theme === 'light'"
                  @click="changeTheme('light')"
                >
                  {{ copy("浅色") }}</button
                ><button
                  type="button"
                  :aria-pressed="theme === 'system'"
                  @click="changeTheme('system')"
                >
                  {{ copy("跟随系统") }}
                </button>
              </div>
            </details>
          </div>
          <div class="tf-nav-auth">
            <RouterLink
              class="tf-nav-register"
              v-if="!authStore.isAuthenticated"
              to="/register"
              >{{ copy("注册") }}</RouterLink
            ><RouterLink
              class="tf-nav-login"
              v-if="!authStore.isAuthenticated"
              to="/login"
              >{{ copy("登录") }}</RouterLink
            ><RouterLink
              class="tf-nav-register"
              data-auth-console
              v-if="authStore.isAuthenticated"
              :to="dashboardPath"
              ><span data-auth-label>{{ copy("进入控制台") }}</span></RouterLink
            >
          </div>
        </div>
      </nav>
    </div>
    <div class="tf-rebrand-wrap">
      <aside class="tf-rebrand-notice" role="note" :aria-label="copy('更名公告')">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" aria-hidden="true">
          <circle cx="12" cy="12" r="9" />
          <path d="M12 11v6M12 7v1" stroke-linecap="round" />
        </svg>
        <p><strong>{{ copy('更名公告') }}</strong><span>{{ copy('LLM Free 现已更名为 TokenFree。') }}</span></p>
      </aside>
    </div>
    <header class="tf-hero">
      <div class="tf-hero-inner" id="top">
        <div>
          <span class="tf-eyebrow"
            ><i></i
            ><span
              ><span>{{ copy("一个") }}</span
              ><span class="tf-eyebrow-api">API</span
              ><span>{{ copy("，无限可能。") }}</span></span
            ></span
          >
          <h1>
            {{ copy("让想法，") }}<br />{{ copy("即刻连接") }}{{ isEnglish ? " " : "" }}<em>{{ copy("AI。") }}</em>
          </h1>
          <p class="tf-lead">
            {{ copy("从灵感到第一行代码，一个 API 就够了。") }}<br />{{
              copy("连接多种模型，把精力留给你的下一个好点子。")
            }}
          </p>
          <div class="tf-actions">
            <RouterLink
              class="tf-btn tf-btn-primary"
              data-auth-primary
              :to="primaryPath"
              ><span data-auth-label>{{ primaryLabel }}</span
              ><span aria-hidden="true">↗</span></RouterLink
            >
          </div>
          <div class="tf-hero-note">
            <div><strong>01 / CONNECT</strong>{{ copy("统一接入") }}</div>
            <div><strong>02 / BUILD</strong>{{ copy("自由构建") }}</div>
            <div><strong>03 / SHIP</strong>{{ copy("让想法上线") }}</div>
          </div>
        </div>
        <div class="tf-hero-visual" :aria-label="copy('模型路由示意图')">
          <div class="tf-visual-card">
            <div class="tf-vc-top">
              <span>{{ copy("模型网络 / 预览") }}</span
              ><span class="tf-status"><b></b>{{ copy("演示模式") }}</span>
            </div>
            <div class="tf-route">
              <div class="tf-orbit"></div>
              <span class="tf-route-tag">{{ copy("✧ 一次连接") }}</span>
              <div class="tf-route-line"></div>
              <div class="tf-node tf-n1">
                <span class="tf-node-glyph">&gt;_</span
                ><span>{{ copy("你的应用") }}</span
                ><small>OpenAI SDK</small>
              </div>
              <div class="tf-node tf-n2">
                <img src="/tokenfree/logo.svg" alt="" /><span>TokenFree</span
                ><small>{{ copy("智能路由") }}</small>
              </div>
              <div class="tf-node tf-n3">
                <span class="tf-node-glyph">✳</span
                ><span>{{ copy("模型集群") }}</span
                ><small>Claude · GPT<br />Gemini</small>
              </div>
              <div class="tf-route-caption">
                {{ copy("你的想法 → 无限可能") }}
              </div>
            </div>
            <div class="tf-vc-top">
              <span>{{ copy("一个 API · 多种模型") }}</span
              ><span style="color: #71f6df">{{ copy("● 已连接") }}</span>
            </div>
          </div>
        </div>
      </div>
    </header>
    <div class="tf-metrics">
      <div class="tf-metric-box">
        <div class="tf-metric">
          <strong>One API</strong><span>{{ copy("一个入口，连接模型") }}</span>
        </div>
        <div class="tf-metric">
          <strong>Multi-model</strong
          ><span>{{ copy("为不同灵感选择模型") }}</span>
        </div>
        <div class="tf-metric">
          <strong>SDK ready</strong
          ><span>{{ copy("沿用熟悉的开发方式") }}</span>
        </div>
        <div class="tf-metric">
          <strong>Your control</strong
          ><span>{{ copy("用量与成本清晰可见") }}</span>
        </div>
      </div>
    </div>

    <section class="tf-section" id="features">
      <div class="tf-section-head">
        <div>
          <div class="tf-kicker">{{ copy("为创造者而生") }}</div>
          <h2>{{ copy("少一点配置，") }}<br />{{ copy("多一点创造。") }}</h2>
        </div>
        <p class="tf-section-intro">
          {{
            copy(
              "从第一次调用到生产环境扩展，TokenFree 把模型接入、用量和 API Key 集中在一个清晰的控制面板里。",
            )
          }}
        </p>
      </div>
      <div class="tf-features">
        <article class="tf-feature">
          <div class="tf-feature-icon">↗</div>
          <h3>{{ copy("统一 API 接入") }}</h3>
          <p>
            {{ copy("兼容主流协议，替换一个 Base URL 即可接入现有应用。") }}
          </p>
        </article>
        <article class="tf-feature">
          <div class="tf-feature-icon">⌁</div>
          <h3>{{ copy("智能模型路由") }}</h3>
          <p>
            {{ copy("根据可用性、延迟与成本，为每一次请求选择更合适的模型。") }}
          </p>
        </article>
        <article class="tf-feature">
          <div class="tf-feature-icon">▥</div>
          <h3>{{ copy("用量与成本控制") }}</h3>
          <p>
            {{ copy("实时查看消耗、设置预算和限额，让调用花费清晰可控。") }}
          </p>
        </article>
        <article class="tf-feature">
          <div class="tf-feature-icon">♧</div>
          <h3>{{ copy("API Key 管理") }}</h3>
          <p>{{ copy("创建和管理 API Key，为不同应用配置调用凭证。") }}</p>
        </article>
      </div>
    </section>

    <section class="tf-code-section" id="developers">
      <div class="tf-code-wrap">
        <div>
          <div class="tf-kicker">{{ copy("开发者优先") }}</div>
          <h2>
            {{ copy("从这一行代码，") }}<br />{{ copy("开始你的下一步。") }}
          </h2>
          <p>
            {{
              copy(
                "熟悉的 SDK、清晰的文档、可观测的请求链路。把接入模型的时间，留给构建产品。",
              )
            }}
          </p>
          <div class="tf-steps">
            <div class="tf-step">
              <span class="tf-step-no">1</span>
              <div>
                <strong>{{ copy("替换 Base URL") }}</strong
                ><small>{{ copy("保持现有代码结构不变") }}</small>
              </div>
            </div>
            <div class="tf-step">
              <span class="tf-step-no">2</span>
              <div>
                <strong>{{ copy("配置 API Key") }}</strong
                ><small>{{ copy("为应用配置调用凭证") }}</small>
              </div>
            </div>
            <div class="tf-step">
              <span class="tf-step-no">3</span>
              <div>
                <strong>{{ copy("开始调用") }}</strong
                ><small>{{ copy("在控制台查看每一次请求") }}</small>
              </div>
            </div>
          </div>
        </div>
        <div class="tf-code-card">
          <div class="tf-code-bar">
            <span class="tf-dots"><i></i><i></i><i></i></span
            ><span>quick-start.sh</span><span>cURL</span>
          </div>
          <pre><span class="tf-comment">{{ copy("# 将现有应用连接到 TokenFree") }}</span>
<span class="tf-key">export</span> OPENAI_BASE_URL=<span class="tf-str">"https://api.tokenfree.dev"</span>
<span class="tf-key">export</span> OPENAI_API_KEY=<span class="tf-str">"sk-live-••••••••"</span>

<span class="tf-comment">{{ copy("# 发起第一次请求") }}</span>
curl <span class="tf-str">"$OPENAI_BASE_URL/chat/completions"</span> \
  -H <span class="tf-str">"Authorization: Bearer $OPENAI_API_KEY"</span> \
  -H <span class="tf-str">"Content-Type: application/json"</span> \
  -d <span class="tf-str">'{"model":"auto","messages":[...]}'</span></pre>
        </div>
      </div>
    </section>

    <section class="tf-section tf-trust-section" id="principles">
      <div class="tf-kicker">{{ copy("清晰 · 可控 · 持续") }}</div>
      <blockquote>
        <span class="tf-trust-line">{{
          copy("技术服务的可信度，不来自夸张承诺，")
        }}</span
        ><span class="tf-trust-line">{{
          copy("而来自清晰、克制与持续可验证。")
        }}</span>
      </blockquote>
      <div class="tf-principle-grid">
        <article>
          <span class="tf-principle-number">01</span>
          <h3>{{ copy("清晰") }}</h3>
          <p>
            {{ copy("清晰展示接口、用量与运行状态，减少不可解释的黑盒。") }}
          </p>
        </article>
        <article>
          <span class="tf-principle-number">02</span>
          <h3>{{ copy("可控") }}</h3>
          <p>{{ copy("密钥、额度与访问路径由你掌握，适配真实开发流程。") }}</p>
        </article>
        <article>
          <span class="tf-principle-number">03</span>
          <h3>{{ copy("持续") }}</h3>
          <p>
            {{ copy("以持续可用为目标，把稳定运行视为基础能力而非附加功能。") }}
          </p>
        </article>
      </div>
    </section>

    <section class="tf-section tf-faq-section" id="faq">
      <div class="tf-faq-intro">
        <div class="tf-kicker">{{ copy("常见问题") }}</div>
        <h2>{{ copy("关于 TokenFree") }}</h2>
        <p>{{ copy("关于 AI API 中转、模型支持与商务联系的简要说明。") }}</p>
      </div>
      <div class="tf-faq-list">
        <details open>
          <summary>{{ copy("TokenFree 是什么？") }}</summary>
          <p>
            {{
              copy(
                "TokenFree 是一个兼容 OpenAI API 格式的 AI API 中转站，为开发者和团队提供统一的大模型访问入口、API 密钥管理、用量统计与成本查看。",
              )
            }}
          </p>
        </details>

        <details>
          <summary>{{ copy("支持哪些大模型？") }}</summary>
          <p>
            {{
              copy(
                "平台可接入 Claude、GPT、Gemini 等主流模型，实际可用模型以模型广场展示为准。",
              )
            }}
          </p>
        </details>

        <details>
          <summary>{{ copy("如何联系商务支持？") }}</summary>
          <p>
            {{ copy("商务合作、企业接入与定制需求请联系 QQ：751077517。") }}
          </p>
        </details>
      </div>
    </section>

    <section class="tf-bottom-cta tf-community-section" id="start">
      <div class="tf-kicker">{{ copy("一起连接更多可能") }}</div>
      <h2>{{ copy("加入社群") }}</h2>
      <p>
        {{
          copy("加入 QQ群或 Telegram 群交流；商务支持请联系 QQ：751077517。")
        }}
      </p>
      <div class="tf-community-actions">
        <a
          class="tf-btn tf-btn-primary"
          href="https://qm.qq.com/cgi-bin/qm/qr?k=4m4LsIHtnUmdqd-G3vWiPNLVq9g_fx-a&amp;jump_from=webapi&amp;authKey=PAe3IwrmVErvv6/3dyjOj7t2gHCXz2GfmlG9nS2PuUWcaAJ3iQ8lqit6nM3q9EgR"
          target="_blank"
          rel="noopener noreferrer"
          >{{ copy("加入QQ群") }}</a
        ><a
          class="tf-btn tf-btn-ghost"
          href="https://t.me/+nqFUXD_y66liYTll"
          target="_blank"
          rel="noopener noreferrer"
          >Telegram</a
        ><button
          class="tf-btn tf-btn-ghost"
          type="button"
          id="copy-support"
          @click="copySupport"
        >
          <span>{{ copy("复制商务 QQ") }}</span
          ><span>751077517</span>
        </button>
      </div>
      <p
        class="tf-copy-status"
        id="copy-status"
        role="status"
        aria-live="polite"
      >
        {{ copyStatus }}
      </p>
    </section>

    <footer class="tf-footer">
      <a
        class="tf-brand tf-brand-lockup tf-footer-brand"
        href="#top"
        aria-label="TokenFree"
        ><img src="/tokenfree/logo.svg" alt="" /><span class="tf-brand-word"
          >oken<span class="tf-brand-free">Free</span></span
        ></a
      ><span>{{ copy("© 2026 TokenFree. 价廉物美，Token 自由。") }}</span>
    </footer>
  </div>
</template>

<style scoped>
.tokenfree-home {
  --bg: #070912;
  --surface: #10141f;
  --ink: #f4f6ff;
  --muted: #99a3b9;
  --blue: #536eff;
  --cyan: #71f6df;
  --orange: #ff854a;
  --line: #ffffff16;
}

* {
  box-sizing: border-box;
}
.tokenfree-home {
  margin: 0;
  background: var(--bg);
  color: var(--ink);
  font-family: "JetBrains Mono", "Segoe UI", "Microsoft YaHei", sans-serif;
  -webkit-font-smoothing: antialiased;
}
a {
  color: inherit;
  text-decoration: none;
}
button,
a {
  -webkit-tap-highlight-color: transparent;
}
a:focus-visible {
  outline: 2px solid var(--cyan);
  outline-offset: 5px;
}
.tf-page {
  overflow-x: clip;
}
.tf-nav-bar {
  position: sticky;
  top: 0;
  z-index: 40;
  background: transparent;
  transition: background-color 300ms ease;
}
.tf-nav-bar::after {
  content: "";
  position: absolute;
  inset: auto 0 0;
  height: 1px;
  background: var(--line);
  opacity: 0;
  transition: opacity 300ms ease;
  pointer-events: none;
}
.tf-nav-bar.tf-nav-scrolled {
  background: color-mix(in srgb, var(--bg) 92%, transparent);
  backdrop-filter: blur(16px);
}
.tf-nav-bar.tf-nav-scrolled::after {
  opacity: 1;
}
.tf-nav {
  max-width: 1280px;
  margin: auto;
  padding: 24px 36px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  position: relative;
  z-index: 3;
}
.tf-rebrand-wrap {
  max-width: 1280px;
  margin: 0 auto;
  padding: 0 clamp(20px, 3vw, 36px);
}
.tf-rebrand-notice {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 16px;
  border: 1px solid rgb(137 155 255 / 0.3);
  border-radius: 12px;
  color: var(--ink);
  background: linear-gradient(110deg, rgb(83 110 255 / 0.1), rgb(119 85 243 / 0.04));
  font-size: 12px;
  line-height: 1.7;
}
.tf-rebrand-notice svg {
  width: 18px;
  height: 18px;
  flex: none;
  margin-top: 1px;
  color: var(--tf-console-active-ink, #b7c2ff);
}
.tf-rebrand-notice p {
  display: flex;
  flex-wrap: wrap;
  column-gap: 12px;
  margin: 0;
}
.tf-rebrand-notice strong {
  color: var(--tf-console-active-ink, #b7c2ff);
  font-weight: 600;
}
.tf-brand {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 23px;
  font-weight: 750;
  letter-spacing: -1px;
}
.tf-brand img {
  width: 43px;
  height: 43px;
}
.tf-brand small {
  font:
    12px "JetBrains Mono",
    ui-monospace,
    monospace;
  letter-spacing: 2px;
  color: #939db4;
  margin-top: 4px;
  display: block;
}
.nav-links {
  display: flex;
  align-items: center;
  gap: 30px;
  font-size: 13px;
  color: #aab1c4;
}
.nav-links a:hover {
  color: var(--cyan);
}
.nav-cta {
  background: #ffffff0c;
  border: 1px solid #ffffff28;
  border-radius: 24px;
  padding: 11px 22px;
  color: white;
}
.tf-hero {
  position: relative;
  background: radial-gradient(ellipse at 78% 48%, #3849c029, transparent 52%);
}
.tf-hero:before {
  content: "";
  position: absolute;
  inset: 70px 0 0;
  background-image: radial-gradient(#a5afff35 0.7px, transparent 0.7px);
  background-size: 26px 26px;
  mask-image: linear-gradient(90deg, transparent 15%, black);
  pointer-events: none;
}
.tf-hero-inner {
  max-width: 1280px;
  margin: auto;
  padding: 82px 36px 95px;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 40px;
  align-items: center;
  position: relative;
}
.tf-eyebrow {
  display: inline-flex;
  gap: 10px;
  align-items: center;
  border: 1px solid #71f6df30;
  border-radius: 24px;
  padding: 8px 12px;
  background: #71f6df08;
  color: var(--cyan);
  font:
    12px "JetBrains Mono",
    ui-monospace,
    monospace;
  letter-spacing: 1.7px;
}
.tf-eyebrow i {
  width: 5px;
  height: 5px;
  background: var(--cyan);
  border-radius: 50%;
  box-shadow: 0 0 10px var(--cyan);
}
h1 {
  font-size: clamp(45px, 5vw, 70px);
  line-height: 1.2;
  letter-spacing: -3px;
  margin: 26px 0 22px;
  font-weight: 750;
}
h1 em {
  font-style: normal;
  background: linear-gradient(100deg, #8aa0ff 0%, #b797ff 45%, #71f6df 100%);
  color: transparent;
  background-clip: text;
}
.tokenfree-home[lang="zh-CN"] h1 em {
  margin-left: 0.12em;
}
.tf-lead {
  font-size: 15px;
  color: #a7b1c6;
  line-height: 1.95;
  max-width: 440px;
}
.tf-actions {
  display: flex;
  gap: 14px;
  margin-top: 30px;
  flex-wrap: wrap;
}
.tf-btn {
  display: inline-flex;
  align-items: center;
  gap: 22px;
  padding: 15px 22px;
  border-radius: 9px;
  font-size: 13px;
  font-weight: 650;
  transition:
    transform 0.2s,
    box-shadow 0.2s;
}
.tf-btn:hover {
  transform: translateY(-3px);
}
.tf-btn-primary {
  color: #fff;
  background: linear-gradient(115deg, #486aff, #7755f3);
  box-shadow:
    0 0 25px #536eff45,
    inset 0 1px #ffffff55;
  border: 1px solid #899bff88;
}
.tf-btn-primary:hover {
  box-shadow: 0 0 38px #536eff80;
}
.tf-btn-ghost {
  border: 1px solid #ffffff25;
  background: #ffffff05;
}
.tf-hero-note {
  display: flex;
  gap: 22px;
  margin-top: 32px;
  font-size: 12px;
  color: #838fa7;
}
.tf-hero-note strong {
  display: block;
  font:
    12px "JetBrains Mono",
    ui-monospace,
    monospace;
  color: #d4daeb;
  margin-bottom: 7px;
}
.tf-hero-visual {
  position: relative;
  min-width: 0;
  height: 420px;
}
.tf-hero-visual:before {
  content: "";
  position: absolute;
  width: 300px;
  height: 300px;
  left: 20%;
  top: 14%;
  background: conic-gradient(from 45deg, #536eff, #b578ff, #71f6df, #536eff);
  filter: blur(80px);
  opacity: 0.22;
  border-radius: 50%;
}
.tf-visual-card {
  position: absolute;
  inset: 0;
  border: 1px solid #899bff30;
  border-radius: 20px;
  background: linear-gradient(135deg, #19213b99, #0b0f1bbf);
  box-shadow:
    0 30px 90px #0006,
    inset 0 1px #ffffff12;
  backdrop-filter: blur(18px);
  padding: 22px;
  transform: perspective(1200px) rotateY(-5deg) rotateX(2deg);
}
.tf-vc-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font:
    12px "JetBrains Mono",
    ui-monospace,
    monospace;
  letter-spacing: 1px;
  color: #909db7;
  gap: 8px;
}
.tf-status {
  color: var(--cyan);
  display: flex;
  align-items: center;
  gap: 7px;
}
.tf-status b {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--cyan);
}
.tf-route {
  height: 286px;
  position: relative;
  margin: 12px 0 20px;
}
.tf-orbit {
  position: absolute;
  width: 238px;
  height: 238px;
  border: 1px solid #8393ff23;
  border-radius: 50%;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
}
.tf-orbit:before {
  content: "";
  position: absolute;
  inset: 28px;
  border: 1px dashed #7e8bea2c;
  border-radius: 50%;
}
.tf-orbit:after {
  content: "";
  position: absolute;
  inset: -25px;
  border: 1px solid #8393ff0c;
  border-radius: 50%;
}
.tf-route-line {
  position: absolute;
  left: 12%;
  right: 12%;
  top: 49%;
  height: 1px;
  background: linear-gradient(90deg, #536eff, #71f6df, #b58cff);
}
.tf-route-line:after {
  content: "";
  position: absolute;
  width: 5px;
  height: 5px;
  top: -2px;
  border-radius: 50%;
  background: var(--cyan);
  box-shadow: 0 0 14px var(--cyan);
  animation: flow 4s linear infinite;
}
.tf-node {
  position: absolute;
  top: 99px;
  width: 84px;
  height: 84px;
  text-align: center;
  border: 1px solid #ffffff1c;
  border-radius: 14px;
  background: #151b2b;
  padding: 12px 5px;
  font-size: 12px;
}
.tf-node small {
  display: block;
  font-size: 12px;
  color: #9ba7c1;
  margin-top: 5px;
}
.tf-node img {
  display: block;
  width: 35px;
  height: 35px;
  margin: auto;
}
.tf-node-glyph {
  display: block;
  font:
    23px "JetBrains Mono",
    ui-monospace,
    monospace;
  margin-bottom: 9px;
  color: #c1cdff;
}
.tf-node.tf-n1 {
  left: 0;
}
.tf-node.tf-n2 {
  left: calc(50% - 49px);
  top: 91px;
  width: 98px;
  height: 100px;
  background: linear-gradient(140deg, #334589, #191d38);
  border-color: #a5a7ff66;
  box-shadow: 0 0 34px #536eff35;
}
.tf-node.tf-n3 {
  right: 0;
}
.tf-route-caption {
  position: absolute;
  bottom: 3px;
  left: 0;
  right: 0;
  text-align: center;
  font:
    12px "JetBrains Mono",
    ui-monospace,
    monospace;
  letter-spacing: 1.4px;
  color: #91a0bb;
}
.tf-route-tag {
  position: absolute;
  left: 50%;
  top: 24px;
  transform: translateX(-50%);
  font:
    12px "JetBrains Mono",
    ui-monospace,
    monospace;
  color: #b8a4ff;
  background: #151529;
  border: 1px solid #b897ff30;
  padding: 7px 12px;
  border-radius: 20px;
  white-space: nowrap;
}
.tf-metrics {
  max-width: 1208px;
  margin: 0 auto;
  padding: 0 0 24px;
}
.tf-metric-box {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  border-block: 1px solid var(--line);
  padding: 27px 0;
}
.tf-metric {
  padding-left: 30px;
  border-right: 1px solid var(--line);
}
.tf-metric:first-child {
  padding-left: 0;
}
.tf-metric:last-child {
  border: 0;
}
.tf-metric strong {
  font:
    21px "JetBrains Mono",
    ui-monospace,
    monospace;
  color: #d7dff7;
  letter-spacing: -0.5px;
}
.tf-metric:nth-child(2) strong {
  color: #a995ff;
}
.tf-metric:nth-child(3) strong {
  color: var(--cyan);
}
.tf-metric span {
  display: block;
  font-size: 12px;
  color: #8996b0;
  margin-top: 8px;
}
.tf-section {
  max-width: 1280px;
  margin: auto;
  padding: 94px 36px;
}
.tf-section-head {
  display: flex;
  justify-content: space-between;
  align-items: end;
  gap: 40px;
  margin-bottom: 34px;
}
.tf-kicker {
  font:
    12px "JetBrains Mono",
    ui-monospace,
    monospace;
  letter-spacing: 2px;
  color: var(--cyan);
  text-transform: uppercase;
}
.tf-section h2,
.tf-code-wrap h2 {
  font-size: 38px;
  line-height: 1.3;
  letter-spacing: -1.5px;
  margin: 16px 0 0;
}
.tf-section-intro {
  font-size: 13px;
  line-height: 1.9;
  color: var(--muted);
  max-width: 360px;
  margin: 0;
}
.tf-features {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 14px;
}
.tf-feature {
  border: 1px solid var(--line);
  background: linear-gradient(135deg, #151a29, #0d111c);
  padding: 26px 23px;
  border-radius: 14px;
  transition:
    transform 0.25s,
    border-color 0.25s;
}
.tf-feature:hover {
  transform: translateY(-5px);
  border-color: #768cff88;
}
.tf-feature-icon {
  width: 39px;
  height: 39px;
  display: grid;
  place-items: center;
  color: #94a5ff;
  border: 1px solid #6d80ff30;
  background: #536eff12;
  border-radius: 11px;
  font-size: 21px;
  margin-bottom: 32px;
}
.tf-feature:nth-child(2) .tf-feature-icon {
  color: #ba9bff;
  background: #b08aff12;
}
.tf-feature:nth-child(3) .tf-feature-icon {
  color: var(--cyan);
  background: #71f6df10;
}
.tf-feature:nth-child(4) .tf-feature-icon {
  color: var(--orange);
  background: #ff854a12;
}
.tf-feature h3 {
  font-size: 15px;
  margin: 0 0 11px;
}
.tf-feature p {
  font-size: 12px;
  line-height: 1.85;
  color: var(--muted);
  margin: 0;
}
.tf-code-section {
  border-block: 1px solid var(--line);
  background: radial-gradient(ellipse at 80% 80%, #536eff12, transparent 60%);
}
.tf-code-wrap {
  max-width: 1280px;
  margin: auto;
  padding: 85px 36px;
  display: grid;
  grid-template-columns: 0.85fr 1.15fr;
  gap: 70px;
  align-items: center;
}
.tf-code-wrap > div {
  min-width: 0;
}
.tf-code-wrap p {
  font-size: 13px;
  line-height: 1.9;
  color: var(--muted);
  max-width: 360px;
}
.tf-steps {
  display: grid;
  gap: 20px;
  margin-top: 30px;
}
.tf-step {
  display: flex;
  gap: 13px;
}
.tf-step-no {
  width: 24px;
  height: 24px;
  display: grid;
  place-items: center;
  border: 1px solid #536eff66;
  background: #536eff15;
  border-radius: 6px;
  font:
    12px "JetBrains Mono",
    ui-monospace,
    monospace;
  color: #a3b3ff;
  flex: none;
}
.tf-step strong {
  font-size: 12px;
}
.tf-step small {
  display: block;
  color: #8996b0;
  font-size: 12px;
  margin-top: 6px;
}
.tf-code-card {
  min-width: 0;
  border: 1px solid #ffffff20;
  background: #0d121f;
  border-radius: 13px;
  box-shadow: 0 25px 60px #0004;
  overflow: hidden;
}
.tf-code-bar {
  padding: 15px 18px;
  display: flex;
  justify-content: space-between;
  border-bottom: 1px solid var(--line);
  font:
    12px "JetBrains Mono",
    ui-monospace,
    monospace;
  color: #a0acc6;
}
.tf-dots {
  display: flex;
  gap: 6px;
}
.tf-dots i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #ff8b70;
}
.tf-dots i:nth-child(2) {
  background: #ffca7c;
}
.tf-dots i:last-child {
  background: #71f6df;
}
pre {
  font:
    12px/2 "JetBrains Mono",
    ui-monospace,
    Consolas,
    monospace;
  overflow: auto;
  margin: 0;
  padding: 25px;
  color: #e6edff;
}
.tf-key {
  color: #bba2ff;
}
.tf-str {
  color: #83dfd3;
}
.tf-comment {
  color: #8592ad;
}
.tf-bottom-cta {
  position: relative;
  text-align: center;
  padding: 105px 24px 100px;
  background: radial-gradient(ellipse at 50% 100%, #536eff26, transparent 65%);
}
.tf-bottom-cta h2 {
  font-size: 46px;
  line-height: 1.25;
  letter-spacing: -2px;
  margin: 22px auto 18px;
  max-width: 680px;
}
.tf-bottom-cta p {
  font-size: 13px;
  color: var(--muted);
  margin-bottom: 28px;
}
.tf-footer {
  max-width: 1208px;
  border-top: 1px solid var(--line);
  margin: auto;
  padding: 24px 0;
  display: flex;
  justify-content: space-between;
  color: #8490a9;
  font-size: 12px;
}
.tf-footer b {
  color: #e2e7fa;
  font-size: 13px;
}
@keyframes flow {
  from {
    left: 0;
  }
  to {
    left: 100%;
  }
}
@media (prefers-reduced-motion: reduce) {
  *,
  *:after {
    animation: none !important;
    transition: none !important;
  }
}

@media (max-width: 1000px) {
  .tf-hero-inner {
    gap: 25px;
    padding-top: 55px;
  }
  .tf-visual-card {
    transform: none;
    padding: 17px;
  }
  .tf-hero-visual {
    height: 400px;
  }
  .tf-route {
    height: 275px;
  }
  .nav-links {
    gap: 22px;
  }
  h1 {
    font-size: 48px;
    letter-spacing: -2px;
  }
  .tf-metrics,
  .tf-footer {
    margin-inline: 36px;
  }
  .tf-features {
    grid-template-columns: repeat(2, 1fr);
  }
  .tf-code-wrap {
    gap: 35px;
  }
}

@media (max-width: 720px) {
  .tf-nav {
    padding: 18px 22px;
  }
  .nav-links a:not(.nav-cta) {
    display: none;
  }
  .tf-brand {
    font-size: 21px;
  }
  .tf-hero-inner {
    grid-template-columns: 1fr;
    padding: 45px 22px 45px;
    gap: 42px;
  }
  .tf-hero-visual {
    max-width: 500px;
    width: 100%;
    margin: auto;
  }
  .tf-lead {
    max-width: 480px;
  }
  h1 {
    font-size: 52px;
  }
  .tf-metrics,
  .tf-footer {
    margin-inline: 22px;
  }
  .tf-metric-box {
    grid-template-columns: repeat(2, 1fr);
    gap: 24px 0;
  }
  .tf-metric {
    padding-left: 20px;
  }
  .tf-metric:nth-child(odd) {
    padding-left: 0;
  }
  .tf-metric:nth-child(2) {
    border: 0;
  }
  .tf-metric strong {
    font-size: 19px;
  }
  .tf-section,
  .tf-code-wrap {
    padding: 64px 22px;
  }
  .tf-section-head {
    display: block;
  }
  .tf-section-intro {
    margin-top: 20px;
  }
  .tf-code-wrap {
    grid-template-columns: 1fr;
    gap: 35px;
  }
  .tf-section h2,
  .tf-code-wrap h2 {
    font-size: 32px;
  }
  .tf-bottom-cta {
    padding: 75px 22px;
  }
  .tf-bottom-cta h2 {
    font-size: 36px;
  }
  .tf-footer {
    gap: 14px;
    flex-wrap: wrap;
  }
}

@media (max-width: 420px) {
  h1 {
    font-size: 44px;
  }
  .tf-hero-note {
    gap: 18px;
  }
  .tf-hero-visual {
    height: 380px;
  }
  .tf-visual-card {
    padding: 15px;
  }
  .tf-vc-top {
    font-size: 12px;
    letter-spacing: 0.3px;
  }
  .tf-route {
    height: 255px;
  }
  .tf-orbit {
    width: 210px;
    height: 210px;
  }
  .tf-node {
    width: 74px;
    font-size: 12px;
  }
  .tf-node.tf-n2 {
    width: 88px;
    left: calc(50% - 44px);
  }
  .tf-features {
    grid-template-columns: 1fr;
  }
  .tf-feature {
    padding: 22px;
  }
  .tf-feature-icon {
    margin-bottom: 20px;
  }
  .tf-eyebrow {
    font-size: 12px;
  }
  .nav-cta {
    padding: 10px 16px;
  }
  .tf-brand small {
    font-size: 12px;
  }
}

/* The supplied T symbol is the first letter of the wordmark. */
.tf-brand-lockup {
  display: inline-flex;
  align-items: center;
  gap: 0;
  letter-spacing: -1.1px;
  font-weight: 750;
  line-height: 1;
  white-space: nowrap;
  color: #f4f6ff;
}

.tf-brand-lockup img {
  width: 44px;
  height: 44px;
  flex: none;
  margin-right: -7px;
  transform: translateY(1px);
}

/* Optical alignment accounts for the SVG canvas padding and font descender space. */
.tf-brand-word {
  font-size: 29px;
  transform: translateY(0.14em);
}
.tf-brand-free {
  color: #e8590c;
}

.tf-footer {
  align-items: center;
  position: relative;
  border-top: 0;
}
.tf-footer::before {
  content: "";
  position: absolute;
  top: 0;
  left: 50%;
  width: 100vw;
  height: 1px;
  transform: translateX(-50%);
  background: var(--line);
  pointer-events: none;
}

@media (max-width: 720px) {
  .tf-brand-lockup .tf-brand-word {
    font-size: 27px;
  }
  .tf-brand-lockup img {
    width: 41px;
    height: 41px;
  }
}

/* Readable small type and room for the complete model list. */
.tf-node {
  height: 100px;
}
.tf-node.tf-n2 {
  height: 112px;
}
.tf-node.tf-n3 {
  width: 116px;
  height: 112px;
}

.tf-node.tf-n3 small {
  white-space: normal;
  letter-spacing: 0;
  line-height: 1.5;
}

.tf-vc-top {
  flex-wrap: wrap;
  line-height: 1.6;
  letter-spacing: 0.3px;
}

.tf-route-caption {
  line-height: 1.6;
  letter-spacing: 0.5px;
}

.tf-brand small,
.tf-eyebrow,
.tf-kicker {
  line-height: 1.6;
}

@media (max-width: 1000px) {
  .tf-hero-inner {
    grid-template-columns: 1fr;
  }
  .tf-hero-visual {
    width: 100%;
    max-width: 580px;
    margin: auto;
    height: 440px;
  }
  .tf-route {
    height: 286px;
  }
}

@media (max-width: 520px) {
  .tf-hero-visual {
    height: 490px;
  }
  .tf-route {
    height: 350px;
    margin-top: 16px;
    margin-bottom: 16px;
  }

  .tf-node.tf-n1 {
    top: 78px;
    left: 12px;
    width: 88px;
    height: 100px;
  }

  .tf-node.tf-n2 {
    top: 70px;
    left: auto;
    right: 12px;
    width: 98px;
    height: 112px;
  }

  .tf-node.tf-n3 {
    top: 204px;
    left: calc(50% - 58px);
    right: auto;
    width: 116px;
    height: 112px;
  }

  .tf-route-tag {
    top: 16px;
  }
  .tf-route-line {
    top: 125px;
    left: 56px;
    right: 61px;
  }

  .tf-route-caption {
    bottom: 0;
    font-size: 12px;
    letter-spacing: 0;
  }

  .tf-vc-top {
    font-size: 12px;
    letter-spacing: 0;
    gap: 4px 12px;
  }

  .tf-eyebrow {
    font-size: 12px;
    letter-spacing: 0.5px;
  }
  .tf-hero-note {
    flex-wrap: wrap;
  }
}

/* Align the routing cards and their icon/title rows. */
.tf-node.tf-n1,
.tf-node.tf-n2,
.tf-node.tf-n3 {
  top: 99px;
  height: 112px;
}

.tf-node .tf-node-glyph {
  height: 35px;
  line-height: 35px;
  margin-bottom: 0;
}

.tf-route-line {
  top: 155px;
}

@media (max-width: 520px) {
  .tf-node.tf-n1,
  .tf-node.tf-n2 {
    top: 78px;
    height: 112px;
  }

  .tf-node.tf-n3 {
    top: 204px;
    height: 112px;
  }

  .tf-route-line {
    top: 134px;
  }
}

@font-face {
  font-family: "JetBrains Mono";

  src: url("/tokenfree/fonts/JetBrainsMono-Variable.ttf") format("truetype");

  font-weight: 100 800;

  font-style: normal;

  font-display: swap;
}

/* Smooth, visible 3D movement for the routing demo. */
.tf-visual-card {
  transform-origin: 50% 50%;

  animation: route-card-sway 10s ease-in-out infinite;

  will-change: transform;
}

@keyframes route-card-sway {
  0%,
  100% {
    transform: perspective(1200px) rotateX(3deg) rotateY(-8deg) rotateZ(-0.7deg);
  }

  33% {
    transform: perspective(1200px) rotateX(-4deg) rotateY(2deg) rotateZ(0.5deg);
  }

  66% {
    transform: perspective(1200px) rotateX(2deg) rotateY(8deg) rotateZ(0.7deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .tf-visual-card {
    animation: none;
    transform: none;
    will-change: auto;
  }
}

/* Reference navigation, adapted to the TokenFree visual system. */
.tf-nav-right,
.tf-nav-destinations,
.tf-nav-tools,
.tf-nav-auth {
  display: flex;
  align-items: center;
}

.tf-nav-right {
  gap: 24px;
}
.tf-nav-destinations {
  gap: 22px;
}
.tf-nav-destinations a {
  display: flex;
  align-items: center;
  gap: 7px;
  color: #adb8d0;
  font-size: 12px;
  white-space: nowrap;
}

.tf-nav-right svg {
  width: 16px;
  height: 16px;
  flex: none;
  color: #91a4ff;
}
.tf-nav-destinations a:hover {
  color: var(--cyan);
}

.tf-nav-tools,
.tf-nav-auth {
  gap: 8px;
}
.tf-nav-tools {
  border-left: 1px solid var(--line);
  padding-left: 22px;
}

.tf-nav-menu {
  position: relative;
}
.tf-nav-menu summary {
  list-style: none;
  cursor: pointer;
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  border: 1px solid #ffffff1c;
  border-radius: 8px;
  background: #ffffff05;
  font-size: 12px;
  color: #d4ddf5;
}

.tf-nav-menu summary::-webkit-details-marker {
  display: none;
}
.tf-nav-menu summary:hover,
.tf-nav-menu[open] summary {
  border-color: #8799ff88;
  background: #536eff15;
}

.tf-nav-options {
  position: absolute;
  top: 43px;
  right: 0;
  width: 144px;
  padding: 6px;
  background: #131a2b;
  border: 1px solid #ffffff25;
  border-radius: 10px;
  box-shadow: 0 12px 35px #0006;
  z-index: 20;
}

.tf-nav-options button {
  display: block;
  width: 100%;
  border: 0;
  border-radius: 6px;
  padding: 9px 10px;
  text-align: left;
  font:
    12px "JetBrains Mono",
    "Microsoft YaHei",
    sans-serif;
  background: transparent;
  color: #bec9df;
  cursor: pointer;
}

.tf-nav-options button:hover,
.tf-nav-options button[aria-pressed="true"] {
  background: #536eff22;
  color: #aebcff;
}

.tf-nav-register,
.tf-nav-login {
  font-size: 12px;
  padding: 9px 14px;
  border-radius: 8px;
  white-space: nowrap;
}

.tf-nav-register {
  background: linear-gradient(115deg, #486aff, #7755f3);
  border: 1px solid #899bff88;
  color: white;
  box-shadow: 0 0 18px #536eff22;
}

.tf-nav-login {
  border: 1px solid #ffffff25;
  background: #ffffff05;
  color: #dbe2f5;
}

.tf-nav-right :focus-visible {
  outline: 2px solid var(--cyan);
  outline-offset: 4px;
}

@media (max-width: 1000px) {
  .tf-nav-right {
    gap: 14px;
  }
  .tf-nav-destinations {
    gap: 14px;
  }
  .tf-nav-tools {
    padding-left: 14px;
  }
}

@media (max-width: 850px) {
  .tf-nav {
    flex-wrap: wrap;
    gap: 16px;
  }
  .tf-nav-right {
    flex-wrap: wrap;
    justify-content: flex-end;
  }
  .tf-nav-destinations {
    gap: 12px;
  }
  .tf-nav-tools {
    border: 0;
    padding-left: 0;
  }
}

@media (max-width: 720px) {
  .tf-nav-right {
    width: 100%;
    justify-content: space-between;
    gap: 14px 8px;
  }
  .tf-nav-destinations {
    width: 100%;
    justify-content: space-between;
    padding-bottom: 12px;
    border-bottom: 1px solid var(--line);
  }
  .tf-nav-tools {
    margin-right: auto;
  }
}

.tokenfree-home[data-theme="light"] {
  --bg: #f5f7ff;
  --surface: #fff;
  --ink: #16213b;
  --muted: #53617c;
  --line: #182b5320;
  --cyan: #087e76;
}

[data-theme="light"] .tf-brand-lockup {
  color: #182641;
}

[data-theme="light"] .tf-nav-destinations a,
[data-theme="light"] .tf-nav-login,
[data-theme="light"] .tf-nav-menu summary {
  color: #334567;
}

[data-theme="light"] .tf-nav-login,
[data-theme="light"] .tf-nav-menu summary {
  background: #fff9;
  border-color: #1e45d82c;
}

[data-theme="light"] .tf-nav-options {
  background: #fff;
  border-color: #d8dfef;
  box-shadow: 0 12px 35px #263b681a;
}

[data-theme="light"] .tf-nav-options button {
  color: #425476;
}
[data-theme="light"] .tf-nav-options button[aria-pressed="true"] {
  color: #1e45d8;
}

[data-theme="light"] .tf-lead,
[data-theme="light"] .tf-hero-note,
[data-theme="light"] .tf-hero-note strong,
[data-theme="light"] .tf-metric span,
[data-theme="light"] .tf-step small,
[data-theme="light"] .tf-footer {
  color: #53617c;
}

[data-theme="light"] .tf-metric strong {
  color: #364bb5;
}
[data-theme="light"] .tf-metric:nth-child(2) strong {
  color: #7952b7;
}
[data-theme="light"] .tf-metric:nth-child(3) strong {
  color: #087e76;
}

[data-theme="light"] .tf-feature {
  background: linear-gradient(135deg, #fff, #eef2ff);
}
[data-theme="light"] .tf-btn-ghost {
  border-color: #536eff44;
  background: #fff8;
}

[data-theme="light"] .tf-visual-card {
  color: #f4f6ff;
  background: linear-gradient(135deg, #18223c, #101525);
}

/* Give longer English copy room without shrinking the 12px labels. */
.tokenfree-home[lang="en"] h1 {
  font-size: clamp(38px, 4.2vw, 58px);
  letter-spacing: -2px;
}

.tokenfree-home[lang="en"] .tf-section h2,
.tokenfree-home[lang="en"] .tf-code-wrap h2 {
  font-size: 34px;
}

.tokenfree-home[lang="en"] .tf-feature h3 {
  line-height: 1.5;
}

.tokenfree-home[lang="en"] .tf-metric span {
  line-height: 1.6;
  padding-right: 12px;
}

.tokenfree-home[lang="en"] .tf-hero-note {
  flex-wrap: wrap;
}

@media (max-width: 520px) {
  .tokenfree-home[lang="en"] h1 {
    font-size: 38px;
  }
  .tokenfree-home[lang="en"] .tf-bottom-cta h2 {
    font-size: 32px;
  }
  .tokenfree-home[lang="en"] .tf-section h2,
  .tokenfree-home[lang="en"] .tf-code-wrap h2 {
    font-size: 29px;
  }
}

[hidden] {
  display: none !important;
}

.tf-trust-section {
  padding-bottom: 60px;
}
.tf-trust-section blockquote {
  margin: 24px 0 48px;
  max-width: 1000px;
  font-size: clamp(25px, 3vw, 38px);
  line-height: 1.65;
  letter-spacing: -1px;
  color: var(--ink);
}

.tf-principle-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 32px;
}
.tf-principle-grid article {
  border-top: 1px solid var(--line);
  padding-top: 22px;
}
.tf-principle-number {
  color: var(--cyan);
  font-size: 12px;
}
.tf-principle-grid h3 {
  font-size: 18px;
  margin: 14px 0;
}
.tf-principle-grid p,
.tf-faq-section p {
  font-size: 14px;
  line-height: 1.9;
  color: var(--muted);
}

.tf-faq-section {
  display: grid;
  grid-template-columns: 0.8fr 1.2fr;
  gap: 75px;
  padding-top: 60px;
}
.tf-faq-intro h2 {
  margin-bottom: 20px;
}
.tf-faq-list details {
  border-bottom: 1px solid var(--line);
  padding: 23px 0;
}
.tf-faq-list details:first-child {
  border-top: 1px solid var(--line);
}
.tf-faq-list summary {
  display: flex;
  justify-content: space-between;
  gap: 20px;
  list-style: none;
  font-size: 15px;
  font-weight: 600;
  cursor: pointer;
  line-height: 1.6;
}
.tf-faq-list summary::-webkit-details-marker {
  display: none;
}
.tf-faq-list summary::after {
  content: "+";
  color: var(--cyan);
  flex: none;
}
.tf-faq-list details[open] summary::after {
  content: "−";
}
.tf-faq-list p {
  margin: 16px 0 0;
}
.tf-faq-list summary:focus-visible {
  outline: 2px solid var(--cyan);
  outline-offset: 8px;
}

.tf-community-section {
  border-top: 1px solid var(--line);
}
.tf-community-actions {
  display: flex;
  justify-content: center;
  gap: 12px;
  flex-wrap: wrap;
}
.tf-community-actions button {
  font-family: inherit;
  color: inherit;
  cursor: pointer;
}
.tf-community-section .tf-copy-status {
  min-height: 20px;
  font-size: 12px;
  margin: 16px 0 0;
  color: var(--cyan);
}

@media (max-width: 720px) {
  .tf-principle-grid {
    grid-template-columns: 1fr;
    gap: 24px;
  }
  .tf-faq-section {
    grid-template-columns: 1fr;
    gap: 24px;
  }
  .tf-trust-section blockquote {
    font-size: 26px;
  }
  .tf-community-actions {
    gap: 10px;
  }
}

.tf-trust-section blockquote {
  max-width: none;
}
.tf-trust-line {
  display: block;
  text-wrap: balance;
}

.tokenfree-home[lang="zh-CN"] .tf-eyebrow-api {
  position: relative;
  top: 1px;
}

.tokenfree-home {
  min-height: 100vh;
  line-height: normal;
}
.tokenfree-home img {
  vertical-align: middle;
}
.tokenfree-home h2 {
  font-weight: 700;
}
.tokenfree-home [id] {
  scroll-margin-top: 95px;
}
.tf-feature h3 {
  font-weight: 700;
}
.tf-principle-grid h3 {
  font-weight: 700;
}
.tf-node img {
  display: block;
}
.tf-copy-status {
  white-space: normal;
}
</style>
