<template>
  <div class="card" data-testid="login-security-settings">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <div class="flex items-center gap-2">
        <Icon name="shield" size="md" class="text-primary-500" />
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t("admin.settings.loginSecurity.title") }}
        </h2>
      </div>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ t("admin.settings.loginSecurity.description") }}
      </p>
    </div>

    <div class="space-y-5 p-6">
      <div v-if="loading" class="flex items-center gap-2 text-gray-500">
        <div class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"></div>
        {{ t("common.loading") }}
      </div>

      <template v-else>
        <div class="rounded-lg border border-sky-200 bg-sky-50 p-4 dark:border-sky-800 dark:bg-sky-900/20">
          <div class="flex items-start">
            <Icon name="infoCircle" size="md" class="mt-0.5 flex-shrink-0 text-sky-500" />
            <p class="ml-3 text-sm text-sky-700 dark:text-sky-300">
              {{ t("admin.settings.loginSecurity.passwordFailureNote") }}
            </p>
          </div>
        </div>

        <div class="flex items-center justify-between gap-4">
          <div>
            <label class="font-medium text-gray-900 dark:text-white">
              {{ t("admin.settings.loginSecurity.enabled") }}
            </label>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t("admin.settings.loginSecurity.enabledHint") }}
            </p>
          </div>
          <Toggle v-model="form.enabled" :label="t('admin.settings.loginSecurity.enabled')" />
        </div>

        <div v-if="form.enabled" class="space-y-6 border-t border-gray-100 pt-5 dark:border-dark-700">
          <div class="grid grid-cols-1 gap-5 sm:grid-cols-2">
            <div>
              <label for="login-security-request-limit" class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t("admin.settings.loginSecurity.requestLimit") }}
              </label>
              <div class="flex flex-wrap items-center gap-2">
                <input id="login-security-request-limit" v-model.number="form.request_limit_per_minute" type="number" min="5" max="300" step="1" class="input w-32 max-w-full" data-testid="login-security-request-limit" />
                <span class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t("admin.settings.loginSecurity.perMinute") }}
                </span>
              </div>
              <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                {{ t("admin.settings.loginSecurity.requestLimitHint") }}
              </p>
            </div>

            <div class="flex items-center justify-between gap-4 rounded-lg border border-gray-100 p-4 dark:border-dark-700">
              <div>
                <label class="font-medium text-gray-900 dark:text-white">
                  {{ t("admin.settings.loginSecurity.ipv6Group") }}
                </label>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t("admin.settings.loginSecurity.ipv6GroupHint") }}
                </p>
              </div>
              <Toggle v-model="form.group_ipv6_by_64" :label="t('admin.settings.loginSecurity.ipv6Group')" />
            </div>
          </div>

          <div class="rounded-lg border border-gray-100 p-4 dark:border-dark-700">
            <h3 class="font-medium text-gray-900 dark:text-white">
              {{ t("admin.settings.loginSecurity.accountIpTitle") }}
            </h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t("admin.settings.loginSecurity.accountIpDescription") }}
            </p>
            <div class="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-3">
              <div>
                <label for="login-security-account-ip-failure-limit" class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t("admin.settings.loginSecurity.failureLimit") }}</label>
                <input id="login-security-account-ip-failure-limit" v-model.number="form.account_ip_failure_limit" type="number" min="3" max="20" step="1" class="input w-full" />
              </div>
              <div>
                <label for="login-security-account-ip-window" class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t("admin.settings.loginSecurity.windowMinutes") }}</label>
                <div class="flex items-center gap-2">
                  <input id="login-security-account-ip-window" v-model.number="form.account_ip_window_minutes" type="number" min="5" max="120" step="1" class="input w-full" />
                  <span class="text-sm text-gray-500">{{ t("admin.settings.loginSecurity.minuteUnit") }}</span>
                </div>
              </div>
              <div>
                <label for="login-security-account-ip-block" class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t("admin.settings.loginSecurity.blockMinutes") }}</label>
                <div class="flex items-center gap-2">
                  <input id="login-security-account-ip-block" v-model.number="form.account_ip_block_minutes" type="number" min="5" max="1440" step="1" class="input w-full" />
                  <span class="text-sm text-gray-500">{{ t("admin.settings.loginSecurity.minuteUnit") }}</span>
                </div>
              </div>
            </div>
          </div>

          <div class="rounded-lg border border-gray-100 p-4 dark:border-dark-700">
            <h3 class="font-medium text-gray-900 dark:text-white">
              {{ t("admin.settings.loginSecurity.accountTitle") }}
            </h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t("admin.settings.loginSecurity.accountDescription") }}
            </p>
            <div class="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-3">
              <div>
                <label for="login-security-account-failure-limit" class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t("admin.settings.loginSecurity.failureLimit") }}</label>
                <input id="login-security-account-failure-limit" v-model.number="form.account_failure_limit" type="number" :min="form.account_ip_failure_limit" max="200" step="1" class="input w-full" />
              </div>
              <div>
                <label for="login-security-account-window" class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t("admin.settings.loginSecurity.windowMinutes") }}</label>
                <div class="flex items-center gap-2">
                  <input id="login-security-account-window" v-model.number="form.account_window_minutes" type="number" min="5" max="240" step="1" class="input w-full" />
                  <span class="text-sm text-gray-500">{{ t("admin.settings.loginSecurity.minuteUnit") }}</span>
                </div>
              </div>
              <div>
                <label for="login-security-account-block" class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t("admin.settings.loginSecurity.blockMinutes") }}</label>
                <div class="flex items-center gap-2">
                  <input id="login-security-account-block" v-model.number="form.account_block_minutes" type="number" min="5" max="1440" step="1" class="input w-full" />
                  <span class="text-sm text-gray-500">{{ t("admin.settings.loginSecurity.minuteUnit") }}</span>
                </div>
              </div>
            </div>
          </div>

        </div>

        <div class="rounded-lg border border-amber-200 bg-amber-50 p-4 dark:border-amber-800 dark:bg-amber-900/20">
          <div class="flex items-start justify-between gap-4">
            <div class="min-w-0">
              <p class="font-medium text-amber-900 dark:text-amber-200">
                {{ t("admin.settings.loginSecurity.adminMfa") }}
              </p>
              <p class="mt-1 text-sm text-amber-700 dark:text-amber-300">
                {{ t("admin.settings.loginSecurity.adminMfaHint") }}
              </p>
              <p class="mt-2 text-xs text-amber-700 dark:text-amber-300">
                {{ t("admin.settings.loginSecurity.adminMfaWarning") }}
              </p>
            </div>
            <Toggle v-model="form.admin_mfa_required" :label="t('admin.settings.loginSecurity.adminMfa')" />
          </div>
        </div>

        <p v-if="!settingsValid" role="alert" class="text-sm text-red-600 dark:text-red-400">
          {{ t("admin.settings.loginSecurity.invalidSettings") }}
        </p>
        <div class="flex justify-end">
          <button type="button" class="btn btn-primary" :disabled="saving || !settingsValid" @click="save">
            {{ saving ? t("common.saving") : t("admin.settings.loginSecurity.save") }}
          </button>
        </div>
      </template>
    </div>

    <TotpStepUpDialog :controller="stepUp" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { useI18n } from "vue-i18n";
