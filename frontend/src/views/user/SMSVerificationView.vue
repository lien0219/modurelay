<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-[1600px] space-y-5 overflow-x-hidden">
      <header class="flex flex-wrap items-end justify-between gap-3">
        <div class="min-w-0">
          <p class="text-xs font-semibold uppercase tracking-[0.14em] text-cyan-600 dark:text-cyan-400">ModuRelay</p>
          <h1 class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{{ t('nav.smsService') }}</h1>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" :aria-label="t('common.refresh')" @click="loadAll">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" aria-hidden="true" />
        </button>
      </header>

      <div v-if="recentSuccessItems.length" class="flex min-h-11 items-center gap-3 overflow-hidden rounded-xl border border-gray-200 bg-white/70 px-3 py-2 dark:border-dark-700 dark:bg-dark-800/60">
        <span class="flex shrink-0 items-center gap-1.5 text-xs font-semibold text-emerald-600 dark:text-emerald-400">
          <Icon name="checkCircle" size="sm" aria-hidden="true" />
          {{ t('sms.user.recentSuccess') }}
        </span>
        <div class="min-w-0 flex-1 overflow-hidden">
          <div class="sms-success-track flex w-max items-center gap-2">
            <div v-for="(item, index) in recentSuccessLoop" :key="`${index}-${item.username}-${item.phone}`" class="flex shrink-0 items-center gap-2 rounded-lg border border-gray-100 bg-gray-50/80 px-3 py-1.5 text-xs text-gray-600 dark:border-dark-700 dark:bg-dark-900/60 dark:text-gray-300">
              <span class="font-medium text-gray-800 dark:text-gray-100">{{ item.username }}</span>
              <span :class="flagClass(item.country_code)" class="fi fis rounded-sm shadow-sm" aria-hidden="true"></span>
              <span>{{ displayRegionName(item.country_code) }}</span>
              <span class="font-mono text-gray-500">{{ item.phone }}</span>
            </div>
          </div>
        </div>
      </div>

      <div class="flex gap-2 border-b border-gray-200 dark:border-dark-700" role="tablist" :aria-label="t('nav.smsService')">
        <button v-for="tab in tabs" :key="tab.value" type="button" class="border-b-2 px-3 py-2 text-sm font-medium disabled:cursor-not-allowed disabled:opacity-40" :disabled="tab.disabled" :class="productType === tab.value && activeTab !== 'orders' ? 'border-primary-600 text-primary-600' : 'border-transparent text-gray-500'" role="tab" :aria-selected="productType === tab.value && activeTab !== 'orders'" @click="switchProductType(tab.value)">{{ tab.label }}</button>
        <button type="button" class="border-b-2 px-3 py-2 text-sm font-medium" :class="activeTab === 'orders' ? 'border-primary-600 text-primary-600' : 'border-transparent text-gray-500'" role="tab" :aria-selected="activeTab === 'orders'" @click="activeTab = 'orders'; loadOrders()">{{ t('sms.user.orders') }}</button>
      </div>

      <section v-if="activeTab !== 'orders'" class="card space-y-5 p-4 sm:p-5">
        <div>
          <div class="mb-3 flex items-center justify-between gap-3">
            <div>
              <p class="text-xs font-semibold uppercase tracking-wide text-gray-500">1 · {{ t('sms.admin.channels') }}</p>
            </div>
          </div>
          <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <button
              v-for="(provider, index) in providers"
              :key="provider.code"
              type="button"
              class="relative min-h-20 rounded-xl border p-4 text-left transition"
              :class="[
                providerCode === provider.code ? 'border-primary-500 bg-primary-50 ring-1 ring-primary-200 dark:bg-primary-900/20' : 'border-gray-200 dark:border-dark-700',
                !isProviderSelectable(provider) ? 'cursor-not-allowed bg-gray-50 opacity-50 grayscale dark:bg-dark-800' : 'hover:border-primary-300'
              ]"
              :disabled="!isProviderSelectable(provider)"
              :title="providerDisabledReason(provider)"
              @click="switchProvider(provider.code)"
            >
              <div class="flex items-center justify-between gap-2">
                <span class="font-semibold text-gray-900 dark:text-white">{{ t('sms.user.channel') }}{{ index + 1 }}</span>
                <span v-if="provider.beta" class="rounded bg-gray-200 px-2 py-0.5 text-[10px] font-bold tracking-wider text-gray-600 dark:bg-dark-600 dark:text-gray-300">BETA</span>
              </div>
              <div v-if="!isProviderSelectable(provider)" class="mt-2 text-xs text-gray-400">{{ providerDisabledReason(provider) }}</div>
            </button>
          </div>
        </div>

        <div class="sms-selection-grid grid gap-4 lg:grid-cols-3 lg:items-start">
          <div class="grid min-w-0 gap-4 lg:col-span-2 lg:grid-cols-2">
          <div class="flex min-h-0 min-w-0 flex-col rounded-xl border border-gray-200 p-4 dark:border-dark-700">
            <p class="text-xs font-semibold uppercase tracking-wide text-gray-500">2 · {{ t('sms.user.service') }}</p>
            <div class="mt-3"><input v-model.trim="serviceKeyword" class="input h-10 w-full" :placeholder="t('sms.user.serviceSearch')" /></div>
            <div class="mt-3 min-h-0 space-y-2 overflow-y-auto pr-1 lg:max-h-[440px]">
              <button v-for="option in filteredServiceOptions" :key="String(option.value)" type="button" class="flex min-h-14 w-full items-center justify-between gap-3 rounded-lg border px-3 py-2 text-left text-sm transition" :class="serviceCode === option.value ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-900/20' : 'border-gray-200 hover:border-primary-300 dark:border-dark-700'" @click="selectService(String(option.value))">
                <span class="flex min-w-0 items-center gap-3">
                  <SMSServiceLogo :icon="option.logo" :label="option.label" class="h-9 w-9 text-base" />
                  <span class="min-w-0">
                    <span class="block truncate font-medium">{{ option.label }}</span>
                    <span v-if="option.stock != null && option.stock > 0" class="block text-[11px] text-emerald-600 dark:text-emerald-400">{{ t('sms.user.numbersAvailable', { count: option.stock.toLocaleString() }) }}</span>
                  </span>
                </span>
                <span class="shrink-0 text-right">
                  <span v-if="option.startingPrice != null && option.startingPrice > 0" class="block font-semibold tabular-nums text-gray-900 dark:text-white">{{ t('sms.user.startingAt', { price: formatPrice(option.startingPrice) }) }}</span>
                  <span class="block font-mono text-[10px] text-gray-400">{{ option.value }}</span>
                </span>
              </button>
              <div v-if="servicesLoading && !services.length" class="py-8 text-center text-sm text-gray-400">{{ t('sms.user.loading') }}</div>
              <div v-else-if="!filteredServiceOptions.length" class="py-8 text-center text-sm text-gray-400">{{ t('sms.user.noServiceMatch') }}</div>
              <button v-if="servicePagination.hasMore" type="button" class="btn btn-secondary w-full" :disabled="servicesLoading" @click="loadServicePage(false)">{{ servicesLoading ? t('sms.user.loading') : t('sms.user.loadMore') }}</button>
            </div>
          </div>

          <div class="flex min-h-0 min-w-0 flex-col rounded-xl border border-gray-200 p-4 dark:border-dark-700">
            <div class="flex items-center justify-between gap-2">
              <p class="text-xs font-semibold uppercase tracking-wide text-gray-500">3 · {{ t('sms.user.country') }}</p>
              <select v-if="currentProvider?.capabilities.supports_conversion_stats" v-model="countrySortMode" class="input h-8 w-auto min-w-28 py-1 text-xs">
                <option v-for="item in countrySortOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
              </select>
            </div>
            <div class="mt-3"><input v-model.trim="countryKeyword" class="input h-10 w-full" :placeholder="t('sms.user.countrySearch')" :disabled="!serviceCode" /></div>
            <div class="mt-3 min-h-0 space-y-2 overflow-y-auto pr-1 lg:max-h-[440px]">
              <button v-for="option in filteredCountryOptions" :key="String(option.value)" type="button" class="flex min-h-14 w-full items-center justify-between gap-3 rounded-lg border px-3 py-2 text-left text-sm disabled:cursor-not-allowed disabled:opacity-45" :disabled="option.available === false" :class="countryCode === option.value ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-900/20' : 'border-gray-200 hover:border-primary-300 dark:border-dark-700'" @click="selectCountry(String(option.value))">
                <span class="flex min-w-0 items-center gap-3">
                  <span :class="flagClass(String(option.value))" class="fi fis shrink-0 rounded-sm shadow-sm" aria-hidden="true"></span>
                  <span class="min-w-0">
                    <span class="flex items-center gap-2">
                      <span class="block truncate font-medium">{{ option.label }}</span>
                      <span v-if="option.platform30dSuccessRate != null && option.platform30dSampleSize >= 20 && option.platform30dSuccessRate >= 70" class="rounded-full bg-emerald-100 px-2 py-0.5 text-[10px] font-semibold text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300">{{ t('sms.user.platformVerified') }}</span>
                      <span v-else-if="currentProvider?.capabilities.supports_conversion_stats && option.conversionRate != null && option.conversionRate >= 70" class="rounded-full bg-cyan-100 px-2 py-0.5 text-[10px] font-semibold text-cyan-700 dark:bg-cyan-900/40 dark:text-cyan-300">{{ t('sms.user.highDeliveryRate') }}</span>
                    </span>
                    <span class="block text-[11px] text-gray-400">{{ option.value }}<template v-if="option.stock != null"> · {{ t('sms.user.numbersAvailable', { count: option.stock.toLocaleString() }) }}</template></span>
                    <span v-if="currentProvider?.capabilities.supports_conversion_stats && option.conversionRate != null && option.conversionRate > 0" class="mt-0.5 block text-[11px] font-medium text-cyan-600 dark:text-cyan-400">
                      {{ t('sms.user.providerDeliveryRateValue', { rate: option.conversionRate.toFixed(2) }) }}
                      <template v-if="option.recommendedOperator"> · {{ option.recommendedOperator }}</template>
                    </span>
                    <span v-if="option.platform30dSuccessRate != null" class="mt-0.5 block text-[11px] font-medium text-emerald-600 dark:text-emerald-400">
                      {{ t('sms.user.platform30dRateValue', { rate: option.platform30dSuccessRate.toFixed(2), count: option.platform30dSampleSize }) }}
                    </span>
                    <span v-else-if="option.platform30dSampleSize > 0" class="mt-0.5 block text-[11px] text-gray-400">
                      {{ t('sms.user.platform30dInsufficient', { count: option.platform30dSampleSize, minimum: 20 }) }}
                    </span>
                  </span>
                </span>
                <span class="shrink-0 text-right">
                  <span v-if="option.recommendedStartingPrice != null && option.recommendedStartingPrice > 0 && currentProvider?.capabilities.supports_conversion_stats && option.conversionRate != null && option.conversionRate > 0" class="block font-semibold tabular-nums text-gray-900 dark:text-white">{{ t('sms.user.startingAt', { price: formatPrice(option.recommendedStartingPrice) }) }}</span>
                  <span v-else-if="option.startingPrice != null && option.startingPrice > 0" class="block font-semibold tabular-nums text-gray-900 dark:text-white">{{ t('sms.user.startingAt', { price: formatPrice(option.startingPrice) }) }}</span>
                  <span v-if="currentProvider?.capabilities.supports_conversion_stats && option.recommendedOperatorStock != null && option.recommendedOperatorStock > 0" class="block text-[10px] text-gray-400">{{ t('sms.user.recommendedStock', { count: option.recommendedOperatorStock.toLocaleString() }) }}</span>
                </span>
              </button>
              <div v-if="countriesLoading && !countries.length" class="py-8 text-center text-sm text-gray-400">{{ t('sms.user.loading') }}</div>
              <div v-else-if="serviceCode && !filteredCountryOptions.length" class="py-8 text-center text-sm text-gray-400">{{ t('sms.user.noCountryMatch') }}</div>
              <button v-if="countryPagination.hasMore" type="button" class="btn btn-secondary w-full" :disabled="countriesLoading" @click="loadCountryPage(false)">{{ countriesLoading ? t('sms.user.loading') : t('sms.user.loadMore') }}</button>
            </div>
            <p v-if="!serviceCode" class="mt-3 text-xs text-gray-500">{{ t('sms.user.selectService') }}</p>
          </div>

          </div>

          <div class="flex min-h-0 min-w-0 flex-col rounded-xl border border-gray-200 p-4 dark:border-dark-700">
            <p class="text-xs font-semibold uppercase tracking-wide text-gray-500">4 · {{ t('sms.user.confirmSelection') }}</p>
            <div class="mt-4 space-y-4">
              <div class="space-y-3 rounded-xl bg-gray-50 p-3 dark:bg-dark-800/60">
                <div class="flex items-center justify-between gap-3">
                  <span class="text-xs text-gray-500">{{ t('sms.user.selectedService') }}</span>
                  <span class="flex min-w-0 items-center gap-2 font-medium text-gray-900 dark:text-white">
                    <SMSServiceLogo :icon="selectedServiceLogo" :label="serviceLabel(serviceCode)" class="h-8 w-8 text-xs" />
                    <span class="max-w-44 truncate">{{ serviceLabel(serviceCode) || '-' }}</span>
                  </span>
                </div>
                <div class="border-t border-gray-200 dark:border-dark-700"></div>
                <div class="flex items-center justify-between gap-3">
                  <span class="text-xs text-gray-500">{{ t('sms.user.selectedCountry') }}</span>
                  <span class="flex items-center gap-2 font-medium text-gray-900 dark:text-white"><span v-if="countryCode" :class="flagClass(countryCode)" class="fi fis rounded-sm shadow-sm" aria-hidden="true"></span>{{ countryLabel(countryCode) || '-' }}</span>
                </div>
                <div v-if="countryCode && selectedCountryStats" class="border-t border-gray-200 pt-2 text-[11px] dark:border-dark-700">
                  <div v-if="currentProvider?.capabilities.supports_conversion_stats && selectedCountryStats.conversion_rate != null && selectedCountryStats.conversion_rate > 0" class="flex items-center justify-between gap-3 text-cyan-600 dark:text-cyan-400">
                    <span>{{ t('sms.user.providerDeliveryRate') }}</span><span class="font-semibold tabular-nums">{{ selectedCountryStats.conversion_rate.toFixed(2) }}%</span>
                  </div>
                  <div class="mt-1 flex items-center justify-between gap-3" :class="selectedCountryStats.platform_30d_success_rate != null ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-400'">
                    <span>{{ t('sms.user.platform30dSuccessRate') }}</span>
                    <span v-if="selectedCountryStats.platform_30d_success_rate != null" class="font-semibold tabular-nums">{{ selectedCountryStats.platform_30d_success_rate.toFixed(2) }}% · n={{ selectedCountryStats.platform_30d_sample_size || 0 }}</span>
                    <span v-else>{{ t('sms.user.platform30dInsufficientShort', { count: selectedCountryStats.platform_30d_sample_size || 0 }) }}</span>
                  </div>
                </div>
              </div>

              <div v-if="productType === 'rental' && providerCode === 'smspva'" class="space-y-3">
                <div class="grid grid-cols-2 gap-2">
                  <label class="block min-w-0"><span class="input-label">{{ t('sms.user.duration') }}</span><input v-model.number="durationValue" class="input h-[42px]" type="number" min="1" @change="reloadRentalCatalog" /></label>
                  <Select v-model="durationUnit" :label="t('sms.user.unit')" :options="durationUnitOptions" @update:model-value="reloadRentalCatalog" />
                </div>
                <div v-if="currentProvider?.capabilities.supports_rental_multi_service" class="rounded-xl border border-gray-200 p-3 dark:border-dark-700">
                  <div class="flex items-center justify-between gap-2">
                    <div>
                      <div class="text-sm font-medium text-gray-900 dark:text-white">{{ t('sms.user.multiServiceRental') }}</div>
                      <div class="mt-0.5 text-xs text-gray-500">{{ t('sms.user.multiServiceRentalHint') }}</div>
                    </div>
                    <span v-if="additionalRentalServiceCodes.length" class="badge">{{ t('sms.user.additionalServiceCount', { count: additionalRentalServiceCodes.length }) }}</span>
                  </div>
                  <div class="mt-3 max-h-36 space-y-1.5 overflow-y-auto pr-1">
                    <label v-for="item in services.filter(item => item.code !== serviceCode)" :key="item.code" class="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-sm hover:bg-gray-50 dark:hover:bg-dark-800">
                      <input v-model="additionalRentalServiceCodes" type="checkbox" :value="item.code" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" @change="quotes = []; if (countryCode) loadQuotes()" />
                      <SMSServiceLogo :icon="item.icon || serviceLogo(item.code, item.name)" :label="item.name || item.code" class="h-6 w-6 rounded-md text-[10px]" />
                      <span class="min-w-0 flex-1 truncate">{{ item.name || item.code }}</span>
                    </label>
                  </div>
                </div>
              </div>
              <label v-if="currentProvider?.capabilities.supports_voice" class="block"><span class="input-label">{{ t('sms.user.verificationType') }}</span><select v-model.number="voiceMode" class="input h-[42px] w-full" @change="changeVoiceMode"><option v-for="item in voiceModeOptions" :key="item.value" :value="item.value">{{ item.label }}</option></select></label>
              <label v-if="currentProvider?.capabilities.supports_operator_selection" class="block"><span class="input-label">{{ t('sms.user.operator') }}</span><select v-model="operatorCode" class="input h-[42px] w-full" @change="quotes = []; loadQuotes()"><option value="any">{{ t('sms.user.autoOperator') }}</option><option v-for="item in operators.filter(op => op.code !== 'any')" :key="item.code" :value="item.code" :disabled="item.available === false">{{ item.name }}{{ item.provider_rate ? ` · ${t('sms.user.channelReferenceShort')} ${item.provider_rate.toFixed(2)}%` : '' }}{{ item.platform_30d_success_rate != null ? ` · ${t('sms.user.platform30dShort')} ${item.platform_30d_success_rate.toFixed(2)}% (n=${item.platform_30d_sample_size || 0})` : '' }}{{ operatorStockLabel(item) }}</option></select><p v-if="currentProvider?.capabilities.supports_conversion_stats && operatorCode !== 'any'" class="mt-1 text-[11px] text-cyan-600 dark:text-cyan-400">{{ t('sms.user.recommendedOperatorSelected') }}</p></label>
              <label class="block"><span class="input-label">{{ t('sms.user.quantity') }}</span><input v-model.number="purchaseQuantity" class="input h-[42px] w-full" :class="purchaseQuantityError ? 'border-red-500 focus-visible:border-red-500 focus-visible:ring-red-500/30' : ''" type="number" min="1" :max="batchPurchaseLimit" :aria-invalid="purchaseQuantityError ? 'true' : undefined" :aria-describedby="purchaseQuantityError ? 'sms-quantity-error' : undefined" /><span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">{{ t('sms.user.batchPurchaseHint', { max: batchPurchaseLimit }) }}</span><span v-if="purchaseQuantityError" id="sms-quantity-error" class="mt-1 block text-xs text-red-600 dark:text-red-400" role="alert">{{ purchaseQuantityError }}</span></label>

              <div v-if="bestQuote" class="flex items-center justify-between border-t border-gray-200 pt-3 dark:border-dark-700">
                <span class="text-sm text-gray-500">{{ t('sms.user.price') }}</span>
                <span class="text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatPrice(bestQuote.sale_price) }}</span>
              </div>
              <button type="button" class="btn btn-primary w-full" :disabled="!serviceCode || !countryCode || quoting || purchasing || !isPurchaseQuantityValid" @click="confirmSelection">
                {{ purchasing ? t('sms.user.processing') : quoting ? t('sms.user.quoting') : bestQuote ? t('sms.user.purchase') : t('sms.user.getQuote') }}
              </button>
              <p v-if="currentProvider?.capabilities.supports_refund && productType === 'temporary'" class="text-center text-xs text-gray-500">{{ t('sms.user.refundGuarantee') }}</p>
              <div v-if="countryCode" class="rounded-xl border border-amber-200 bg-amber-50/70 p-3 dark:border-amber-900/50 dark:bg-amber-950/20">
                <div class="flex gap-2.5">
                  <span class="mt-0.5 text-amber-600"><Icon name="shield" size="sm" aria-hidden="true" /></span>
                  <div><div class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('sms.user.deliveryTipTitle') }}</div><div class="mt-1 text-xs leading-5 text-gray-600 dark:text-gray-300">{{ t('sms.user.deliveryTipBody') }}</div></div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section v-if="activeTab !== 'orders' && liveOrders.length" class="space-y-3">
        <div v-for="order in liveOrders" :key="order.id" class="card p-5">
          <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <span class="badge" :class="statusClass(order.status)">{{ statusLabel(order.status, order.reconciliation_action) }}</span>
                <span class="text-sm font-medium text-gray-900 dark:text-white">{{ serviceLabel(order.service_code) }}</span>
                <span class="text-gray-300 dark:text-dark-600">·</span>
                <span class="flex items-center gap-1.5 text-sm text-gray-600 dark:text-gray-300">
                  <span :class="flagClass(order.country_code)" class="fi fis rounded-sm shadow-sm" aria-hidden="true"></span>
                  {{ countryLabel(order.country_code) }}
                </span>
              </div>
              <div v-if="order.status === 'reconciling' && order.reconciliation_action === 'purchase'" class="mt-3 rounded-lg border border-blue-200 bg-blue-50 px-3 py-2 text-xs text-blue-700 dark:border-blue-900/60 dark:bg-blue-950/30 dark:text-blue-300">
                {{ t('sms.user.purchaseConfirmingInline') }}
              </div>
              <div class="mt-3 flex flex-wrap items-center gap-x-6 gap-y-3">
                <div>
                  <div class="text-xs text-gray-500">{{ t('sms.user.phone') }}</div>
                  <div class="mt-1 flex items-center gap-2">
                    <span class="font-mono text-base font-semibold text-gray-900 dark:text-white">{{ order.phone_number || '-' }}</span>
                    <button v-if="order.phone_number" type="button" class="btn btn-secondary btn-sm" @click="copyText(order.phone_number)">{{ t('common.copy') }}</button>
                  </div>
                </div>
                <div>
                  <div class="text-xs text-gray-500">{{ t('sms.user.expiresIn') }}</div>
                  <div class="mt-1 font-mono text-base font-semibold tabular-nums text-gray-900 dark:text-white">{{ remainingLabel(order) }}</div>
                </div>
                <div>
                  <div class="text-xs text-gray-500">{{ t('sms.user.code') }}</div>
                  <div v-if="latestVerificationCode(order)" class="mt-1 flex items-center gap-2">
                    <code class="rounded bg-emerald-50 px-2 py-1 font-mono text-base font-bold text-emerald-700 dark:bg-emerald-950/30 dark:text-emerald-300">{{ latestVerificationCode(order) }}</code>
                    <button type="button" class="btn btn-secondary btn-sm" @click="copyText(latestVerificationCode(order))">{{ t('common.copy') }}</button>
                  </div>
                  <div v-else class="mt-1 text-sm text-gray-400">{{ isOrderWaiting(order) ? t('sms.user.waitingForCode') : '-' }}</div>
                </div>
              </div>
            </div>
            <div class="flex shrink-0 flex-wrap gap-2">
              <button v-if="showCancelAction(order)" type="button" class="btn btn-secondary" :disabled="!canCancelOrder(order)" :title="cancelActionHint(order)" @click="cancel(order)">{{ t('sms.user.cancel') }}</button>
              <button v-if="order.status === 'active' && order.product_type === 'temporary' && order.capabilities?.supports_resend" type="button" class="btn btn-secondary" @click="resend(order.id)">{{ t('sms.user.resend') }}</button>
            </div>
          </div>
        </div>
      </section>

      <section v-if="activeTab !== 'orders'" class="space-y-3">
        <div v-if="quoting" class="card p-8 text-center text-sm text-gray-500">{{ t('sms.user.checkingStock') }}</div>
        <div v-else-if="!quotes.length" class="card p-8 text-center text-sm text-gray-500">{{ serviceCode && countryCode ? t('sms.user.noChannel') : t('sms.user.chooseForQuote') }}</div>
        <div v-for="quote in quotes" :key="quote.quote_id" class="card flex flex-col gap-4 p-5 sm:flex-row sm:items-center sm:justify-between">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h2 class="font-semibold text-gray-900 dark:text-white">{{ quote.public_name }}</h2>
              <span class="badge" :class="quote.channel_role === 'primary' ? 'badge-info' : 'badge-gray'">{{ quote.channel_role === 'primary' ? t('sms.admin.primary') : t('sms.admin.backup') }}</span>
            </div>
            <div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-sm text-gray-500 dark:text-gray-400">
              <span>{{ t('sms.user.stock') }}: {{ quote.stock }}</span>
              <span>{{ t('sms.user.eta') }}: {{ quote.estimated_delivery_seconds }}{{ t('sms.user.seconds') }}</span>
              <span v-if="quote.success_rate != null">{{ t('sms.user.platform30dSuccessRate') }}: {{ (quote.success_rate * 100).toFixed(2) }}% · n={{ quote.success_rate_sample_size || 0 }}{{ t('sms.user.separator') }}{{ quote.success_rate_grade || '-' }}</span>
              <span v-else>{{ t('sms.user.platform30dInsufficient', { count: quote.success_rate_sample_size || 0, minimum: 20 }) }}</span>
              <span v-if="productType === 'temporary' && quote.capabilities.supports_cancel && quote.capabilities.supports_refund">{{ t('sms.user.capabilities.cancelRefund') }}</span>
              <template v-else>
                <span>{{ quote.capabilities.supports_cancel ? t('sms.user.capabilities.cancel') : t('sms.user.capabilities.noCancel') }}</span>
                <span>{{ quote.capabilities.supports_refund ? t('sms.user.capabilities.refund') : t('sms.user.capabilities.noRefund') }}</span>
              </template>
            </div>
          </div>
          <div class="flex items-center justify-between gap-4 sm:justify-end">
            <div class="text-right"><div class="text-lg font-semibold tabular-nums text-gray-900 dark:text-white">{{ quote.sale_price.toFixed(4) }}</div><div class="text-xs text-gray-500">{{ t('sms.user.currency') }}</div></div>
            <button type="button" class="btn btn-primary" :disabled="purchasing || !isPurchaseQuantityValid" @click="purchase(quote)">{{ purchasing ? t('sms.user.processing') : purchaseQuantity > 1 ? t('sms.user.batchPurchase') : t('sms.user.purchase') }}</button>
          </div>
        </div>
      </section>

      <section v-else class="space-y-3">
        <form class="glass-panel grid items-end gap-3 rounded-xl p-4 md:grid-cols-[minmax(240px,1fr)_220px_auto]" @submit.prevent="applyOrderFilters">
          <label class="block min-w-0"><span class="input-label">{{ t('verificationRecords.filters.keyword') }}</span><input v-model.trim="orderDraft.keyword" class="input h-[42px]" :placeholder="t('verificationRecords.filters.keywordPlaceholder')" /></label>
          <Select v-model="orderDraft.status" :label="t('verificationRecords.filters.outcome')" :options="orderStatusOptions" :placeholder="t('verificationRecords.filters.allOutcomes')" clearable searchable />
          <div class="flex h-[42px] items-center gap-2 self-end"><button type="submit" class="btn btn-primary h-[42px]" :disabled="ordersLoading"><Icon name="search" size="sm" aria-hidden="true" />{{ t('common.search') }}</button><button type="button" class="btn btn-secondary h-[42px]" :disabled="ordersLoading" @click="resetOrderFilters"><Icon name="eraser" size="sm" aria-hidden="true" />{{ t('common.reset') }}</button></div>
        </form>
        <div v-if="ordersLoading" class="card p-8 text-center text-sm text-gray-500">{{ t('sms.user.loading') }}</div>
        <div v-else-if="!orders.length" class="card p-8 text-center text-sm text-gray-500">{{ t('sms.user.noOrders') }}</div>
        <div
          v-else
          class="sms-orders-scroll card max-w-full overflow-x-auto focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500"
          tabindex="0"
          :aria-label="t('sms.user.orders')"
        >
          <table class="sms-orders-table w-full min-w-[2200px] table-fixed text-left text-sm">
            <colgroup>
              <col class="w-[320px]" />
              <col class="w-[120px]" />
              <col class="w-[200px]" />
              <col class="w-[160px]" />
              <col class="w-[130px]" />
              <col class="w-[220px]" />
              <col class="w-[220px]" />
              <col class="w-[340px]" />
              <col class="w-[110px]" />
              <col class="w-[130px]" />
              <col class="w-[250px]" />
            </colgroup>
            <thead class="whitespace-nowrap bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3">{{ t('sms.user.order') }}</th>
                <th class="px-4 py-3">{{ t('sms.user.channel') }}</th>
                <th class="px-4 py-3">{{ t('sms.user.service') }}</th>
                <th class="px-4 py-3">{{ t('sms.user.country') }}</th>
                <th class="px-4 py-3">{{ t('sms.user.operator') }}</th>
                <th class="px-4 py-3">{{ t('sms.user.phone') }}</th>
                <th class="px-4 py-3">{{ t('sms.user.status') }}</th>
                <th class="px-4 py-3">{{ t('sms.user.code') }}</th>
                <th class="px-4 py-3">{{ t('sms.user.price') }}</th>
                <th class="px-4 py-3">{{ t('sms.user.expiresIn') }}</th>
                <th class="px-4 py-3">{{ t('sms.user.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="order in orders" :key="order.id" class="border-t border-gray-100 align-middle dark:border-dark-700">
                <td class="whitespace-nowrap px-4 py-3">
                  <div class="flex items-center gap-2">
                    <span class="font-mono text-xs">{{ order.id }}</span>
                    <button type="button" class="btn btn-secondary btn-sm shrink-0" :title="t('common.copy')" :aria-label="`${t('common.copy')} ${order.id}`" @click="copyText(order.id)"><Icon name="copy" size="xs" aria-hidden="true" /></button>
                  </div>
                </td>
                <td class="whitespace-nowrap px-4 py-3"><span class="block truncate" :title="order.channel_name || order.channel_code">{{ order.channel_name || order.channel_code }}</span></td>
                <td class="whitespace-nowrap px-4 py-3">
                  <div class="flex min-w-0 items-center gap-2">
                    <SMSServiceLogo :icon="serviceIcon(order.service_code, serviceLabel(order.service_code))" :label="serviceLabel(order.service_code)" class="h-7 w-7 rounded-md text-xs" />
                    <span class="block min-w-0 truncate" :title="serviceLabel(order.service_code)">{{ serviceLabel(order.service_code) }}</span>
                  </div>
                </td>
                <td class="whitespace-nowrap px-4 py-3">
                  <div class="flex min-w-0 items-center gap-2">
                    <span :class="flagClass(order.country_code)" class="fi fis shrink-0 rounded-sm shadow-sm" aria-hidden="true"></span>
                    <span class="block min-w-0 truncate" :title="countryLabel(order.country_code)">{{ countryLabel(order.country_code) }}</span>
                  </div>
                </td>
                <td class="whitespace-nowrap px-4 py-3 font-mono text-xs"><span class="block truncate" :title="order.operator_code || 'any'">{{ order.operator_code || 'any' }}</span></td>
                <td class="whitespace-nowrap px-4 py-3">
                  <div class="flex items-center gap-2">
                    <span class="font-mono">{{ order.phone_number || '-' }}</span>
                    <button v-if="order.phone_number" type="button" class="btn btn-secondary btn-sm shrink-0" @click="copyText(order.phone_number)">{{ t('common.copy') }}</button>
                  </div>
                </td>
                <td class="whitespace-nowrap px-4 py-3">
                  <div class="sms-order-statuses flex min-w-max items-center gap-1.5">
                    <span class="badge whitespace-nowrap" :class="statusClass(order.status)">{{ statusLabel(order.status, order.reconciliation_action) }}</span>
                    <span v-if="order.refund_status !== 'not_requested'" class="badge badge-warning whitespace-nowrap">{{ refundLabel(order.refund_status) }}</span>
                  </div>
                </td>
                <td class="whitespace-nowrap px-4 py-3">
                  <div v-if="order.messages?.length" class="space-y-2">
                    <div v-for="message in order.messages" :key="message.id" class="min-w-0 max-w-[308px]">
                      <div v-if="message.verification_code" class="flex min-w-0 items-center gap-2">
                        <code class="block min-w-0 flex-1 truncate font-mono font-semibold" :title="message.verification_code">{{ message.verification_code }}</code>
                        <button type="button" class="btn btn-secondary btn-sm shrink-0" @click="copyText(message.verification_code)">{{ t('common.copy') }}</button>
                      </div>
                      <div class="mt-1 flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-[11px] text-gray-500 dark:text-gray-400">
                        <span v-if="message.sender" class="min-w-0 max-w-[14rem] truncate font-medium text-gray-700 dark:text-gray-300" :title="message.sender">{{ message.sender }}</span>
                        <time v-if="message.provider_received_at" class="shrink-0 tabular-nums" :datetime="message.provider_received_at">{{ formatSMSMessageTime(message.provider_received_at) }}</time>
                        <span v-if="message.other_sms" class="badge whitespace-nowrap">{{ t('sms.user.otherMessage') }}</span>
                      </div>
                      <div class="sms-order-message-text mt-1 block truncate text-xs text-gray-500 dark:text-gray-400" :title="message.message_text || undefined">{{ message.message_text || '-' }}</div>
                    </div>
                  </div>
                  <span v-else>-</span>
                </td>
                <td class="whitespace-nowrap px-4 py-3 tabular-nums">{{ order.price.toFixed(4) }}</td>
                <td class="whitespace-nowrap px-4 py-3 tabular-nums">{{ remainingLabel(order) }}</td>
                <td class="whitespace-nowrap px-4 py-3">
                  <div class="flex min-w-max items-center gap-2">
                    <button v-if="showCancelAction(order)" type="button" class="btn btn-secondary btn-sm" :disabled="!canCancelOrder(order)" :title="cancelActionHint(order)" @click="cancel(order)">{{ t('sms.user.cancel') }}</button>
                    <button v-if="order.status === 'active' && order.product_type === 'rental' && order.capabilities?.supports_rental_add_service" type="button" class="btn btn-secondary btn-sm" @click="openRentalServiceModal(order)">{{ t('sms.user.addService') }}</button>
                    <button v-if="order.status === 'active' && order.product_type === 'rental' && order.capabilities?.supports_extend" type="button" class="btn btn-secondary btn-sm" @click="extend(order.id)">{{ t('sms.user.extend') }}</button>
                    <button v-if="canRestoreRental(order)" type="button" class="btn btn-secondary btn-sm" @click="restoreRental(order)">{{ t('sms.user.restoreRental') }}</button>
                    <button v-if="order.status === 'active' && order.product_type === 'temporary' && order.capabilities?.supports_resend" type="button" class="btn btn-secondary btn-sm" @click="resend(order.id)">{{ t('sms.user.resend') }}</button>
                    <button type="button" class="btn btn-secondary btn-sm" :disabled="refreshingId === order.id" :aria-label="t('common.refresh')" @click="refreshOrder(order.id)"><Icon name="refresh" size="sm" :class="refreshingId === order.id ? 'animate-spin' : ''" aria-hidden="true" /></button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="orderPagination.total > 0" class="card overflow-hidden"><Pagination :page="orderPagination.page" :total="orderPagination.total" :page-size="orderPagination.pageSize" @update:page="changeOrderPage" @update:page-size="changeOrderPageSize" /></div>
      </section>

      <div v-if="rentalServiceState.show" class="fixed inset-0 z-50 flex items-center justify-center bg-black/45 p-4" @click.self="closeRentalServiceModal">
        <div class="card w-full max-w-lg space-y-4 p-5 shadow-xl">
          <div class="flex items-start justify-between gap-3">
            <div>
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('sms.user.addServiceTitle') }}</h3>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('sms.user.addServiceDescription') }}</p>
            </div>
            <button type="button" class="btn btn-secondary btn-sm" @click="closeRentalServiceModal">{{ t('common.close') }}</button>
          </div>
          <label class="block">
            <span class="input-label">{{ t('sms.user.rentDays') }}</span>
            <div class="mt-1 flex gap-2">
              <input v-model.number="rentalServiceState.rentDays" class="input flex-1" type="number" min="1" max="366" />
              <button type="button" class="btn btn-secondary" :disabled="rentalServiceState.loading" @click="loadRentalServiceOptions">{{ t('common.refresh') }}</button>
            </div>
          </label>
          <label class="block">
            <span class="input-label">{{ t('sms.user.service') }}</span>
            <select v-model="rentalServiceState.selectedService" class="input mt-1 w-full" :disabled="rentalServiceState.loading">
              <option value="">{{ t('sms.user.selectService') }}</option>
              <option v-for="item in rentalServiceState.options" :key="item.code" :value="item.code">
                {{ item.name }} · {{ formatPrice(item.sale_price) }} {{ t('sms.user.currency') }}
              </option>
            </select>
          </label>
          <div v-if="rentalServiceState.loading" class="py-4 text-center text-sm text-gray-500">{{ t('sms.user.loading') }}</div>
          <div v-else-if="!rentalServiceState.options.length" class="rounded-lg bg-gray-50 p-3 text-sm text-gray-500 dark:bg-dark-800">{{ t('sms.user.noAdditionalServices') }}</div>
          <div class="flex justify-end gap-2">
            <button type="button" class="btn btn-secondary" @click="closeRentalServiceModal">{{ t('common.cancel') }}</button>
            <button type="button" class="btn btn-primary" :disabled="!rentalServiceState.selectedService || rentalServiceState.loading" @click="quoteRentalServiceAddition">{{ t('sms.user.getQuote') }}</button>
          </div>
        </div>
      </div>

      <ConfirmDialog
        :show="confirmState.show"
        :title="confirmState.title"
        :message="confirmState.message"
        :confirm-text="confirmState.confirmText"
        :cancel-text="confirmState.cancelText"
        :danger="confirmState.danger"
        :confirming="confirmingAction"
        @confirm="runConfirmedAction"
        @cancel="closeConfirm"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatDateTime } from '@/utils/format'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import SMSServiceLogo from '@/components/sms/SMSServiceLogo.vue'
