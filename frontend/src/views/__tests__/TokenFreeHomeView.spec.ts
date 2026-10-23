import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { mount, flushPromises, RouterLinkStub } from "@vue/test-utils";
import { nextTick, reactive, ref } from "vue";
import TokenFreeHomeView from "../TokenFreeHomeView.vue";
import { setLocale } from "@/i18n";

const auth = reactive({ isAuthenticated: false, isAdmin: false });
const locale = ref("zh");
vi.mock("@/stores/auth", () => ({ useAuthStore: () => auth }));
vi.mock("vue-i18n", () => ({ useI18n: () => ({ locale }) }));
vi.mock("@/i18n", () => ({
  setLocale: vi.fn(async (language: string) => {
    locale.value = language;
    localStorage.setItem("sub2api_locale", language);
  }),
}));

const mounted: ReturnType<typeof mount>[] = [];
const media = {
  matches: false,
  addEventListener: vi.fn(),
  removeEventListener: vi.fn(),
};
function render() {
  const wrapper = mount(TokenFreeHomeView, {
    global: { stubs: { RouterLink: RouterLinkStub } },
  });
  mounted.push(wrapper);
  return wrapper;
}
function destination(wrapper: ReturnType<typeof render>) {
  return wrapper
    .findAllComponents(RouterLinkStub)
    .find((link) => link.attributes("data-auth-primary") !== undefined)!
    .props("to");
}

beforeEach(() => {
  auth.isAuthenticated = false;
  auth.isAdmin = false;
  locale.value = "zh";
  localStorage.clear();
  media.matches = false;
  vi.clearAllMocks();
  vi.spyOn(window, "matchMedia").mockReturnValue(
    media as unknown as MediaQueryList,
  );
});
afterEach(() => {
  mounted.splice(0).forEach((wrapper) => wrapper.unmount());
  vi.restoreAllMocks();
});

describe("TokenFree homepage integration", () => {
  it("defaults to dark even on a light system and remembers an explicit system preference", async () => {
    media.matches = true;
    const wrapper = render();
    expect(wrapper.attributes("data-theme")).toBe("dark");
    await wrapper.findAll("button").find(b => b.text() === "跟随系统")!.trigger("click");
    expect(wrapper.attributes("data-theme")).toBe("light");
    const remounted = render();
    await nextTick();
    expect(remounted.attributes("data-theme")).toBe("light");
  });

  it("reacts to shared authentication and role changes, without a second session implementation", async () => {
    const wrapper = render();
    expect(destination(wrapper)).toBe("/register");
    expect(wrapper.findAllComponents(RouterLinkStub).find(link => link.text() === "模型广场")?.props("to")).toBe("/models");
    expect(wrapper.get(".tf-nav-auth").text()).toBe("注册登录");
    expect(wrapper.find("iframe").exists()).toBe(false);
    expect(wrapper.text()).not.toMatch(/领取试用|查看开发文档/);
    auth.isAuthenticated = true;
    await nextTick();
    expect(destination(wrapper)).toBe("/dashboard");
    expect(wrapper.get(".tf-nav-auth").text()).toBe("进入控制台");
    auth.isAdmin = true;
    await nextTick();
    expect(destination(wrapper)).toBe("/admin/dashboard");
    expect(
      wrapper
        .findAllComponents(RouterLinkStub)
        .filter((link) => link.text() === "进入控制台")
        .every((link) => link.props("to") === "/admin/dashboard"),
    ).toBe(true);
    auth.isAuthenticated = false;
    await nextTick();
    expect(destination(wrapper)).toBe("/register");
    expect(wrapper.get(".tf-nav-auth").text()).toBe("注册登录");
  });

  it("switches all content through the shared locale and keeps authenticated CTA labels in sync", async () => {
    const wrapper = render();
    const english = wrapper
      .findAll("button")
      .find((b) => b.text() === "English")!;
    await english.trigger("click");
    await flushPromises();
    expect(setLocale).toHaveBeenCalledWith("en");
    expect(localStorage.getItem("sub2api_locale")).toBe("en");
    expect(wrapper.attributes("lang")).toBe("en");
    expect(wrapper.get("h1").text()).toContain("Your ideas.");
    for (const selector of ["#principles", "#faq", "#start"])
      expect(wrapper.get(selector).text()).not.toMatch(/[\u4e00-\u9fff]/);
    expect(wrapper.get(".tf-footer").text()).toContain(
      "Token freedom, at a fair price.",
    );
    expect(wrapper.get("pre").text()).toContain("https://api.tokenfree.dev");
    auth.isAuthenticated = true;
    await nextTick();
    expect(wrapper.get("[data-auth-primary]").text()).toContain(
      "Open dashboard",
    );
    locale.value = "zh";
    await nextTick();
    expect(wrapper.get("[data-auth-primary]").text()).toContain("进入控制台");
    expect(wrapper.get(".tf-eyebrow").text()).toBe("一个API，无限可能。");
    expect(wrapper.get(".tf-footer").text()).toContain(
      "价廉物美，Token 自由。",
    );
  });

  it("persists the app theme convention and cleans up system listeners on unmount", async () => {
    const remove = vi.spyOn(document, "removeEventListener");
    const wrapper = render();
    await wrapper
      .findAll("button")
      .find((b) => b.text() === "浅色")!
      .trigger("click");
    expect(wrapper.attributes("data-theme")).toBe("light");
    expect(localStorage.getItem("theme")).toBe("light");
    expect(document.documentElement.classList.contains("dark")).toBe(false);
    await wrapper
      .findAll("button")
      .find((b) => b.text() === "跟随系统")!
      .trigger("click");
    expect(localStorage.getItem("theme")).toBe(null);
    expect(wrapper.attributes("data-theme")).toBe("dark");
    wrapper.unmount();
    mounted.splice(mounted.indexOf(wrapper), 1);
    expect(media.removeEventListener).toHaveBeenCalledWith(
      "change",
      expect.any(Function),
    );
    expect(remove).toHaveBeenCalledWith("keydown", expect.any(Function));
    expect(remove).toHaveBeenCalledWith("click", expect.any(Function));
  });

  it("provides copy feedback and a readable fallback when clipboard access fails", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    const wrapper = render();
    await wrapper.get("#copy-support").trigger("click");
    await flushPromises();
    expect(writeText).toHaveBeenCalledWith("751077517");
    expect(wrapper.get('[role="status"]').text()).toBe("QQ号已复制。");
    writeText.mockRejectedValueOnce(new Error("unavailable"));
    await wrapper.get("#copy-support").trigger("click");
    await flushPromises();
    expect(wrapper.get('[role="status"]').text()).toContain(
      "请手动复制：751077517",
    );
  });
});