import { adminAPI } from "@/api";
import type { LoginSecuritySettings } from "@/api/admin/settings";
import Icon from "@/components/icons/Icon.vue";
import Toggle from "@/components/common/Toggle.vue";
import TotpStepUpDialog from "@/components/auth/TotpStepUpDialog.vue";
import { useAppStore } from "@/stores";
import { useStepUp, isStepUpCancelled } from "@/composables/useStepUp";
import { extractApiErrorMessage, extractI18nErrorMessage } from "@/utils/apiError";

const { t } = useI18n();
const appStore = useAppStore();
const stepUp = useStepUp();
const loading = ref(true);
const saving = ref(false);
const settingsValid = computed(() => {
  const integerBetween = (value: number, min: number, max: number) =>
    Number.isInteger(Number(value)) && Number(value) >= min && Number(value) <= max;
  return integerBetween(form.request_limit_per_minute, 5, 300)
    && integerBetween(form.account_ip_failure_limit, 3, 20)
    && integerBetween(form.account_ip_window_minutes, 5, 120)
    && integerBetween(form.account_ip_block_minutes, 5, 1440)
    && integerBetween(form.account_failure_limit, Number(form.account_ip_failure_limit), 200)
    && integerBetween(form.account_window_minutes, 5, 240)
    && integerBetween(form.account_block_minutes, 5, 1440);
});

const form = reactive<LoginSecuritySettings>({
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
});

function apply(settings: LoginSecuritySettings) {
  Object.assign(form, settings);
}

async function load() {
  loading.value = true;
  try {
    apply(await adminAPI.settings.getLoginSecuritySettings());
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t("admin.settings.loginSecurity.loadFailed")));
  } finally {
    loading.value = false;
  }
}

async function save() {
  if (!settingsValid.value) return;
  saving.value = true;
  try {
    const payload: LoginSecuritySettings = {
      ...form,
      request_limit_per_minute: Number(form.request_limit_per_minute),
      account_ip_failure_limit: Number(form.account_ip_failure_limit),
      account_ip_window_minutes: Number(form.account_ip_window_minutes),
      account_ip_block_minutes: Number(form.account_ip_block_minutes),
      account_failure_limit: Number(form.account_failure_limit),
      account_window_minutes: Number(form.account_window_minutes),
      account_block_minutes: Number(form.account_block_minutes),
    };
    const updated = await stepUp.run(() => adminAPI.settings.updateLoginSecuritySettings(payload));
    apply(updated);
    appStore.showSuccess(t("admin.settings.loginSecurity.saved"));
  } catch (err) {
    if (!isStepUpCancelled(err)) {
      appStore.showError(extractI18nErrorMessage(
        err,
        t,
        "admin.settings.loginSecurity.errors",
        extractApiErrorMessage(err, t("admin.settings.loginSecurity.saveFailed")),
      ));
    }
  } finally {
    saving.value = false;
  }
}

onMounted(load);
</script>