import { smsAPI, type SMSCountryItem, type SMSOperatorItem, type SMSOrder, type SMSOrderPage, type SMSProviderItem, type SMSQuote, type SMSRecentSuccessItem, type SMSRentalServiceOption, type SMSServiceItem } from '@/api/sms'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useAppStore, useAuthStore } from '@/stores'

const { locale, t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const productType = ref<'temporary' | 'rental'>('temporary')
const activeTab = ref<'temporary' | 'rental' | 'orders'>('temporary')
const providers = ref<SMSProviderItem[]>([])
const services = ref<SMSServiceItem[]>([])
const providerCode = ref('')
const countries = ref<SMSCountryItem[]>([])
const operators = ref<SMSOperatorItem[]>([])
const operatorCode = ref('any')
const voiceMode = ref(0)
const serviceKeyword = ref('')
const countryKeyword = ref('')
const countrySortMode = ref<'recommended' | 'platform' | 'rate' | 'price' | 'stock' | 'name'>('recommended')
const serviceCode = ref('')
const additionalRentalServiceCodes = ref<string[]>([])
const countryCode = ref('')
const durationValue = ref(1)
const durationUnit = ref('week')
const quotes = ref<SMSQuote[]>([])
const orders = ref<SMSOrder[]>([])
const liveOrders = ref<SMSOrder[]>([])
const recentSuccessItems = ref<SMSRecentSuccessItem[]>([])
const loading = ref(false)
const quoting = ref(false)
const servicesLoading = ref(false)
const countriesLoading = ref(false)
const servicePagination = reactive({ page: 1, pageSize: 20, total: 0, hasMore: false })
const countryPagination = reactive({ page: 1, pageSize: 20, total: 0, hasMore: false })
const ordersLoading = ref(false)
const purchasing = ref(false)
const purchaseQuantity = ref(1)
const batchPurchaseLimit = ref(5)
const purchaseQuantityError = computed(() => {
  const value = Number(purchaseQuantity.value)
  if (!Number.isInteger(value)) return t('sms.user.errors.quantityInteger', { max: batchPurchaseLimit.value })
  if (value < 1 || value > batchPurchaseLimit.value) return t('sms.user.errors.quantityRange', { max: batchPurchaseLimit.value })
  return ''
})
const isPurchaseQuantityValid = computed(() => purchaseQuantityError.value === '')
const refreshingId = ref('')
const orderPagination = reactive({ page: 1, pageSize: getPersistedPageSize(20), total: 0 })
const orderDraft = reactive({ keyword: '', status: '' })
const orderFilters = reactive({ keyword: '', status: '' })
const confirmState = reactive({ show: false, title: '', message: '', confirmText: '', cancelText: '', danger: false, action: null as null | (() => Promise<void>) })
const confirmingAction = ref(false)
const rentalServiceState = reactive({
  show: false,
  orderId: '',
  rentDays: 7,
  selectedService: '',
  options: [] as SMSRentalServiceOption[],
  loading: false,
})
let pollTimer: number | undefined
let countdownTimer: number | undefined
let serviceSearchTimer: number | undefined
let countrySearchTimer: number | undefined

const currentProvider = computed(() => providers.value.find(item => item.code === providerCode.value))
const tabs = computed(() => [
  { value: 'temporary' as const, label: t('sms.user.temporary'), disabled: !currentProvider.value?.capabilities.supports_temporary },
  { value: 'rental' as const, label: t('sms.user.rental'), disabled: !currentProvider.value?.capabilities.supports_rental },
])
function isProviderSelectable(provider: SMSProviderItem) {
  return provider.selectable && (productType.value !== 'rental' || provider.capabilities.supports_rental)
}
function providerDisabledReason(provider: SMSProviderItem) {
  if (!provider.selectable) return t('sms.user.unavailable')
  if (productType.value === 'rental' && !provider.capabilities.supports_rental) return t('sms.user.rentalUnavailable')
  return ''
}
const selectedService = computed(() => services.value.find(item => item.code === serviceCode.value))
const selectedServiceLogo = computed(() => serviceLogo(selectedService.value?.code || '', selectedService.value?.name || ''))
const selectedCountryStats = computed(() => countries.value.find(item => item.iso2 === countryCode.value))
const bestQuote = computed(() => [...quotes.value].sort((a, b) => a.sale_price - b.sale_price)[0])
const recentSuccessLoop = computed(() => recentSuccessItems.value.length ? [...recentSuccessItems.value, ...recentSuccessItems.value] : [])
const serviceOptions = computed(() => services.value.map(item => ({ value: item.code, label: item.name || item.code, logo: item.icon || serviceLogo(item.code, item.name), stock: item.stock, startingPrice: item.starting_price })))
const filteredServiceOptions = computed(() => serviceOptions.value)
const countryOptions = computed(() => countries.value.map(item => ({
  value: item.iso2,
  label: countryName(item),
  stock: item.stock,
  startingPrice: item.starting_price,
  conversionRate: item.conversion_rate,
  platform30dSuccessRate: item.platform_30d_success_rate,
  platform30dSampleSize: item.platform_30d_sample_size || 0,
  platform30dSuccesses: item.platform_30d_successes || 0,
  platform30dFailures: item.platform_30d_failures || 0,
  recommendedOperator: item.recommended_operator,
  recommendedOperatorStock: item.recommended_operator_stock,
  recommendedStartingPrice: item.recommended_starting_price,
  available: item.available,
})))
const filteredCountryOptions = computed(() => countryOptions.value)
const countrySortOptions = computed(() => [
  { value: 'recommended', label: t('sms.user.sortRecommended') },
  { value: 'platform', label: t('sms.user.sortByPlatform30d') },
  { value: 'rate', label: t('sms.user.sortByRate') },
  { value: 'price', label: t('sms.user.sortByPrice') },
  { value: 'stock', label: t('sms.user.sortByStock') },
  { value: 'name', label: t('sms.user.sortByName') },
])
const durationUnitOptions = computed(() => {
  if (providerCode.value === 'smspva') {
    return [{ value: 'week', label: t('sms.user.week') }, { value: 'month', label: t('sms.user.month') }]
  }
  return [{ value: 'hour', label: t('sms.user.hour') }, { value: 'day', label: t('sms.user.day') }, { value: 'week', label: t('sms.user.week') }]
})
const voiceModeOptions = computed(() => {
  const cap = currentProvider.value?.capabilities
  if (!cap) return [{ value: 0, label: t('sms.user.smsType') }]
  const items: Array<{ value: number; label: string }> = []
  if (cap.supports_voice_sms || !cap.supports_voice) items.push({ value: 0, label: t('sms.user.smsType') })
  if (cap.supports_voice_caller_id) items.push({ value: 1, label: t('sms.user.callerIdType') })
  if (cap.supports_voice_call) items.push({ value: 2, label: t('sms.user.voiceType') })
  return items.length ? items : [{ value: 0, label: t('sms.user.smsType') }]
})
const orderStatuses = ['pending', 'active', 'reconciling', 'provider_unknown', 'completed', 'cancelled', 'failed', 'refunded', 'expired']
const orderStatusOptions = computed(() => orderStatuses.map(value => ({ value, label: statusLabel(value) })))
const serviceLabel = (code: string) => services.value.find(item => item.code === code)?.name || code
const countryLabel = (code: string) => {
  const item = countries.value.find(country => country.iso2.toUpperCase() === code.toUpperCase())
  return item ? countryName(item) : displayRegionName(code)
}
function errorMessage(error: unknown, fallback: string) {
  const candidate = error as { message?: string; code?: string | number; reason?: string; metadata?: Record<string, string | number> }
  const reason = candidate?.reason || (typeof candidate?.code === 'string' ? candidate.code : '')
  if (reason === 'BATCH_PURCHASE_LIMIT_EXCEEDED') {
    const configuredMax = Number(candidate?.metadata?.max)
    if (Number.isInteger(configuredMax) && configuredMax >= 1 && configuredMax <= 50) {
      batchPurchaseLimit.value = configuredMax
      purchaseQuantity.value = Math.min(configuredMax, Math.max(1, Number(purchaseQuantity.value) || 1))
      return t('sms.user.errors.batchLimitExceeded', { max: configuredMax })
    }
  }
  if (reason === 'CANCEL_TOO_EARLY') return t('sms.user.errors.cancelTooEarly')
  if (reason === 'CANCEL_TOO_LATE') return t('sms.user.ended')
  if (reason === 'INSUFFICIENT_STOCK') return t('sms.user.errors.insufficientStock')
  if (reason === 'PROVIDER_NO_STOCK') return t('sms.user.errors.providerNoStock')
  if (reason === 'PROVIDER_PRICE_LIMIT') return t('sms.user.errors.providerPriceLimit')
  if (reason === 'PROVIDER_BALANCE_LOW') return t('sms.user.errors.providerBalanceLow')
  if (reason === 'PROVIDER_RATE_LIMITED') return t('sms.user.errors.providerRateLimited')
  if (reason === 'PROVIDER_AUTH_FAILED') return t('sms.user.errors.providerAuthFailed')
  if (reason === 'PROVIDER_COUNTRY_INVALID') return t('sms.user.errors.providerCountryInvalid')
  if (reason === 'PROVIDER_OPERATOR_INVALID') return t('sms.user.errors.providerOperatorInvalid')
  if (reason === 'PROVIDER_SERVICE_INVALID') return t('sms.user.errors.providerServiceInvalid')
  if (reason === 'PROVIDER_UPSTREAM_ERROR') return t('sms.user.errors.providerUpstreamError')
  return candidate?.message || fallback
}
const isOrderWaiting = (order: SMSOrder) => ['pending', 'active', 'provider_unknown', 'reconciling'].includes(order.status) && !isTerminalOrder(order)
const terminalOrderStatuses = new Set(['completed', 'cancelled', 'canceled', 'failed', 'refunded', 'expired'])
const terminalReconciliationActions = new Set(['cancel', 'refund', 'expire'])
const isTerminalOrder = (order: SMSOrder) => {
  const status = String(order.status || '').toLowerCase()
  const action = String(order.reconciliation_action || '').toLowerCase()
  return terminalOrderStatuses.has(status)
    || (status === 'reconciling' && terminalReconciliationActions.has(action))
    || ['approved', 'released', 'succeeded'].includes(String(order.refund_status || '').toLowerCase())
}

function cancellationRemainingSeconds(order: SMSOrder): number | null {
  const explicit = [order.cancel_remaining_seconds, order.cancellation_remaining_seconds, order.cancel_after_seconds]
    .find(value => typeof value === 'number' && Number.isFinite(value))
  if (explicit != null) return Math.max(0, Math.ceil(explicit))
  const availableAt = order.cancel_available_at || order.cancellation_available_at || order.cancel_after
  if (availableAt) {
    const timestamp = new Date(availableAt).getTime()
    if (Number.isFinite(timestamp)) return Math.max(0, Math.ceil((timestamp - Date.now()) / 1000))
  }
  return null
}

function showCancelAction(order: SMSOrder) {
  if (isTerminalOrder(order)) return false
  if (order.status === 'reconciling' && order.reconciliation_action === 'purchase') {
    return order.capabilities?.supports_cancel === true || order.capabilities?.supports_refund === true
  }
  if (order.status !== 'active') return false
  return order.product_type === 'rental'
    ? order.capabilities?.supports_rental_cancel === true
    : order.capabilities?.supports_cancel === true || order.capabilities?.supports_refund === true
}

function canCancelOrder(order: SMSOrder) {
  if (!showCancelAction(order) || order.status !== 'active') return false
  if (order.can_cancel === false || order.cancellation_available === false) return false
  if (order.can_cancel === true || order.cancellation_available === true) return true
  const remaining = cancellationRemainingSeconds(order)
  if (remaining != null) return remaining <= 0
  // Newer backends provide the cancellation window. Until then, keep
  // temporary cancellation closed rather than allowing an unsafe early refund.
  return order.product_type === 'rental'
}

function cancelActionHint(order: SMSOrder) {
  if (order.status === 'reconciling') return t('sms.user.cancelWaitingForConfirmation')
  if (canCancelOrder(order)) return t('sms.user.cancelAvailable')
  const remaining = cancellationRemainingSeconds(order)
  if (remaining != null && remaining > 0) return t('sms.user.cancelUnavailableUntil', { time: formatDuration(remaining) })
  return t('sms.user.cancelUnavailable')
}

function formatDuration(seconds: number) {
  const minutes = Math.floor(Math.max(0, seconds) / 60)
  const remainder = Math.max(0, seconds) % 60
  return `${minutes}:${String(remainder).padStart(2, '0')}`
}
const pollingCandidates = () => [...liveOrders.value, ...(activeTab.value === 'orders' ? orders.value : [])]
  .filter(isOrderWaiting)
  .filter((order, index, items) => items.findIndex(item => item.id === order.id) === index)
  .slice(0, 20)

function stopOrderPolling() {
  if (pollTimer) window.clearTimeout(pollTimer)
  pollTimer = undefined
}

function ensureOrderPolling() {
  if (pollTimer || pollingCandidates().length === 0) return
  pollTimer = window.setTimeout(async () => {
    pollTimer = undefined
    await refreshWaitingOrders()
    ensureOrderPolling()
  }, 3000)
}
const latestVerificationCode = (order: SMSOrder) => [...(order.messages || [])].reverse().find(message => message.verification_code)?.verification_code || ''
const formatSMSMessageTime = (value: string) => formatDateTime(value, { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false })

const remainingLabel = (order: SMSOrder) => {
  if (isTerminalOrder(order)) return t('sms.user.ended')
  const seconds = Math.max(0, order.expires_at
    ? Math.floor((new Date(order.expires_at).getTime() - Date.now()) / 1000)
    : (order.remaining_seconds ?? 0))
  if (seconds <= 0) return ['active', 'provider_unknown'].includes(order.status) ? t('sms.user.refundProcessing') : '-'
  const minutes = Math.floor(seconds / 60)
  return `${minutes}:${String(seconds % 60).padStart(2, '0')}`
}

function refreshCountdowns() {
  orders.value = orders.value.map(order => ({ ...order }))
  liveOrders.value = liveOrders.value.map(order => ({ ...order }))
}

function countryName(country: SMSCountryItem) {
  const fallback = locale.value.startsWith('zh') ? country.name_zh || country.name_en || country.iso2 : country.name_en || country.name_zh || country.iso2
  try {
    return new Intl.DisplayNames([String(locale.value || 'en')], { type: 'region' }).of(country.iso2.toUpperCase()) || fallback
  } catch {
    return fallback
  }
}

function flagClass(iso2: string) {
  const code = String(iso2 || '').trim().toLowerCase()
  return /^[a-z]{2}$/.test(code) ? `fi-${code}` : 'fi-un'
}

function displayRegionName(iso2: string) {
  const code = String(iso2 || '').toUpperCase()
  try {
    return new Intl.DisplayNames([String(locale.value || 'en')], { type: 'region' }).of(code) || code
  } catch {
    return code
  }
}

function formatPrice(value: number) {
  return `${Number(value || 0).toFixed(4).replace(/0+$/, '').replace(/\.$/, '')}`
}

function operatorStockLabel(item: SMSOperatorItem) {
  if (item.stock == null) return ''
  if (item.source_type === 'donor' && item.stock >= 9999) return ` · ${t('sms.user.stockAbundant')}`
  return ` · ${t('sms.user.stock')} ${item.stock}`
}

function serviceIcon(code: string, name = '') {
  return services.value.find(item => item.code === code)?.icon || serviceLogo(code, name)
}

function serviceLogo(code: string, name = '') {
  const key = `${code} ${name}`.toLowerCase()
  const logos: Array<[RegExp, string]> = [
    [/amazon/, 'logos:amazon'], [/apple|icloud/, 'logos:apple'], [/discord/, 'logos:discord-icon'],
    [/facebook|messenger|meta/, 'logos:facebook'], [/telegram/, 'logos:telegram'], [/whatsapp/, 'logos:whatsapp-icon'],
    [/google|gmail|youtube/, 'logos:google-icon'], [/microsoft|outlook|hotmail|office/, 'logos:microsoft-icon'],
    [/openai|chatgpt/, 'simple-icons:openai'], [/instagram|threads/, 'skill-icons:instagram'],
    [/ebay/, 'logos:ebay'], [/paypal/, 'logos:paypal'], [/tiktok/, 'logos:tiktok-icon'],
    [/twitter|\bx\b/, 'simple-icons:x'], [/linkedin/, 'logos:linkedin-icon'], [/uber/, 'logos:uber'],
    [/airbnb/, 'logos:airbnb-icon'], [/netflix/, 'logos:netflix-icon'], [/spotify/, 'logos:spotify-icon'],
    [/github/, 'logos:github-icon'], [/yahoo/, 'logos:yahoo'], [/snapchat/, 'logos:snapchat-icon'],
    [/steam/, 'logos:steam'], [/twitch/, 'logos:twitch'], [/reddit/, 'logos:reddit-icon'],
    [/wechat|weixin/, 'logos:wechat-icon'], [/line/, 'logos:line'], [/coinbase/, 'logos:coinbase'],
    [/binance/, 'logos:binance'], [/aliexpress/, 'simple-icons:aliexpress'], [/booking/, 'simple-icons:bookingdotcom'],
    [/doordash/, 'simple-icons:doordash'], [/grab/, 'simple-icons:grab'], [/viber/, 'simple-icons:viber'],
  ]
  return logos.find(([pattern]) => pattern.test(key))?.[1] || ''
}

function statusLabel(status: string, reconciliationAction?: string) {
  if (status === 'reconciling' && reconciliationAction === 'purchase') return t('sms.user.statuses.confirmingPurchase')
  const labels: Record<string, string> = {
    pending: t('sms.user.statuses.pending'),
    active: t('sms.user.statuses.waitingSms'),
    reconciling: t('sms.user.statuses.reconciling'),
    provider_unknown: t('sms.user.statuses.providerUnknown'),
    completed: t('sms.user.statuses.completed'),
    cancelled: t('sms.user.statuses.cancelled'),
    failed: t('sms.user.statuses.failed'),
    refunded: t('sms.user.statuses.refunded'),
    expired: t('sms.user.statuses.expired'),
  }
  return labels[status] || t('sms.user.statuses.unknown')
}

function statusClass(status: string) {
  return status === 'completed' || status === 'refunded' ? 'badge-success' : status === 'failed' ? 'badge-danger' : status === 'cancelled' ? 'badge-gray' : 'badge-info'
}

async function loadRecentSuccesses() {
  try {
    const feed = await smsAPI.recentSuccesses()
    recentSuccessItems.value = feed.items || []
  } catch {
    recentSuccessItems.value = []
  }
}

async function hydrateLiveOrders() {
  try {
    const page = await smsAPI.orders({ page: 1, page_size: 20 })
    liveOrders.value = page.items
      .filter(order => isOrderWaiting(order))
      .slice(0, 10)
    if (liveOrders.value.length) ensureOrderPolling()
  } catch {
    // Catalog loading must not fail just because recent-order hydration fails.
  }
}

async function loadAll() {
  loading.value = true
  try {
    const [providerItems, smsSettings] = await Promise.all([
      smsAPI.providers(),
      smsAPI.settings().catch(() => ({ batch_purchase_limit: 5 })),
      hydrateLiveOrders(),
    ])
    providers.value = providerItems
    batchPurchaseLimit.value = normalizeBatchPurchaseLimit(smsSettings.batch_purchase_limit)
    purchaseQuantity.value = Math.min(batchPurchaseLimit.value, Math.max(1, Number(purchaseQuantity.value) || 1))
    const preferred = providers.value.find(item => item.code === providerCode.value && item.selectable)
      || providers.value.find(item => item.code === '5sim' && item.selectable)
      || providers.value.find(item => item.selectable)
    providerCode.value = preferred?.code || ''
    await loadProviderCatalog()
  } catch (error) {
    appStore.showError(errorMessage(error, t('sms.user.errors.unavailable')))
  } finally {
    loading.value = false
  }
}

async function loadServicePage(reset = false) {
  if (!providerCode.value || servicesLoading.value) return
  if (reset) {
    services.value = []
    servicePagination.page = 1
    servicePagination.hasMore = false
  }
  servicesLoading.value = true
  try {
    const page = await smsAPI.providerServicesPage(providerCode.value, {
      page: servicePagination.page,
      page_size: servicePagination.pageSize,
      keyword: serviceKeyword.value || undefined,
      product_type: productType.value,
      duration_value: productType.value === 'rental' ? durationValue.value : undefined,
      duration_unit: productType.value === 'rental' ? durationUnit.value : undefined,
    })
    services.value = reset ? page.items : [...services.value, ...page.items.filter(item => !services.value.some(existing => existing.code === item.code))]
    servicePagination.total = page.total
    servicePagination.hasMore = page.has_more
    if (page.has_more) servicePagination.page += 1
  } finally {
    servicesLoading.value = false
  }
}

async function loadCountryPage(reset = false) {
  if (!providerCode.value || !serviceCode.value || countriesLoading.value) return
  if (reset) {
    countries.value = []
    operators.value = []
    quotes.value = []
    countryPagination.page = 1
    countryPagination.hasMore = false
    countryCode.value = ''
    operatorCode.value = 'any'
  }
  countriesLoading.value = true
  try {
    const page = await smsAPI.serviceCountriesPage(providerCode.value, serviceCode.value, {
      page: countryPagination.page,
      page_size: countryPagination.pageSize,
      keyword: countryKeyword.value || undefined,
      sort: currentProvider.value?.capabilities.supports_conversion_stats ? countrySortMode.value : 'name',
      product_type: productType.value,
      duration_value: productType.value === 'rental' ? durationValue.value : undefined,
      duration_unit: productType.value === 'rental' ? durationUnit.value : undefined,
    })
    countries.value = reset ? page.items : [...countries.value, ...page.items.filter(item => !countries.value.some(existing => existing.iso2 === item.iso2))]
    countryPagination.total = page.total
    countryPagination.hasMore = page.has_more
    if (page.has_more) countryPagination.page += 1
    if (reset) countryCode.value = countries.value.find(item => item.available !== false)?.iso2 || ''
    await loadOperators()
  } finally {
    countriesLoading.value = false
  }
}

async function loadProviderCatalog() {
  services.value = []
  countries.value = []
  operators.value = []
  operatorCode.value = 'any'
  voiceMode.value = 0
  serviceKeyword.value = ''
  countryKeyword.value = ''
  countrySortMode.value = currentProvider.value?.capabilities.supports_conversion_stats ? 'recommended' : 'name'
  serviceCode.value = ''
  countryCode.value = ''
  quotes.value = []
  if (!providerCode.value) return
  await loadServicePage(true)
  serviceCode.value = services.value[0]?.code || ''
  if (serviceCode.value) await loadCountryPage(true)
}

async function switchProvider(code: string) {
  const provider = providers.value.find(item => item.code === code)
  if (!provider || !isProviderSelectable(provider) || code === providerCode.value) return
  providerCode.value = code
  additionalRentalServiceCodes.value = []
  productType.value = provider.capabilities.supports_temporary ? 'temporary' : 'rental'
  durationUnit.value = provider.code === 'smspva' ? 'week' : 'hour'
  voiceMode.value = voiceModeOptions.value[0]?.value ?? 0
  activeTab.value = productType.value
  loading.value = true
  try {
    await loadProviderCatalog()
  } catch (error) {
    appStore.showError(errorMessage(error, t('sms.user.errors.unavailable')))
  } finally {
    loading.value = false
  }
}

async function switchProductType(type: 'temporary' | 'rental') {
  if (type === 'rental' && !currentProvider.value?.capabilities.supports_rental) return
  productType.value = type
  additionalRentalServiceCodes.value = []
  activeTab.value = type
  if (type === 'rental' && providerCode.value === 'smspva' && !['week', 'month'].includes(durationUnit.value)) {
    durationUnit.value = 'week'
  }
  quotes.value = []
  await loadProviderCatalog()
  if (serviceCode.value && countryCode.value) await loadQuotes()
}

async function loadQuotes() {
  if (!serviceCode.value || !countryCode.value) return
  quoting.value = true
  try {
    quotes.value = await smsAPI.quotes({
      provider: providerCode.value,
      service: serviceCode.value,
      services: productType.value === 'rental' && currentProvider.value?.capabilities.supports_rental_multi_service
        ? [serviceCode.value, ...additionalRentalServiceCodes.value].join(',')
        : undefined,
      country: countryCode.value,
      product_type: productType.value,
      operator: operatorCode.value || 'any',
      voice_mode: voiceMode.value,
      duration_value: productType.value === 'rental' ? durationValue.value : undefined,
      duration_unit: productType.value === 'rental' ? durationUnit.value : undefined,
    })
  } catch (error) {
    quotes.value = []
    appStore.showError(errorMessage(error, t('sms.user.errors.quote')))
  } finally {
    quoting.value = false
  }
}

async function loadOrders() {
  ordersLoading.value = true
  try {
    const result = await smsAPI.orders({ page: orderPagination.page, page_size: orderPagination.pageSize, ...orderFilters }) as SMSOrderPage | SMSOrder[]
    if (Array.isArray(result)) {
      orders.value = result
      orderPagination.total = result.length
    } else {
      orders.value = result.items
      orderPagination.total = result.total
      orderPagination.page = result.page
      orderPagination.pageSize = result.page_size
    }
    if (pollingCandidates().length > 0) ensureOrderPolling()
    else stopOrderPolling()
  } catch (error) {
    appStore.showError(errorMessage(error, t('sms.user.errors.orders')))
  } finally {
    ordersLoading.value = false
  }
}

function applyOrderFilters() { Object.assign(orderFilters, orderDraft); orderPagination.page = 1; void loadOrders() }
function resetOrderFilters() { Object.assign(orderDraft, { keyword: '', status: '' }); Object.assign(orderFilters, orderDraft); orderPagination.page = 1; void loadOrders() }
function changeOrderPage(page: number) { orderPagination.page = page; void loadOrders() }
function changeOrderPageSize(pageSize: number) { orderPagination.pageSize = pageSize; orderPagination.page = 1; void loadOrders() }

async function purchase(quote: SMSQuote) {
  if (!isPurchaseQuantityValid.value) {
    appStore.showError(purchaseQuantityError.value)
    return
  }
  purchasing.value = true
  try {
    const key = `sms-${Date.now()}-${Math.random().toString(36).slice(2)}`
    const quantity = Number(purchaseQuantity.value)
    const item = { channel_code: quote.channel_code, service_code: serviceCode.value, service_codes: productType.value === 'rental' && currentProvider.value?.capabilities.supports_rental_multi_service ? [serviceCode.value, ...additionalRentalServiceCodes.value] : undefined, country_code: countryCode.value, product_type: productType.value, operator_code: operatorCode.value || 'any', voice_mode: voiceMode.value, duration_value: productType.value === 'rental' ? durationValue.value : undefined, duration_unit: productType.value === 'rental' ? durationUnit.value : undefined, quote_id: quote.quote_id, expected_price: quote.sale_price }
    if (quantity > 1) {
      const result = await smsAPI.purchaseBatch({ items: Array.from({ length: quantity }, () => item) }, key)
      liveOrders.value = [...result.items, ...liveOrders.value.filter(existing => !result.items.some(item => item.id === existing.id))]
      if (result.partial_error) appStore.showError(result.partial_error)
    } else {
      const order = await smsAPI.purchase(item, key)
      liveOrders.value = [order, ...liveOrders.value.filter(existing => existing.id !== order.id)]
    }
    ensureOrderPolling()
    quotes.value = []
    await authStore.refreshUser().catch(() => undefined)
    const confirming = liveOrders.value.some(order => order.status === 'reconciling' && order.reconciliation_action === 'purchase')
    appStore.showSuccess(confirming ? t('sms.user.purchaseConfirming') : t('sms.user.purchaseSuccess'))
  } catch (error) {
    await authStore.refreshUser().catch(() => undefined)
    void loadOrders()
    const candidate = error as { reason?: string; code?: string | number }
    const reason = candidate?.reason || (typeof candidate?.code === 'string' ? candidate.code : '')
    if (reason === 'ORDER_RECONCILING') {
      await hydrateLiveOrders()
      appStore.showSuccess(t('sms.user.purchaseConfirming'))
      ensureOrderPolling()
    } else {
      appStore.showError(errorMessage(error, t('sms.user.errors.purchase')))
    }
  } finally {
    purchasing.value = false
  }
}

function normalizeBatchPurchaseLimit(value: unknown) {
  const parsed = Number(value)
  return Number.isInteger(parsed) ? Math.min(50, Math.max(1, parsed)) : 5
}

function askConfirm(options: { title: string; message: string; confirmText?: string; cancelText?: string; danger?: boolean; action: () => Promise<void> }) {
  confirmState.title = options.title
  confirmState.message = options.message
  confirmState.confirmText = options.confirmText || t('common.confirm')
  confirmState.cancelText = options.cancelText || t('common.cancel')
  confirmState.danger = options.danger ?? false
  confirmState.action = options.action
  confirmState.show = true
}

function closeConfirm() {
  if (confirmingAction.value) return
  confirmState.show = false
  confirmState.action = null
}

async function runConfirmedAction() {
  if (!confirmState.action || confirmingAction.value) return
  confirmingAction.value = true
  try {
    await confirmState.action()
    confirmState.show = false
    confirmState.action = null
  } finally {
    confirmingAction.value = false
  }
}

function cancel(order: SMSOrder) {
  if (!canCancelOrder(order)) return
  askConfirm({
    title: t('sms.user.cancelConfirmTitle'),
    message: t('sms.user.cancelConfirmMessage'),
    confirmText: t('sms.user.cancelConfirmAction'),
    cancelText: t('common.back'),
    danger: true,
    action: async () => {
      try {
        await smsAPI.cancel(order.id)
        await refreshOrder(order.id)
        await authStore.refreshUser().catch(() => undefined)
      } catch (error) {
        await refreshOrder(order.id)
        await authStore.refreshUser().catch(() => undefined)
        appStore.showError(errorMessage(error, t('sms.user.errors.cancel')))
      }
    },
  })
}

async function copyText(value?: string) {
  if (!value) return
  try {
    await navigator.clipboard.writeText(value)
    appStore.showSuccess(t('sms.user.copySuccess'))
  } catch {
    appStore.showError(t('sms.user.copyFailed'))
  }
}

async function resend(id: string) {
  try { await smsAPI.resend(id); await refreshOrder(id); appStore.showSuccess(t('sms.user.resendSuccess')) }
  catch (error) { appStore.showError(errorMessage(error, t('sms.user.resendFailed'))) }
}



async function selectService(code: string) {
  if (code === serviceCode.value) return
  serviceCode.value = code
  additionalRentalServiceCodes.value = []
  countryKeyword.value = ''
  quotes.value = []
  await loadCountryPage(true)
}

async function selectCountry(code: string) {
  const country = countries.value.find(item => item.iso2 === code)
  if (!country || country.available === false) return
  countryCode.value = code
  quotes.value = []
  await loadOperators()
  await loadQuotes()
}

async function loadOperators() {
  operators.value = []
  operatorCode.value = 'any'
  if (!providerCode.value || !serviceCode.value || !countryCode.value) return
  if (!currentProvider.value?.capabilities.supports_operator_selection) return
  try {
    operators.value = await smsAPI.operators(providerCode.value, serviceCode.value, countryCode.value, {
      voice_mode: voiceMode.value,
      product_type: productType.value,
      duration_value: productType.value === 'rental' ? durationValue.value : undefined,
      duration_unit: productType.value === 'rental' ? durationUnit.value : undefined,
    })
    const selectedCountry = countries.value.find(item => item.iso2 === countryCode.value)
    const recommended = currentProvider.value?.capabilities.supports_conversion_stats
      ? selectedCountry?.recommended_operator
      : ''
    if (recommended && operators.value.some(item => item.code === recommended && item.available !== false)) {
      operatorCode.value = recommended
    } else {
      operatorCode.value = 'any'
    }
  } catch {
    operators.value = []
    operatorCode.value = 'any'
  }
}

async function reloadRentalCatalog() {
  if (productType.value !== 'rental') return
  additionalRentalServiceCodes.value = []
  quotes.value = []
  await loadProviderCatalog()
  if (serviceCode.value && countryCode.value) await loadQuotes()
}

async function changeVoiceMode() {
  quotes.value = []
  await loadOperators()
  if (countryCode.value) await loadQuotes()
}



async function confirmSelection() {
  if (!serviceCode.value || !countryCode.value) return
  if (!bestQuote.value) {
    await loadQuotes()
    return
  }
  await purchase(bestQuote.value)
}

watch(serviceKeyword, () => {
  if (serviceSearchTimer) window.clearTimeout(serviceSearchTimer)
  serviceSearchTimer = window.setTimeout(() => { void loadServicePage(true) }, 250)
})

watch(countryKeyword, () => {
  if (countrySearchTimer) window.clearTimeout(countrySearchTimer)
  countrySearchTimer = window.setTimeout(() => { if (serviceCode.value) void loadCountryPage(true) }, 250)
})

watch(countrySortMode, () => {
  if (serviceCode.value) void loadCountryPage(true)
})

function refundLabel(status: string) {
  const labels: Record<string, string> = { approved: t('sms.user.refunds.approved'), rejected: t('sms.user.refunds.rejected'), pending: t('sms.user.refunds.pending') }
  return labels[status] || status
}

function canRestoreRental(order: SMSOrder) {
  return order.product_type === 'rental' && order.capabilities?.supports_rental_restore === true && ['expired', 'cancelled', 'refunded', 'failed', 'completed'].includes(order.status)
}

async function openRentalServiceModal(order: SMSOrder) {
  rentalServiceState.orderId = order.id
  rentalServiceState.rentDays = 7
  rentalServiceState.selectedService = ''
  rentalServiceState.options = []
  rentalServiceState.show = true
  await loadRentalServiceOptions()
}

function closeRentalServiceModal() {
  if (rentalServiceState.loading) return
  rentalServiceState.show = false
  rentalServiceState.orderId = ''
  rentalServiceState.selectedService = ''
  rentalServiceState.options = []
}

async function loadRentalServiceOptions() {
  const days = Number(rentalServiceState.rentDays)
  if (!rentalServiceState.orderId || !Number.isInteger(days) || days <= 0 || days > 366) {
    appStore.showError(t('sms.user.invalidRentDays'))
    return
  }
  rentalServiceState.loading = true
  try {
    rentalServiceState.options = await smsAPI.rentalServiceOptions(rentalServiceState.orderId, days)
    if (!rentalServiceState.options.some(item => item.code === rentalServiceState.selectedService)) {
      rentalServiceState.selectedService = ''
    }
  } catch (error) {
    appStore.showError(errorMessage(error, t('sms.user.errors.rentalServices')))
  } finally {
    rentalServiceState.loading = false
  }
}

async function quoteRentalServiceAddition() {
  const id = rentalServiceState.orderId
  const serviceCode = rentalServiceState.selectedService
  const rentDays = Number(rentalServiceState.rentDays)
  if (!id || !serviceCode || !Number.isInteger(rentDays) || rentDays <= 0) return
  rentalServiceState.loading = true
  try {
    const quote = await smsAPI.rentalServiceQuote(id, { service_code: serviceCode, rent_days: rentDays })
    rentalServiceState.show = false
    askConfirm({
      title: t('sms.user.addServiceConfirmTitle'),
      message: t('sms.user.addServiceConfirmMessage', { service: serviceCode, days: rentDays, price: formatPrice(quote.sale_price) }),
      confirmText: t('sms.user.addService'),
      action: async () => {
        try {
          await smsAPI.addRentalService(id, { quote_id: quote.quote_id, expected_price: quote.sale_price }, `sms-add-service-${id}-${Date.now()}`)
          await refreshOrder(id)
          await authStore.refreshUser().catch(() => undefined)
          appStore.showSuccess(t('sms.user.addServiceSuccess'))
        } catch (error) {
          appStore.showError(errorMessage(error, t('sms.user.errors.addService')))
        }
      },
    })
  } catch (error) {
    appStore.showError(errorMessage(error, t('sms.user.errors.rentalServiceQuote')))
  } finally {
    rentalServiceState.loading = false
  }
}

async function restoreRental(order: SMSOrder) {
  try {
    const quote = await smsAPI.rentalRestoreQuote(order.id)
    askConfirm({
      title: t('sms.user.restoreRentalTitle'),
      message: t('sms.user.restoreRentalConfirmMessage', {
        service: serviceLabel(quote.service_code),
        days: quote.duration_days,
        price: formatPrice(quote.sale_price),
      }),
      confirmText: t('sms.user.restoreRental'),
      action: async () => {
        try {
          const restored = await smsAPI.restoreRental(order.id, { quote_id: quote.quote_id, expected_price: quote.sale_price }, `sms-restore-${order.id}-${Date.now()}`)
          await loadOrders()
          await authStore.refreshUser().catch(() => undefined)
          appStore.showSuccess(t('sms.user.restoreRentalSuccess'))
          if (restored.id) await refreshOrder(restored.id)
        } catch (error) {
          appStore.showError(errorMessage(error, t('sms.user.errors.restoreRental')))
        }
      },
    })
  } catch (error) {
    appStore.showError(errorMessage(error, t('sms.user.errors.restoreRentalQuote')))
  }
}

async function extend(id: string) {
  try {
    const constraints = await smsAPI.rentalConstraints(id)
    if (!constraints.can_extend) {
      appStore.showError(t('sms.user.rentalCannotExtend'))
      return
    }
    const unit = (window.prompt(t('sms.user.extendUnitPrompt'), 'week') || '').trim().toLowerCase()
    if (!['day', 'week', 'month'].includes(unit)) {
      appStore.showError(t('sms.user.extendUnitInvalid'))
      return
    }
    const raw = window.prompt(t('sms.user.extendValuePrompt', { unit }), '1')
    const value = Number(raw)
    if (!Number.isInteger(value) || value <= 0) return
    if (unit === 'day' && constraints.can_prolong_max && value > constraints.can_prolong_max) {
      appStore.showError(t('sms.user.extendMaxExceeded', { max: constraints.can_prolong_max }))
      return
    }
    await smsAPI.extendRental(id, { duration_value: value, duration_unit: unit }, `sms-renew-${id}-${Date.now()}`)
    await loadOrders()
  } catch (error) {
    appStore.showError(errorMessage(error, t('sms.user.errors.extend')))
  }
}

function replaceOrder(updated: SMSOrder) {
  const orderIndex = orders.value.findIndex(item => item.id === updated.id)
  if (orderIndex >= 0) orders.value[orderIndex] = updated
  const liveIndex = liveOrders.value.findIndex(item => item.id === updated.id)
  if (liveIndex >= 0) liveOrders.value[liveIndex] = updated
  if (pollingCandidates().length === 0) stopOrderPolling()
}

async function refreshOrder(id: string) {
  refreshingId.value = id
  try {
    const updated = await smsAPI.order(id)
    replaceOrder(updated)
  } finally {
    refreshingId.value = ''
  }
}

async function refreshWaitingOrders() {
  const candidates = pollingCandidates()
  if (!candidates.length) {
    stopOrderPolling()
    return
  }
  const updates = await Promise.allSettled(candidates.map(order => smsAPI.order(order.id)))
  updates.forEach(result => {
    if (result.status === 'fulfilled' && result.value) replaceOrder(result.value)
  })
}

onMounted(() => {
  void loadRecentSuccesses()
  loadAll()
  countdownTimer = window.setInterval(refreshCountdowns, 1000)
})

onBeforeUnmount(() => {
  stopOrderPolling()
  if (countdownTimer) window.clearInterval(countdownTimer)
  if (serviceSearchTimer) window.clearTimeout(serviceSearchTimer)
  if (countrySearchTimer) window.clearTimeout(countrySearchTimer)
})
</script>

<style scoped>
.sms-success-track {
  animation: sms-success-marquee 38s linear infinite;
}
.sms-success-track:hover {
  animation-play-state: paused;
}
@keyframes sms-success-marquee {
  from { transform: translateX(0); }
  to { transform: translateX(-50%); }
}
@media (prefers-reduced-motion: reduce) {
  .sms-success-track { animation: none; }
}
</style>
