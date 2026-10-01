import { flushPromises, mount } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";

import LoginSecuritySettings from "../LoginSecuritySettings.vue";

const { getLoginSecuritySettings, updateLoginSecuritySettings, stepUpRun, showSuccess, showError } = vi.hoisted(() => ({
  getLoginSecuritySettings: vi.fn(),
  updateLoginSecuritySettings: vi.fn(),
  stepUpRun: vi.fn((action: () => Promise<unknown>) => action()),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}));

vi.mock("@/api", () => ({
  adminAPI: {
    settings: { getLoginSecuritySettings, updateLoginSecuritySettings },
  },
}));

vi.mock("@/stores", () => ({
  useAppStore: () => ({ showError, showSuccess }),
}));

vi.mock("@/composables/useStepUp", () => ({
  useStepUp: () => ({ run: stepUpRun }),
  isStepUpCancelled: () => false,
}));

vi.mock("vue-i18n", () => ({
  useI18n: () => ({
    t: (key: string) => key === "admin.settings.loginSecurity.errors.ADMIN_MFA_TOTP_REQUIRED"
      ? "请先为当前管理员配置 TOTP，再开启管理员强制 MFA。"
      : key,
  }),
}));

const defaults = {
  enabled: true,
  request_limit_per_minute: 20,
  group_ipv6_by_64: true,
  account_ip_failure_limit: 5,
  account_ip_window_minutes: 30,
  account_ip_block_minutes: 30,
  account_failure_limit: 20,
  account_window_minutes: 30,
  account_block_minutes: 30,
  admin_mfa_required: false,
};

describe("LoginSecuritySettings", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getLoginSecuritySettings.mockResolvedValue({ ...defaults });
    updateLoginSecuritySettings.mockImplementation(async (settings) => settings);
  });

  it("keeps admin MFA and save available when login abuse protection is disabled", async () => {
    const wrapper = mount(LoginSecuritySettings, {
      global: { stubs: { Icon: true, TotpStepUpDialog: true } },
    });
    await flushPromises();

    const switches = wrapper.findAll('[role="switch"]');
    await switches[0].trigger("click");
    await flushPromises();

    expect(wrapper.text()).toContain("admin.settings.loginSecurity.adminMfa");
    expect(wrapper.text()).toContain("admin.settings.loginSecurity.save");
    expect(wrapper.find('[data-testid="login-security-request-limit"]').exists()).toBe(false);

    await wrapper.get("button.btn-primary").trigger("click");
    await flushPromises();

    expect(updateLoginSecuritySettings).toHaveBeenCalledWith({ ...defaults, enabled: false });
    expect(showSuccess).toHaveBeenCalledWith("admin.settings.loginSecurity.saved");
  });

  it("blocks saving numeric values outside the supported ranges", async () => {
    const wrapper = mount(LoginSecuritySettings, {
      global: { stubs: { Icon: true, TotpStepUpDialog: true } },
    });
    await flushPromises();
    await wrapper.get('[data-testid="login-security-request-limit"]').setValue(4);

    expect(wrapper.get("button.btn-primary").attributes("disabled")).toBeDefined();
    expect(wrapper.text()).toContain("admin.settings.loginSecurity.invalidSettings");
    expect(updateLoginSecuritySettings).not.toHaveBeenCalled();
  });

  it("localizes administrator TOTP setup errors", async () => {
    updateLoginSecuritySettings.mockRejectedValue({
      reason: "ADMIN_MFA_TOTP_REQUIRED",
      message: "Configure TOTP for the current administrator before enabling mandatory administrator MFA.",
    });
    const wrapper = mount(LoginSecuritySettings, {
      global: { stubs: { Icon: true, TotpStepUpDialog: true } },
    });
    await flushPromises();

    await wrapper.get("button.btn-primary").trigger("click");
    await flushPromises();

    expect(showError).toHaveBeenCalledWith("请先为当前管理员配置 TOTP，再开启管理员强制 MFA。");
  });
});
