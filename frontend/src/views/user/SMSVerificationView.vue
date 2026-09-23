<template>
  <AppLayout>
    <main class="sms-page">
      <div class="sms-shell">
        <header class="sms-page-header">
          <div>
            <p class="sms-eyebrow">MODURELAY</p>
            <h1>{{ t('nav.smsService') }}</h1>
            <p>{{ t('sms.user.description') }}</p>
          </div>
          <button
            type="button"
            class="sms-refresh-button"
            :disabled="loading"
            :title="t('common.refresh')"
            :aria-label="t('common.refresh')"
            @click="loadAll"
          >
            <Icon name="refresh" size="sm" :class="{ 'sms-spin': loading }" aria-hidden="true" />
            <span>{{ t('sms.user.refreshData') }}</span>
          </button>
        </header>

        <section v-if="recentSuccessItems.length" class="sms-success-strip" aria-label="recent success">
          <span class="sms-success-strip__label">
            <Icon name="checkCircle" size="sm" aria-hidden="true" />
            {{ t('sms.user.recentSuccess') }}
          </span>
          <div class="sms-success-strip__viewport">
            <div class="sms-success-track">
              <span
                v-for="(item, index) in recentSuccessLoop"
                :key="String(index) + '-' + item.username + '-' + item.phone"
                class="sms-success-item"
              >
                <strong>{{ item.username }}</strong>
                <span :class="flagClass(item.country_code)" class="fi fis" aria-hidden="true"></span>
                <span>{{ displayRegionName(item.country_code) }}</span>
                <code>{{ item.phone }}</code>
              </span>
            </div>
          </div>
        </section>

        <nav class="sms-tabs" role="tablist" :aria-label="t('nav.smsService')">
          <button
            v-for="tab in tabs"
            :key="tab.value"
            type="button"
            role="tab"
            :disabled="tab.disabled"
            :aria-selected="productType === tab.value && activeTab !== 'orders'"
            :class="{ 'is-active': productType === tab.value && activeTab !== 'orders' }"
            @click="switchProductType(tab.value)"
          >
            {{ tab.label }}
          </button>
          <button
            type="button"
            role="tab"
            :aria-selected="activeTab === 'orders'"
            :class="{ 'is-active': activeTab === 'orders' }"
            @click="activeTab = 'orders'; loadOrders()"
          >
            {{ t('sms.user.orders') }}
          </button>
        </nav>

        <template v-if="activeTab !== 'orders'">
          <section class="sms-workspace">
            <div class="sms-workspace__left">
              <section class="sms-step sms-step--channels">
                <header class="sms-step__header">
                  <span class="sms-step__number">1</span>
                  <div>
                    <div class="sms-step__title-line">
                      <h2>{{ t('sms.user.stepChannelTitle') }}</h2>
                      <span>{{ t('sms.user.publicChannels') }}</span>
                    </div>
                    <p>{{ t('sms.user.stepChannelDescription') }}</p>
                  </div>
                </header>

                <div class="sms-provider-grid">
                  <button
                    v-for="(provider, index) in providers"
                    :key="provider.code"
                    type="button"
                    class="sms-provider-card"
                    :class="[
                      providerCode === provider.code ? 'is-selected border-primary-500' : '',
                      !isProviderSelectable(provider) ? 'is-disabled' : '',
                    ]"
                    :disabled="!isProviderSelectable(provider)"
                    :title="providerDisabledReason(provider)"
                    @click="switchProvider(provider.code)"
                  >
                    <span class="sms-provider-card__icon" aria-hidden="true">
                      <span></span><span></span><span></span>
                    </span>
                    <span class="sms-provider-card__copy">
                      <strong>{{ t('sms.user.channel') }}{{ index + 1 }}</strong>
                      <small v-if="!isProviderSelectable(provider)">{{ providerDisabledReason(provider) }}</small>
                      <small v-else>{{ providerCode === provider.code ? t('sms.user.channelReadySelected') : t('sms.user.channelReady') }}</small>
                    </span>
                    <span v-if="provider.beta" class="sms-provider-card__beta">BETA</span>
                    <span v-else class="sms-provider-card__state"></span>
                  </button>
                </div>
              </section>

              <section class="sms-step sms-step--service">
                <header class="sms-step__header">
                  <span class="sms-step__number">2</span>
                  <div>
                    <h2>{{ t('sms.user.stepServiceTitle') }}</h2>
                    <p>{{ t('sms.user.stepServiceDescription') }}</p>
                  </div>
                </header>

                <label class="sms-search-field">
                  <Icon name="search" size="sm" aria-hidden="true" />
                  <input v-model.trim="serviceKeyword" :placeholder="t('sms.user.serviceSearch')" />
                </label>

                <div class="sms-option-list sms-option-list--service">
                  <button
                    v-for="option in filteredServiceOptions"
                    :key="String(option.value)"
                    type="button"
                    class="sms-option-card"
                    :class="{ 'is-selected': serviceCode === option.value }"
                    @click="selectService(String(option.value))"
                  >
                    <span class="sms-option-card__main">
                      <SMSServiceLogo :icon="option.logo" :label="option.label" class="sms-service-logo" />
                      <span class="sms-option-card__copy">
                        <strong>{{ option.label }}</strong>
                        <small v-if="option.stock != null && option.stock > 0">
                          {{ t('sms.user.numbersAvailable', { count: option.stock.toLocaleString() }) }}
                        </small>
                      </span>
                    </span>
                    <span class="sms-option-card__meta">
                      <strong v-if="option.startingPrice != null && option.startingPrice > 0">
                        {{ t('sms.user.startingAt', { price: formatPrice(option.startingPrice) }) }}
                      </strong>
                      <small>{{ option.value }}</small>
                    </span>
                    <span v-if="serviceCode === option.value" class="sms-selected-mark"><Icon name="check" size="xs" /></span>
                  </button>

                  <div v-if="servicesLoading && !services.length" class="sms-list-state">{{ t('sms.user.loading') }}</div>
                  <div v-else-if="!filteredServiceOptions.length" class="sms-list-state">{{ t('sms.user.noServiceMatch') }}</div>
                  <button
                    v-if="servicePagination.hasMore"
                    type="button"
                    class="sms-load-more"
                    :disabled="servicesLoading"
                    @click="loadServicePage(false)"
                  >
                    {{ servicesLoading ? t('sms.user.loading') : t('sms.user.loadMore') }}
                  </button>
                </div>
              </section>
            </div>

            <section class="sms-step sms-step--country">
              <header class="sms-step__header">
                <span class="sms-step__number">3</span>
                <div>
                  <h2>{{ t('sms.user.stepCountryTitle') }}</h2>
                  <p>{{ t('sms.user.stepCountryDescription') }}</p>
                </div>
              </header>

              <div class="sms-country-toolbar">
                <label class="sms-search-field">
                  <Icon name="search" size="sm" aria-hidden="true" />
                  <input v-model.trim="countryKeyword" :placeholder="t('sms.user.countrySearch')" :disabled="!serviceCode" />
                </label>
                <select
                  v-if="currentProvider?.capabilities.supports_conversion_stats"
                  v-model="countrySortMode"
                  class="sms-native-select"
                >
                  <option v-for="item in countrySortOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
                </select>
              </div>

              <div class="sms-option-list sms-option-list--country">
                <button
                  v-for="option in filteredCountryOptions"
                  :key="String(option.value)"
                  type="button"
                  class="sms-country-card"
                  :class="{ 'is-selected': countryCode === option.value }"
                  :disabled="option.available === false"
                  @click="selectCountry(String(option.value))"
                >
                  <span class="sms-country-card__flag" :class="flagClass(String(option.value)) + ' fi fis'" aria-hidden="true"></span>
                  <span class="sms-country-card__copy">
                    <span class="sms-country-card__name">
                      <strong>{{ option.label }}</strong>
                      <small>{{ option.value }}</small>
                    </span>
                    <span class="sms-country-card__sub">
                      <template v-if="option.stock != null">{{ t('sms.user.stock') }} {{ option.stock.toLocaleString() }}</template>
                      <template v-if="currentProvider?.capabilities.supports_conversion_stats && option.conversionRate != null && option.conversionRate > 0">
                        · {{ t('sms.user.providerDeliveryRateValue', { rate: option.conversionRate.toFixed(2) }) }}
                      </template>
                    </span>
                    <span v-if="option.platform30dSuccessRate != null" class="sms-country-card__rate">
                      {{ t('sms.user.platform30dRateValue', { rate: option.platform30dSuccessRate.toFixed(2), count: option.platform30dSampleSize }) }}
                    </span>
                  </span>
                  <span class="sms-country-card__meta">
                    <strong v-if="option.recommendedStartingPrice != null && option.recommendedStartingPrice > 0">
                      {{ t('sms.user.startingAt', { price: formatPrice(option.recommendedStartingPrice) }) }}
                    </strong>
                    <strong v-else-if="option.startingPrice != null && option.startingPrice > 0">
                      {{ t('sms.user.startingAt', { price: formatPrice(option.startingPrice) }) }}
                    </strong>
                    <small v-if="option.recommendedOperatorStock != null && option.recommendedOperatorStock > 0">
                      {{ t('sms.user.recommendedStock', { count: option.recommendedOperatorStock.toLocaleString() }) }}
                    </small>
                  </span>
                  <span v-if="countryCode === option.value" class="sms-selected-mark"><Icon name="check" size="xs" /></span>
                </button>

                <div v-if="countriesLoading && !countries.length" class="sms-list-state">{{ t('sms.user.loading') }}</div>
                <div v-else-if="serviceCode && !filteredCountryOptions.length" class="sms-list-state">{{ t('sms.user.noCountryMatch') }}</div>
                <button
                  v-if="countryPagination.hasMore"
                  type="button"
                  class="sms-load-more"
                  :disabled="countriesLoading"
                  @click="loadCountryPage(false)"
                >
                  {{ countriesLoading ? t('sms.user.loading') : t('sms.user.loadMore') }}
                </button>
              </div>

              <p v-if="!serviceCode" class="sms-inline-hint">{{ t('sms.user.selectService') }}</p>
            </section>

            <section class="sms-step sms-step--confirm">
              <header class="sms-step__header">
                <span class="sms-step__number">4</span>
                <div>
                  <h2>{{ t('sms.user.stepConfirmTitle') }}</h2>
                  <p>{{ productType === 'rental' ? t('sms.user.stepConfirmRentalDescription') : t('sms.user.stepConfirmDescription') }}</p>
                </div>
              </header>

              <div class="sms-selection-summary">
                <div>
                  <span>{{ t('sms.user.selectedService') }}</span>
                  <strong class="sms-selection-summary__entity">
                    <SMSServiceLogo :icon="selectedServiceLogo" :label="serviceLabel(serviceCode)" class="sms-service-logo sms-service-logo--small" />
                    <span>{{ serviceLabel(serviceCode) || '-' }}</span>
                  </strong>
                </div>
                <div>
                  <span>{{ t('sms.user.selectedCountry') }}</span>
                  <strong class="sms-selection-summary__entity">
                    <span v-if="countryCode" :class="flagClass(countryCode)" class="fi fis" aria-hidden="true"></span>
                    <span>{{ countryLabel(countryCode) || '-' }}</span>
                  </strong>
                </div>
                <div v-if="selectedCountryStats">
                  <span>{{ t('sms.user.platform30dSuccessRate') }}</span>
                  <strong class="sms-success-value">
                    <template v-if="selectedCountryStats.platform_30d_success_rate != null">
                      {{ selectedCountryStats.platform_30d_success_rate.toFixed(2) }}%
                    </template>
                    <template v-else>{{ t('sms.user.platform30dInsufficientShort', { count: selectedCountryStats.platform_30d_sample_size || 0 }) }}</template>
                  </strong>
                </div>
              </div>

              <div v-if="productType === 'rental' && currentProvider?.capabilities.supports_rental" class="sms-form-grid">
                <label>
                  <span class="sms-field-label">{{ t('sms.user.duration') }}</span>
                  <input v-model.number="durationValue" class="sms-field" type="number" min="1" @change="reloadRentalCatalog" />
                </label>
                <Select v-model="durationUnit" :label="t('sms.user.unit')" :options="durationUnitOptions" @update:model-value="reloadRentalCatalog" />
              </div>

              <div
                v-if="productType === 'rental' && currentProvider?.capabilities.supports_rental_constraints && currentProvider?.capabilities.supports_rental_multi_service"
                class="sms-extra-services"
              >
                <div class="sms-extra-services__heading">
                  <div>
                    <strong>{{ t('sms.user.multiServiceRental') }}</strong>
                    <small>{{ t('sms.user.multiServiceRentalHint') }}</small>
                  </div>
                  <span v-if="additionalRentalServiceCodes.length">{{ t('sms.user.additionalServiceCount', { count: additionalRentalServiceCodes.length }) }}</span>
                </div>
                <div class="sms-extra-services__list">
                  <label v-for="item in services.filter(item => item.code !== serviceCode)" :key="item.code">
                    <input v-model="additionalRentalServiceCodes" type="checkbox" :value="item.code" @change="handleAdditionalRentalServiceChange" />
                    <SMSServiceLogo :icon="serviceLogo(item.code, item.name) || item.icon || ''" :label="item.name || item.code" class="sms-service-logo sms-service-logo--tiny" />
                    <span>{{ item.name || item.code }}</span>
                  </label>
                </div>
              </div>

              <label v-if="currentProvider?.capabilities.supports_voice" class="sms-form-field">
                <span class="sms-field-label">{{ t('sms.user.verificationType') }}</span>
                <select v-model.number="voiceMode" class="sms-field" @change="changeVoiceMode">
                  <option v-for="item in voiceModeOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
                </select>
              </label>

              <label v-if="currentProvider?.capabilities.supports_operator_selection" class="sms-form-field">
                <span class="sms-field-label">{{ t('sms.user.operator') }}</span>
                <select v-model="operatorCode" class="sms-field" @change="quotes = []; loadQuotes()">
                  <option value="any">{{ t('sms.user.autoOperator') }}</option>
                  <option
                    v-for="item in operators.filter(op => op.code !== 'any')"
                    :key="item.code"
                    :value="item.code"
                    :disabled="item.available === false"
                  >
                    {{ item.name }}{{ operatorStockLabel(item) }}
                  </option>
                </select>
                <small v-if="currentProvider?.capabilities.supports_conversion_stats && operatorCode !== 'any'" class="sms-field-help sms-field-help--success">
                  {{ t('sms.user.recommendedOperatorSelected') }}
                </small>
              </label>

              <label class="sms-form-field">
                <span class="sms-field-label">{{ t('sms.user.quantity') }}</span>
                <div class="sms-quantity">
                  <button type="button" :disabled="purchaseQuantity <= 1" @click="purchaseQuantity = Math.max(1, Number(purchaseQuantity) - 1)">−</button>
                  <input
                    v-model.number="purchaseQuantity"
                    type="number"
                    min="1"
                    :max="batchPurchaseLimit"
                    :aria-invalid="purchaseQuantityError ? 'true' : undefined"
                    :aria-describedby="purchaseQuantityError ? 'sms-quantity-error' : undefined"
                  />
                  <button type="button" :disabled="purchaseQuantity >= batchPurchaseLimit" @click="purchaseQuantity = Math.min(batchPurchaseLimit, Number(purchaseQuantity) + 1)">+</button>
                </div>
                <small class="sms-field-help">{{ t('sms.user.batchPurchaseHint', { max: batchPurchaseLimit }) }}</small>
                <small v-if="purchaseQuantityError" id="sms-quantity-error" class="sms-field-error">{{ purchaseQuantityError }}</small>
              </label>

              <div v-if="bestQuote" class="sms-price-summary">
                <div>
                  <span>{{ t('sms.user.price') }}</span>
                  <strong>{{ (bestQuote.sale_price * Math.max(1, Number(purchaseQuantity) || 1)).toFixed(4) }}</strong>
                </div>
                <small>{{ formatPrice(bestQuote.sale_price) }} × {{ Math.max(1, Number(purchaseQuantity) || 1) }} · {{ t('sms.user.currency') }}</small>
              </div>

              <button
                type="button"
                class="sms-primary-action"
                :disabled="quoting || purchasing || !serviceCode || !countryCode || !isPurchaseQuantityValid"
                @click="confirmSelection"
              >
                <span v-if="quoting || purchasing" class="sms-action-spinner"></span>
                <Icon v-else name="bolt" size="sm" aria-hidden="true" />
                <span>
                  {{ purchasing
                    ? t('sms.user.processing')
                    : bestQuote
                      ? (purchaseQuantity > 1 ? t('sms.user.batchPurchase') : t('sms.user.purchase'))
                      : (quoting ? t('sms.user.quoting') : t('sms.user.getQuote')) }}
                </span>
                <Icon v-if="!quoting && !purchasing" name="chevronRight" size="sm" aria-hidden="true" />
              </button>

              <p v-if="productType === 'temporary' && bestQuote?.capabilities?.supports_cancel && bestQuote?.capabilities?.supports_refund" class="sms-refund-note">
                {{ t('sms.user.capabilities.cancelRefund') }}
              </p>
              <p v-else-if="productType === 'temporary' && currentProvider?.capabilities.supports_refund" class="sms-refund-note">
                {{ t('sms.user.refundGuarantee') }}
              </p>

              <div class="sms-delivery-tip">
                <Icon name="infoCircle" size="sm" aria-hidden="true" />
                <div>
                  <strong>{{ t('sms.user.deliveryTipTitle') }}</strong>
                  <p>{{ t('sms.user.deliveryTipBody') }}</p>
                </div>
              </div>
            </section>
          </section>

          <section v-if="quotes.length > 1" class="sms-quote-panel">
            <header>
              <div>
                <p class="sms-section-kicker">{{ t('sms.user.quoteOptionsEyebrow') }}</p>
                <h2>{{ t('sms.user.quoteOptionsTitle') }}</h2>
              </div>
              <span>{{ quotes.length }}</span>
            </header>
            <div class="sms-quote-grid">
              <article v-for="quote in quotes" :key="quote.quote_id" class="sms-quote-card">
                <div>
                  <strong>{{ smsChannelLabel(quote.channel_code) }}</strong>
                  <small>{{ t('sms.user.stock') }} {{ quote.stock }} · {{ t('sms.user.eta') }} {{ quote.estimated_delivery_seconds }}{{ t('sms.user.seconds') }}</small>
                  <small v-if="quote.success_rate != null">{{ t('sms.user.platform30dSuccessRate') }} {{ (quote.success_rate * 100).toFixed(2) }}%</small>
                </div>
                <div>
                  <strong>{{ quote.sale_price.toFixed(4) }}</strong>
                  <small>{{ t('sms.user.currency') }}</small>
                </div>
                <button type="button" :disabled="purchasing || !isPurchaseQuantityValid" @click="purchase(quote)">
                  {{ t('sms.user.purchase') }}
                </button>
              </article>
            </div>
          </section>

          <section v-if="liveOrders.length" class="sms-live-orders">
            <header class="sms-live-orders__header">
              <div>
                <span class="sms-live-indicator"></span>
                <div>
                  <h2>{{ t('sms.user.liveOrdersTitle', { count: liveOrders.length }) }}</h2>
                  <p>{{ t('sms.user.liveOrdersDescription') }}</p>
                </div>
              </div>
            </header>

            <div class="sms-live-orders__list">
              <article v-for="order in liveOrders" :key="order.id" class="sms-live-order">
                <div class="sms-live-order__service">
                  <SMSServiceLogo :icon="serviceIcon(order.service_code, serviceLabel(order.service_code))" :label="serviceLabel(order.service_code)" class="sms-service-logo" />
                  <span>
                    <strong>{{ serviceLabel(order.service_code) }}</strong>
                    <small>{{ smsChannelLabel(order.channel_code) }}</small>
                  </span>
                </div>

                <div class="sms-live-order__country">
                  <span :class="flagClass(order.country_code)" class="fi fis" aria-hidden="true"></span>
                  <span>
                    <strong>{{ countryLabel(order.country_code) }}</strong>
                    <small>{{ order.calling_code || '' }}</small>
                  </span>
                </div>

                <div class="sms-live-order__phone">
                  <small>{{ t('sms.user.phone') }}</small>
                  <span>
                    <strong>{{ order.phone_number || '-' }}</strong>
                    <SMSPhoneCopy v-if="order.phone_number" :phone="order.phone_number" :calling-code="order.calling_code" />
                  </span>
                </div>

                <div class="sms-live-order__status">
                  <small>{{ t('sms.user.status') }}</small>
                  <span class="badge" :class="statusClass(order.status)">{{ statusLabel(order.status, order.reconciliation_action) }}</span>
                </div>

                <div class="sms-live-order__expiry">
                  <small>{{ t('sms.user.expiresIn') }}</small>
                  <strong>{{ remainingLabel(order) }}</strong>
                </div>

                <div class="sms-live-order__code">
                  <small>{{ t('sms.user.code') }}</small>
                  <span v-if="latestVerificationCode(order)">
                    <code>{{ latestVerificationCode(order) }}</code>
                    <CopyButton :text="latestVerificationCode(order)" />
                  </span>
                  <span v-else class="sms-waiting-code">{{ isOrderWaiting(order) ? t('sms.user.waitingForCode') : '-' }}</span>
                </div>

                <div class="sms-live-order__actions">
                  <button
                    v-if="showCancelAction(order)"
                    type="button"
                    class="sms-live-action sms-live-action--danger"
                    :disabled="!canCancelOrder(order)"
                    :title="cancelActionHint(order)"
                    @click="cancel(order)"
                  >
                    {{ t('sms.user.cancel') }}
                  </button>
                  <button
                    v-if="order.status === 'active' && order.product_type === 'temporary' && order.capabilities?.supports_resend"
                    type="button"
                    class="sms-live-action"
                    @click="resend(order.id)"
                  >
                    {{ t('sms.user.resend') }}
                  </button>
                </div>

                <p v-if="order.status === 'reconciling' && order.reconciliation_action === 'purchase'" class="sms-live-order__notice">
                  {{ t('sms.user.purchaseConfirmingInline') }}
                </p>
              </article>
            </div>
          </section>

          <section v-if="!liveOrders.length && !quotes.length && serviceCode && countryCode && !quoting" class="sms-empty-panel">
            <Icon name="infoCircle" size="md" aria-hidden="true" />
            <span>{{ t('sms.user.noChannel') }}</span>
          </section>
        </template>

        <section v-else class="sms-orders">
          <form class="sms-order-filters" @submit.prevent="applyOrderFilters">
            <label>
              <span>{{ t('verificationRecords.filters.keyword') }}</span>
              <div class="sms-filter-input">
                <Icon name="search" size="sm" aria-hidden="true" />
                <input v-model.trim="orderDraft.keyword" :placeholder="t('verificationRecords.filters.keywordPlaceholder')" />
              </div>
            </label>
            <Select
              v-model="orderDraft.status"
              :label="t('verificationRecords.filters.outcome')"
              :options="orderStatusOptions"
              :placeholder="t('verificationRecords.filters.allOutcomes')"
              clearable
              searchable
            />
            <div class="sms-order-filters__actions">
              <button type="submit" class="sms-filter-primary" :disabled="ordersLoading">
                <Icon name="search" size="sm" aria-hidden="true" />{{ t('common.search') }}
              </button>
              <button type="button" class="sms-filter-secondary" :disabled="ordersLoading" @click="resetOrderFilters">
                <Icon name="eraser" size="sm" aria-hidden="true" />{{ t('common.reset') }}
              </button>
            </div>
          </form>

          <div v-if="ordersLoading" class="sms-orders-state">{{ t('sms.user.loading') }}</div>
          <div v-else-if="!orders.length" class="sms-orders-state">{{ t('sms.user.noOrders') }}</div>

          <div
            v-else
            class="sms-orders-scroll sms-orders-table-wrap overflow-x-auto"
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
              <thead class="whitespace-nowrap">
                <tr>
                  <th>{{ t('sms.user.order') }}</th>
                  <th>{{ t('sms.user.channel') }}</th>
                  <th>{{ t('sms.user.service') }}</th>
                  <th>{{ t('sms.user.country') }}</th>
                  <th>{{ t('sms.user.operator') }}</th>
                  <th>{{ t('sms.user.phone') }}</th>
                  <th>{{ t('sms.user.status') }}</th>
                  <th>{{ t('sms.user.code') }}</th>
                  <th>{{ t('sms.user.price') }}</th>
                  <th>{{ t('sms.user.expiresIn') }}</th>
                  <th>{{ t('sms.user.actions') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="order in orders" :key="order.id">
                  <td class="whitespace-nowrap">
                    <span class="sms-order-id"><code>{{ order.id }}</code><CopyButton :text="order.id" /></span>
                  </td>
                  <td class="whitespace-nowrap">{{ smsChannelLabel(order.channel_code) }}</td>
                  <td class="whitespace-nowrap">
                    <span class="sms-order-entity">
                      <SMSServiceLogo :icon="serviceIcon(order.service_code, serviceLabel(order.service_code))" :label="serviceLabel(order.service_code)" class="sms-service-logo sms-service-logo--tiny" />
                      <span>{{ serviceLabel(order.service_code) }}</span>
                    </span>
                  </td>
                  <td class="whitespace-nowrap">
                    <span class="sms-order-entity">
                      <span :class="flagClass(order.country_code)" class="fi fis" aria-hidden="true"></span>
                      <span>{{ countryLabel(order.country_code) }}</span>
                    </span>
                  </td>
                  <td class="whitespace-nowrap"><code>{{ order.operator_code || 'any' }}</code></td>
                  <td class="whitespace-nowrap">
                    <span class="sms-order-phone">
                      <code>{{ order.phone_number || '-' }}</code>
                      <SMSPhoneCopy v-if="order.phone_number" :phone="order.phone_number" :calling-code="order.calling_code" />
                    </span>
                  </td>
                  <td class="whitespace-nowrap">
                    <div class="sms-order-statuses min-w-max">
                      <span class="badge" :class="statusClass(order.status)">{{ statusLabel(order.status, order.reconciliation_action) }}</span>
                      <span v-if="order.refund_status !== 'not_requested'" class="badge badge-warning">{{ refundLabel(order.refund_status) }}</span>
                    </div>
                  </td>
                  <td class="whitespace-nowrap">
                    <div v-if="order.messages?.length" class="sms-order-messages">
                      <div v-for="message in order.messages" :key="message.id">
                        <div v-if="message.verification_code" class="sms-order-code">
                          <code>{{ message.verification_code }}</code>
                          <CopyButton :text="message.verification_code" />
                        </div>
                        <div class="sms-order-message-meta">
                          <span v-if="message.sender">{{ message.sender }}</span>
                          <time v-if="message.provider_received_at" :datetime="message.provider_received_at">{{ formatSMSMessageTime(message.provider_received_at) }}</time>
                          <span v-if="message.other_sms" class="badge">{{ t('sms.user.otherMessage') }}</span>
                        </div>
                        <div class="sms-order-message-text truncate" :title="message.message_text || undefined">{{ message.message_text || '-' }}</div>
                      </div>
                    </div>
                    <span v-else>-</span>
                  </td>
                  <td class="whitespace-nowrap">{{ order.price.toFixed(4) }}</td>
                  <td class="whitespace-nowrap">{{ remainingLabel(order) }}</td>
                  <td class="whitespace-nowrap">
                    <div class="sms-order-actions">
                      <button
                        v-if="showCancelAction(order)"
                        type="button"
                        class="sms-row-button"
                        :disabled="!canCancelOrder(order)"
                        :title="cancelActionHint(order)"
                        @click="cancel(order)"
                      >
                        {{ t('sms.user.cancel') }}
                      </button>
                      <button
                        v-if="order.status === 'active' && order.product_type === 'rental' && order.capabilities?.supports_rental_add_service"
                        type="button"
                        class="sms-row-button"
                        @click="openRentalServiceModal(order)"
                      >
                        {{ t('sms.user.addService') }}
                      </button>
                      <button
                        v-if="order.status === 'active' && order.product_type === 'rental' && order.capabilities?.supports_extend"
                        type="button"
                        class="sms-row-button"
                        @click="extend(order.id)"
                      >
                        {{ t('sms.user.extend') }}
                      </button>
                      <button v-if="canRestoreRental(order)" type="button" class="sms-row-button" @click="restoreRental(order)">
                        {{ t('sms.user.restoreRental') }}
                      </button>
                      <button
                        v-if="order.status === 'active' && order.product_type === 'temporary' && order.capabilities?.supports_resend"
                        type="button"
                        class="sms-row-button"
                        @click="resend(order.id)"
                      >
                        {{ t('sms.user.resend') }}
                      </button>
                      <button
                        type="button"
                        class="sms-row-button sms-row-button--icon"
                        :disabled="refreshingId === order.id"
                        :aria-label="t('common.refresh')"
                        @click="refreshOrder(order.id)"
                      >
                        <Icon name="refresh" size="sm" :class="{ 'sms-spin': refreshingId === order.id }" />
                      </button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <div v-if="orderPagination.total > 0" class="sms-orders__pagination">
            <Pagination
              :page="orderPagination.page"
              :total="orderPagination.total"
              :page-size="orderPagination.pageSize"
              @update:page="changeOrderPage"
              @update:page-size="changeOrderPageSize"
            />
          </div>
        </section>

        <div v-if="rentalServiceState.show" class="sms-modal" @click.self="closeRentalServiceModal">
          <section class="sms-modal__dialog">
            <header>
              <div>
                <h2>{{ t('sms.user.addServiceTitle') }}</h2>
                <p>{{ t('sms.user.addServiceDescription') }}</p>
              </div>
              <button type="button" @click="closeRentalServiceModal">{{ t('common.close') }}</button>
            </header>

            <label>
              <span class="sms-field-label">{{ t('sms.user.rentDays') }}</span>
              <div class="sms-modal__inline">
                <input v-model.number="rentalServiceState.rentDays" class="sms-field" type="number" min="1" max="366" />
                <button type="button" class="sms-filter-secondary" :disabled="rentalServiceState.loading" @click="loadRentalServiceOptions">{{ t('common.refresh') }}</button>
              </div>
            </label>

            <label>
              <span class="sms-field-label">{{ t('sms.user.service') }}</span>
              <select v-model="rentalServiceState.selectedService" class="sms-field" :disabled="rentalServiceState.loading">
                <option value="">{{ t('sms.user.selectService') }}</option>
                <option v-for="item in rentalServiceState.options" :key="item.code" :value="item.code">
                  {{ item.name }} · {{ formatPrice(item.sale_price) }} {{ t('sms.user.currency') }}
                </option>
              </select>
            </label>

            <div v-if="rentalServiceState.loading" class="sms-list-state">{{ t('sms.user.loading') }}</div>
            <div v-else-if="!rentalServiceState.options.length" class="sms-list-state">{{ t('sms.user.noAdditionalServices') }}</div>

            <footer>
              <button type="button" class="sms-filter-secondary" @click="closeRentalServiceModal">{{ t('common.cancel') }}</button>
              <button
                type="button"
                class="sms-filter-primary"
                :disabled="!rentalServiceState.selectedService || rentalServiceState.loading"
                @click="quoteRentalServiceAddition"
              >
                {{ t('sms.user.getQuote') }}
              </button>
            </footer>
          </section>
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
    </main>
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
import CopyButton from '@/components/common/CopyButton.vue'
import Icon from '@/components/icons/Icon.vue'
import SMSServiceLogo from '@/components/sms/SMSServiceLogo.vue'
import SMSPhoneCopy from '@/components/sms/SMSPhoneCopy.vue'
import { smsAPI, type SMSCountryItem, type SMSOperatorItem, type SMSOrder, type SMSOrderPage, type SMSProviderItem, type SMSQuote, type SMSRecentSuccessItem, type SMSRentalServiceOption, type SMSServiceItem } from '@/api/sms'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useAppStore, useAuthStore } from '@/stores'
import { smsOrderPollBucket, smsOrderPollDelay, type SMSOrderPollBucket } from './smsPolling'

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
let countdownTimer: number | undefined
let serviceSearchTimer: number | undefined
let countrySearchTimer: number | undefined
let countryRequestVersion = 0
let operatorRequestVersion = 0

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
const selectedServiceLogo = computed(() => serviceLogo(selectedService.value?.code || '', selectedService.value?.name || '') || selectedService.value?.icon || '')
const selectedCountryStats = computed(() => countries.value.find(item => item.iso2 === countryCode.value))
const bestQuote = computed(() => [...quotes.value].sort((a, b) => a.sale_price - b.sale_price)[0])
const recentSuccessLoop = computed(() => recentSuccessItems.value.length ? [...recentSuccessItems.value, ...recentSuccessItems.value] : [])
const serviceOptions = computed(() => services.value.map(item => ({ value: item.code, label: item.name || item.code, logo: serviceLogo(item.code, item.name) || item.icon || '', stock: item.stock, startingPrice: item.starting_price })))
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
  // The user API exposes channel capabilities, never provider identities.
  // Weekly/monthly rental semantics are enabled only for channels whose
  // backend advertises server-authoritative rental constraints.
  if (currentProvider.value?.capabilities.supports_rental_constraints) {
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
const pollTimers: Partial<Record<SMSOrderPollBucket, number>> = {}

const pollingCandidates = () => [...liveOrders.value, ...(activeTab.value === 'orders' ? orders.value : [])]
  .filter(isOrderWaiting)
  .filter((order, index, items) => items.findIndex(item => item.id === order.id) === index)
  .slice(0, 20)

// Channel 1 deliberately retains the frozen three-second browser cadence.
// Channel 2 uses conservative age bands, but only the opaque channel code
// crosses the user boundary.
function stopOrderPolling() {
  Object.keys(pollTimers).forEach(key => {
    const bucket = key as SMSOrderPollBucket
    if (pollTimers[bucket]) window.clearTimeout(pollTimers[bucket])
    delete pollTimers[bucket]
  })
}

function ensureOrderPolling() {
  const candidates = pollingCandidates()
  const buckets: SMSOrderPollBucket[] = ['baseline', 'channel-2-fast', 'channel-2-medium', 'channel-2-slow']
  buckets.forEach(bucket => {
    const bucketCandidates = candidates.filter(order => smsOrderPollBucket(order) === bucket)
    if (!bucketCandidates.length) {
      if (pollTimers[bucket]) window.clearTimeout(pollTimers[bucket])
      delete pollTimers[bucket]
      return
    }
    if (pollTimers[bucket]) return
    pollTimers[bucket] = window.setTimeout(async () => {
      delete pollTimers[bucket]
      await refreshWaitingOrders(bucket)
      ensureOrderPolling()
    }, smsOrderPollDelay(bucket))
  })
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

function smsChannelLabel(code: string) {
  const match = /^channel_(\d+)$/.exec(String(code || '').trim().toLowerCase())
  return match ? `${t('sms.user.channel')}${match[1]}` : t('sms.user.channel')
}

function operatorStockLabel(item: SMSOperatorItem) {
  if (item.stock == null) return ''
  if (item.source_type === 'donor' && item.stock >= 9999) return ` · ${t('sms.user.stockAbundant')}`
  return ` · ${t('sms.user.stock')} ${item.stock}`
}

function serviceIcon(code: string, name = '') {
  const service = services.value.find(item => item.code === code)
  return serviceLogo(code, name || service?.name || '') || service?.icon || ''
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
    [/venmo/, 'simple-icons:venmo'], [/fiverr/, 'simple-icons:fiverr'], [/twilio/, 'simple-icons:twilio'],
    [/etoro/, 'simple-icons:etoro'], [/enel/, 'simple-icons:enel'],
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
  if (!providerCode.value || !serviceCode.value) return

  const requestVersion = ++countryRequestVersion
  const requestedProvider = providerCode.value
  const requestedService = serviceCode.value
  const requestedProductType = productType.value
  const requestedDurationValue = durationValue.value
  const requestedDurationUnit = durationUnit.value
  const requestedKeyword = countryKeyword.value
  const requestedSort = currentProvider.value?.capabilities.supports_conversion_stats ? countrySortMode.value : 'name'
  const requestedPage = reset ? 1 : countryPagination.page

  if (reset) {
    countries.value = []
    operators.value = []
    quotes.value = []
    countryPagination.page = 1
    countryPagination.total = 0
    countryPagination.hasMore = false
    countryCode.value = ''
    operatorCode.value = 'any'
    ++operatorRequestVersion
  }

  countriesLoading.value = true
  try {
    const page = await smsAPI.serviceCountriesPage(requestedProvider, requestedService, {
      page: requestedPage,
      page_size: countryPagination.pageSize,
      keyword: requestedKeyword || undefined,
      sort: requestedSort,
      product_type: requestedProductType,
      duration_value: requestedProductType === 'rental' ? requestedDurationValue : undefined,
      duration_unit: requestedProductType === 'rental' ? requestedDurationUnit : undefined,
    })

    // Only the newest country request is allowed to mutate selection state.
    // A request started for a previously selected service/provider can finish
    // later and must never overwrite the current service's country list.
    if (
      requestVersion !== countryRequestVersion ||
      requestedProvider !== providerCode.value ||
      requestedService !== serviceCode.value ||
      requestedProductType !== productType.value
    ) {
      return
    }

    countries.value = reset
      ? page.items
      : [...countries.value, ...page.items.filter(item => !countries.value.some(existing => existing.iso2 === item.iso2))]
    countryPagination.total = page.total
    countryPagination.hasMore = page.has_more
    countryPagination.page = page.has_more ? requestedPage + 1 : requestedPage

    if (reset) {
      countryCode.value = countries.value.find(item => item.available !== false)?.iso2 || ''
    }
    await loadOperators()
  } finally {
    if (requestVersion === countryRequestVersion) {
      countriesLoading.value = false
    }
  }
}

async function loadProviderCatalog() {
  ++countryRequestVersion
  ++operatorRequestVersion
  countriesLoading.value = false
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
  durationUnit.value = provider.capabilities.supports_rental_constraints ? 'week' : 'hour'
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
  if (type === 'rental' && currentProvider.value?.capabilities.supports_rental_constraints && !['week', 'month'].includes(durationUnit.value)) {
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

async function resend(id: string) {
  try { await smsAPI.resend(id); await refreshOrder(id); appStore.showSuccess(t('sms.user.resendSuccess')) }
  catch (error) { appStore.showError(errorMessage(error, t('sms.user.resendFailed'))) }
}



async function handleAdditionalRentalServiceChange() {
  quotes.value = []
  if (countryCode.value) await loadQuotes()
}

async function selectService(code: string) {
  if (code === serviceCode.value) return

  // Invalidate any in-flight country/operator request immediately. This keeps
  // the UI from briefly pairing the new service with stale countries from the
  // previously selected service.
  ++countryRequestVersion
  ++operatorRequestVersion
  serviceCode.value = code
  additionalRentalServiceCodes.value = []
  countries.value = []
  countryCode.value = ''
  operators.value = []
  operatorCode.value = 'any'
  quotes.value = []
  countryPagination.page = 1
  countryPagination.total = 0
  countryPagination.hasMore = false
  countriesLoading.value = false
  countryKeyword.value = ''

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
  const requestVersion = ++operatorRequestVersion
  const requestedProvider = providerCode.value
  const requestedService = serviceCode.value
  const requestedCountry = countryCode.value
  const requestedProductType = productType.value
  const requestedVoiceMode = voiceMode.value
  const requestedDurationValue = durationValue.value
  const requestedDurationUnit = durationUnit.value

  operators.value = []
  operatorCode.value = 'any'
  if (!requestedProvider || !requestedService || !requestedCountry) return
  if (!currentProvider.value?.capabilities.supports_operator_selection) return

  try {
    const result = await smsAPI.operators(requestedProvider, requestedService, requestedCountry, {
      voice_mode: requestedVoiceMode,
      product_type: requestedProductType,
      duration_value: requestedProductType === 'rental' ? requestedDurationValue : undefined,
      duration_unit: requestedProductType === 'rental' ? requestedDurationUnit : undefined,
    })

    if (
      requestVersion !== operatorRequestVersion ||
      requestedProvider !== providerCode.value ||
      requestedService !== serviceCode.value ||
      requestedCountry !== countryCode.value ||
      requestedProductType !== productType.value
    ) {
      return
    }

    operators.value = result
    const selectedCountry = countries.value.find(item => item.iso2 === requestedCountry)
    const recommended = currentProvider.value?.capabilities.supports_conversion_stats
      ? selectedCountry?.recommended_operator
      : ''
    if (recommended && operators.value.some(item => item.code === recommended && item.available !== false)) {
      operatorCode.value = recommended
    } else {
      operatorCode.value = 'any'
    }
  } catch {
    if (requestVersion !== operatorRequestVersion) return
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

async function refreshWaitingOrders(bucket?: SMSOrderPollBucket) {
  const candidates = pollingCandidates().filter(order => bucket == null || smsOrderPollBucket(order) === bucket)
  if (!candidates.length) {
    if (bucket == null) stopOrderPolling()
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
  ++countryRequestVersion
  ++operatorRequestVersion
  stopOrderPolling()
  if (countdownTimer) window.clearInterval(countdownTimer)
  if (serviceSearchTimer) window.clearTimeout(serviceSearchTimer)
  if (countrySearchTimer) window.clearTimeout(countrySearchTimer)
})
</script>

<style scoped>
.sms-page {
  min-height: calc(100dvh - 4rem);
  color: var(--color-text-primary);
}

.sms-shell {
  width: min(1480px, calc(100% - 40px));
  margin-inline: auto;
  padding: 22px 0 52px;
}

.sms-page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
}

.sms-eyebrow,
.sms-section-kicker {
  margin: 0;
  color: var(--color-accent);
  font-size: .7rem;
  font-weight: 760;
  letter-spacing: .11em;
  text-transform: uppercase;
}

.sms-page-header h1 {
  margin: .48rem 0 0;
  font-size: 1.75rem;
  font-weight: 730;
  line-height: 1.2;
  letter-spacing: -.035em;
}

.sms-page-header p:not(.sms-eyebrow) {
  max-width: 760px;
  margin: .42rem 0 0;
  color: var(--color-text-secondary);
  font-size: .82rem;
  line-height: 1.6;
}

.sms-refresh-button {
  display: inline-flex;
  min-height: 38px;
  align-items: center;
  gap: 7px;
  padding: 0 12px;
  border: 1px solid var(--color-border);
  border-radius: 10px;
  background: var(--color-surface);
  color: var(--color-text-secondary);
  cursor: pointer;
  font: inherit;
  font-size: .7rem;
  font-weight: 630;
  box-shadow: var(--shadow-xs);
}

.sms-refresh-button:hover:not(:disabled) {
  border-color: var(--color-primary-border);
  color: var(--color-primary);
}

.sms-success-strip {
  display: flex;
  min-height: 44px;
  align-items: center;
  gap: 10px;
  margin-top: 12px;
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-radius: 12px;
  background: var(--color-surface);
}

.sms-success-strip__label {
  display: inline-flex;
  min-height: 42px;
  flex: 0 0 auto;
  align-items: center;
  gap: 6px;
  padding: 0 13px;
  border-right: 1px solid var(--color-border-subtle);
  background: color-mix(in srgb, var(--color-success) 7%, var(--color-surface));
  color: var(--color-success);
  font-size: .67rem;
  font-weight: 700;
}

.sms-success-strip__viewport {
  min-width: 0;
  flex: 1 1 auto;
  overflow: hidden;
}

.sms-success-track {
  display: flex;
  width: max-content;
  align-items: center;
  gap: 7px;
  animation: sms-success-marquee 38s linear infinite;
}

.sms-success-track:hover {
  animation-play-state: paused;
}

.sms-success-item {
  display: inline-flex;
  min-height: 29px;
  flex: 0 0 auto;
  align-items: center;
  gap: 6px;
  padding: 0 9px;
  border: 1px solid var(--color-border-subtle);
  border-radius: 8px;
  background: var(--color-surface-soft);
  color: var(--color-text-muted);
  font-size: .63rem;
}

.sms-success-item strong {
  color: var(--color-text-secondary);
  font-weight: 650;
}

.sms-success-item .fi {
  border-radius: 2px;
  box-shadow: 0 0 0 1px var(--color-border-subtle);
}

.sms-success-item code {
  color: var(--color-text-muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

.sms-tabs {
  display: flex;
  min-height: 52px;
  align-items: flex-end;
  gap: 3px;
  margin-top: 8px;
  border-bottom: 1px solid var(--color-border);
}

.sms-tabs button {
  position: relative;
  min-height: 48px;
  padding: 0 14px;
  border: 0;
  background: transparent;
  color: var(--color-text-muted);
  cursor: pointer;
  font: inherit;
  font-size: .76rem;
  font-weight: 630;
}

.sms-tabs button::after {
  position: absolute;
  right: 10px;
  bottom: -1px;
  left: 10px;
  height: 2px;
  border-radius: 999px;
  background: transparent;
  content: '';
}

.sms-tabs button.is-active {
  color: var(--color-primary);
}

.sms-tabs button.is-active::after {
  background: var(--color-primary);
}

.sms-tabs button:disabled {
  cursor: not-allowed;
  opacity: .4;
}

.sms-workspace {
  display: grid;
  grid-template-columns: minmax(0, 1.05fr) minmax(0, .9fr) minmax(330px, .78fr);
  align-items: start;
  gap: 12px;
  margin-top: 14px;
}

.sms-workspace__left,
.sms-step--country,
.sms-step--confirm {
  min-width: 0;
  align-self: start;
}

.sms-workspace__left {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-radius: 16px;
  background: var(--color-surface);
  box-shadow: var(--shadow-xs);
}

.sms-step {
  padding: 16px;
}

.sms-step--channels {
  border-bottom: 1px solid var(--color-border-subtle);
}

.sms-step--country,
.sms-step--confirm {
  border: 1px solid var(--color-border);
  border-radius: 16px;
  background: var(--color-surface);
  box-shadow: var(--shadow-xs);
}

.sms-step__header {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.sms-step__number {
  display: grid;
  width: 30px;
  height: 30px;
  flex: 0 0 30px;
  place-items: center;
  border-radius: 50%;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: .72rem;
  font-weight: 800;
}

.sms-step__header > div {
  min-width: 0;
  flex: 1 1 auto;
}

.sms-step__header h2 {
  margin: 1px 0 0;
  color: var(--color-text-primary);
  font-size: .82rem;
  font-weight: 680;
  line-height: 1.35;
}

.sms-step__header p {
  margin: 3px 0 0;
  color: var(--color-text-muted);
  font-size: .63rem;
  line-height: 1.45;
}

.sms-step__title-line {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 7px;
}

.sms-step__title-line span {
  color: var(--color-text-muted);
  font-size: .62rem;
}

.sms-provider-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin-top: 12px;
}

.sms-provider-card {
  position: relative;
  display: grid;
  min-width: 0;
  min-height: 66px;
  grid-template-columns: 36px minmax(0, 1fr) auto;
  align-items: center;
  gap: 9px;
  padding: 9px 11px;
  border: 1px solid var(--color-border);
  border-radius: 11px;
  background: var(--color-surface-raised);
  color: var(--color-text-primary);
  cursor: pointer;
  text-align: left;
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    box-shadow var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard);
}

.sms-provider-card:hover:not(:disabled) {
  border-color: var(--color-primary-border);
}

.sms-provider-card.is-selected {
  border-color: var(--color-primary);
  background: color-mix(in srgb, var(--color-primary-soft) 52%, var(--color-surface));
  box-shadow: 0 0 0 2px var(--color-primary-ring);
}

.sms-provider-card.is-disabled {
  cursor: not-allowed;
  opacity: .48;
  filter: grayscale(.4);
}

.sms-provider-card__icon {
  position: relative;
  display: grid;
  width: 36px;
  height: 36px;
  place-items: center;
  border-radius: 10px;
  background: var(--color-primary-soft);
}

.sms-provider-card__icon span {
  position: absolute;
  width: 17px;
  height: 9px;
  border: 1.5px solid var(--color-primary);
  border-radius: 3px;
  background: var(--color-surface);
  transform: rotate(-1deg);
}

.sms-provider-card__icon span:first-child { transform: translateY(-6px); }
.sms-provider-card__icon span:nth-child(2) { transform: translateY(0); }
.sms-provider-card__icon span:last-child { transform: translateY(6px); }

.sms-provider-card__copy {
  display: grid;
  min-width: 0;
  gap: 2px;
}

.sms-provider-card__copy strong {
  font-size: .75rem;
  font-weight: 680;
}

.sms-provider-card__copy small {
  overflow: hidden;
  color: var(--color-text-muted);
  font-size: .61rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sms-provider-card.is-selected .sms-provider-card__copy small {
  color: var(--color-success);
}

.sms-provider-card__state {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--color-success);
}

.sms-provider-card__beta {
  padding: 2px 6px;
  border-radius: 999px;
  background: var(--color-surface-soft);
  color: var(--color-text-muted);
  font-size: .55rem;
  font-weight: 750;
  letter-spacing: .05em;
}

.sms-search-field {
  position: relative;
  display: block;
  margin-top: 12px;
}

.sms-search-field svg {
  position: absolute;
  z-index: 2;
  top: 50%;
  left: 11px;
  color: var(--color-text-muted);
  transform: translateY(-50%);
}

.sms-search-field input {
  width: 100%;
  min-height: 40px;
  padding: 0 11px 0 36px;
  border: 1px solid var(--color-border-strong);
  border-radius: 9px;
  outline: none;
  background: var(--color-surface);
  color: var(--color-text-primary);
  font: inherit;
  font-size: .72rem;
}

.sms-search-field input:focus,
.sms-field:focus,
.sms-native-select:focus,
.sms-filter-input input:focus {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 3px var(--color-primary-ring);
}

.sms-search-field input:disabled {
  cursor: not-allowed;
  opacity: .55;
}

.sms-option-list {
  display: grid;
  min-height: 0;
  gap: 6px;
  margin-top: 10px;
  overflow-y: auto;
  padding-right: 2px;
  scrollbar-width: thin;
}

.sms-option-list--service,
.sms-option-list--country {
  max-height: 430px;
}

.sms-option-card,
.sms-country-card {
  position: relative;
  width: 100%;
  min-width: 0;
  border: 1px solid var(--color-border);
  border-radius: 10px;
  background: var(--color-surface-raised);
  color: var(--color-text-primary);
  cursor: pointer;
  text-align: left;
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard),
    box-shadow var(--motion-fast) var(--ease-standard);
}

.sms-option-card {
  display: flex;
  min-height: 56px;
  align-items: center;
  justify-content: space-between;
  gap: 9px;
  padding: 7px 32px 7px 9px;
}

.sms-option-card:hover,
.sms-country-card:hover:not(:disabled) {
  border-color: var(--color-primary-border);
}

.sms-option-card.is-selected,
.sms-country-card.is-selected {
  border-color: var(--color-primary);
  background: color-mix(in srgb, var(--color-primary-soft) 44%, var(--color-surface));
  box-shadow: inset 3px 0 0 var(--color-primary);
}

.sms-option-card__main {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 9px;
}

.sms-service-logo {
  width: 34px !important;
  height: 34px !important;
  flex: 0 0 34px;
  border-radius: 9px;
  font-size: .74rem !important;
}

.sms-service-logo--small {
  width: 28px !important;
  height: 28px !important;
  flex-basis: 28px;
}

.sms-service-logo--tiny {
  width: 24px !important;
  height: 24px !important;
  flex-basis: 24px;
  font-size: .58rem !important;
}

.sms-option-card__copy {
  display: grid;
  min-width: 0;
  gap: 2px;
}

.sms-option-card__copy strong {
  overflow: hidden;
  font-size: .73rem;
  font-weight: 660;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sms-option-card__copy small {
  color: var(--color-success);
  font-size: .6rem;
}

.sms-option-card__meta {
  display: grid;
  flex: 0 0 auto;
  gap: 2px;
  text-align: right;
}

.sms-option-card__meta strong {
  color: var(--color-text-primary);
  font-size: .7rem;
  font-weight: 680;
}

.sms-option-card__meta small {
  color: var(--color-text-muted);
  font-size: .57rem;
}

.sms-selected-mark {
  position: absolute;
  top: 50%;
  right: 8px;
  display: grid;
  width: 18px;
  height: 18px;
  place-items: center;
  border-radius: 50%;
  background: var(--color-primary);
  color: white;
  transform: translateY(-50%);
}

.sms-country-toolbar {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 128px;
  gap: 8px;
}

.sms-country-toolbar .sms-search-field {
  margin-top: 12px;
}

.sms-native-select {
  min-height: 40px;
  align-self: end;
  margin-top: 12px;
  padding: 0 9px;
  border: 1px solid var(--color-border-strong);
  border-radius: 9px;
  outline: none;
  background: var(--color-surface);
  color: var(--color-text-primary);
  font: inherit;
  font-size: .66rem;
}

.sms-country-card {
  display: grid;
  min-height: 62px;
  grid-template-columns: 28px minmax(0, 1fr) auto;
  align-items: center;
  gap: 8px;
  padding: 7px 32px 7px 9px;
}

.sms-country-card:disabled {
  cursor: not-allowed;
  opacity: .48;
}

.sms-country-card__flag {
  font-size: 1.15rem;
  border-radius: 2px;
  box-shadow: 0 0 0 1px var(--color-border-subtle);
}

.sms-country-card__copy {
  display: grid;
  min-width: 0;
  gap: 1px;
}

.sms-country-card__name {
  display: flex;
  min-width: 0;
  align-items: baseline;
  gap: 6px;
}

.sms-country-card__name strong {
  overflow: hidden;
  font-size: .72rem;
  font-weight: 670;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sms-country-card__name small {
  flex: 0 0 auto;
  color: var(--color-text-muted);
  font-size: .58rem;
}

.sms-country-card__sub,
.sms-country-card__rate {
  overflow: hidden;
  color: var(--color-text-muted);
  font-size: .58rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sms-country-card__rate {
  color: var(--color-success);
  font-weight: 610;
}

.sms-country-card__meta {
  display: grid;
  flex: 0 0 auto;
  gap: 2px;
  text-align: right;
}

.sms-country-card__meta strong {
  color: var(--color-text-primary);
  font-size: .69rem;
  font-weight: 680;
}

.sms-country-card__meta small {
  color: var(--color-text-muted);
  font-size: .55rem;
}

.sms-list-state {
  display: flex;
  min-height: 86px;
  align-items: center;
  justify-content: center;
  color: var(--color-text-muted);
  font-size: .68rem;
  text-align: center;
}

.sms-load-more {
  min-height: 34px;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: var(--color-surface);
  color: var(--color-text-secondary);
  cursor: pointer;
  font: inherit;
  font-size: .65rem;
}

.sms-inline-hint {
  margin: 9px 0 0;
  color: var(--color-text-muted);
  font-size: .64rem;
}

.sms-step--confirm {
  background: linear-gradient(180deg, color-mix(in srgb, var(--color-primary-soft) 28%, var(--color-surface)) 0%, var(--color-surface) 30%);
}

.sms-selection-summary {
  margin-top: 12px;
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-radius: 11px;
  background: var(--color-surface-raised);
}

.sms-selection-summary > div {
  display: flex;
  min-height: 48px;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 8px 10px;
  border-bottom: 1px solid var(--color-border-subtle);
}

.sms-selection-summary > div:last-child {
  border-bottom: 0;
}

.sms-selection-summary > div > span:first-child {
  color: var(--color-text-muted);
  font-size: .64rem;
}

.sms-selection-summary strong {
  color: var(--color-text-primary);
  font-size: .71rem;
  font-weight: 670;
  text-align: right;
}

.sms-selection-summary__entity {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: flex-end;
  gap: 7px;
}

.sms-selection-summary__entity > span:last-child {
  max-width: 190px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sms-success-value {
  color: var(--color-success) !important;
  font-size: .86rem !important;
}

.sms-form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  margin-top: 11px;
}

.sms-form-grid :deep(label),
.sms-form-field {
  display: block;
  min-width: 0;
}

.sms-field-label {
  display: block;
  margin-bottom: 5px;
  color: var(--color-text-secondary);
  font-size: .64rem;
  font-weight: 620;
}

.sms-field {
  width: 100%;
  min-height: 40px;
  padding: 0 10px;
  border: 1px solid var(--color-border-strong);
  border-radius: 9px;
  outline: none;
  background: var(--color-surface);
  color: var(--color-text-primary);
  font: inherit;
  font-size: .7rem;
}

.sms-form-field {
  margin-top: 11px;
}

.sms-field-help {
  display: block;
  margin-top: 4px;
  color: var(--color-text-muted);
  font-size: .59rem;
  line-height: 1.4;
}

.sms-field-help--success {
  color: var(--color-success);
}

.sms-field-error {
  display: block;
  margin-top: 4px;
  color: var(--color-danger);
  font-size: .59rem;
}

.sms-extra-services {
  margin-top: 11px;
  padding: 9px;
  border: 1px solid var(--color-border);
  border-radius: 10px;
  background: var(--color-surface-soft);
}

.sms-extra-services__heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
}

.sms-extra-services__heading > div {
  display: grid;
  min-width: 0;
  gap: 2px;
}

.sms-extra-services__heading strong {
  font-size: .68rem;
  font-weight: 650;
}

.sms-extra-services__heading small {
  color: var(--color-text-muted);
  font-size: .58rem;
  line-height: 1.4;
}

.sms-extra-services__heading > span {
  flex: 0 0 auto;
  padding: 2px 6px;
  border-radius: 999px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  font-size: .56rem;
}

.sms-extra-services__list {
  display: grid;
  max-height: 122px;
  gap: 4px;
  margin-top: 8px;
  overflow-y: auto;
}

.sms-extra-services__list label {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 7px;
  padding: 4px 5px;
  border-radius: 7px;
  cursor: pointer;
  font-size: .64rem;
}

.sms-extra-services__list label:hover {
  background: var(--color-surface);
}

.sms-extra-services__list input {
  accent-color: var(--color-primary);
}

.sms-quantity {
  display: grid;
  grid-template-columns: 36px minmax(0, 1fr) 36px;
  min-height: 40px;
  overflow: hidden;
  border: 1px solid var(--color-border-strong);
  border-radius: 9px;
  background: var(--color-surface);
}

.sms-quantity button {
  border: 0;
  background: var(--color-surface-soft);
  color: var(--color-text-secondary);
  cursor: pointer;
  font: inherit;
  font-size: .95rem;
}

.sms-quantity button:disabled {
  cursor: not-allowed;
  opacity: .4;
}

.sms-quantity input {
  min-width: 0;
  border: 0;
  outline: none;
  background: transparent;
  color: var(--color-text-primary);
  font: inherit;
  font-size: .72rem;
  font-weight: 650;
  text-align: center;
}

.sms-price-summary {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 12px;
  margin-top: 12px;
  padding: 10px 11px;
  border-top: 1px solid var(--color-border-subtle);
  border-bottom: 1px solid var(--color-border-subtle);
}

.sms-price-summary > div {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.sms-price-summary span {
  color: var(--color-text-muted);
  font-size: .65rem;
}

.sms-price-summary strong {
  color: var(--color-primary);
  font-size: 1.24rem;
  font-weight: 740;
  font-variant-numeric: tabular-nums;
}

.sms-price-summary small {
  color: var(--color-text-muted);
  font-size: .58rem;
}

.sms-primary-action {
  display: flex;
  width: 100%;
  min-height: 42px;
  align-items: center;
  justify-content: center;
  gap: 7px;
  margin-top: 11px;
  border: 1px solid var(--color-primary);
  border-radius: 9px;
  background: linear-gradient(90deg, var(--color-primary), color-mix(in srgb, var(--color-primary) 76%, var(--color-accent)));
  color: white;
  cursor: pointer;
  font: inherit;
  font-size: .74rem;
  font-weight: 680;
  box-shadow: var(--shadow-sm);
}

.sms-primary-action:disabled {
  cursor: not-allowed;
  opacity: .55;
}

.sms-action-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid color-mix(in srgb, white 42%, transparent);
  border-top-color: white;
  border-radius: 50%;
  animation: sms-spin .75s linear infinite;
}

.sms-refund-note {
  margin: 7px 0 0;
  color: var(--color-text-muted);
  font-size: .59rem;
  text-align: center;
}

.sms-delivery-tip {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin-top: 12px;
  padding: 9px 10px;
  border: 1px solid color-mix(in srgb, var(--color-warning) 30%, var(--color-border));
  border-radius: 10px;
  background: color-mix(in srgb, var(--color-warning) 6%, var(--color-surface));
  color: var(--color-warning);
}

.sms-delivery-tip > svg {
  flex: 0 0 auto;
  margin-top: 1px;
}

.sms-delivery-tip strong {
  display: block;
  color: var(--color-text-primary);
  font-size: .66rem;
  font-weight: 670;
}

.sms-delivery-tip p {
  margin: 3px 0 0;
  color: var(--color-text-muted);
  font-size: .59rem;
  line-height: 1.45;
}

.sms-quote-panel,
.sms-live-orders,
.sms-empty-panel,
.sms-orders {
  margin-top: 14px;
  border: 1px solid var(--color-border);
  border-radius: 15px;
  background: var(--color-surface);
  box-shadow: var(--shadow-xs);
}

.sms-quote-panel {
  overflow: hidden;
}

.sms-quote-panel > header {
  display: flex;
  min-height: 60px;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--color-border-subtle);
}

.sms-quote-panel h2 {
  margin: 3px 0 0;
  font-size: .78rem;
  font-weight: 670;
}

.sms-quote-panel > header > span {
  display: grid;
  min-width: 24px;
  height: 24px;
  place-items: center;
  border-radius: 999px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  font-size: .6rem;
  font-weight: 700;
}

.sms-quote-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  padding: 10px;
}

.sms-quote-card {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 10px;
  padding: 9px 10px;
  border: 1px solid var(--color-border);
  border-radius: 10px;
  background: var(--color-surface-soft);
}

.sms-quote-card > div {
  display: grid;
  min-width: 0;
  gap: 2px;
}

.sms-quote-card strong {
  font-size: .68rem;
  font-weight: 670;
}

.sms-quote-card small {
  color: var(--color-text-muted);
  font-size: .58rem;
}

.sms-quote-card > div:nth-child(2) {
  text-align: right;
}

.sms-quote-card > button {
  min-height: 32px;
  padding: 0 10px;
  border: 1px solid var(--color-primary);
  border-radius: 8px;
  background: var(--color-primary);
  color: white;
  cursor: pointer;
  font: inherit;
  font-size: .62rem;
  font-weight: 650;
}

.sms-live-orders {
  overflow: hidden;
}

.sms-live-orders__header {
  min-height: 58px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--color-border-subtle);
}

.sms-live-orders__header > div {
  display: flex;
  align-items: center;
  gap: 9px;
}

.sms-live-indicator {
  width: 9px;
  height: 9px;
  flex: 0 0 9px;
  border-radius: 50%;
  background: var(--color-success);
  box-shadow: 0 0 0 4px color-mix(in srgb, var(--color-success) 12%, transparent);
}

.sms-live-orders__header h2 {
  margin: 0;
  font-size: .78rem;
  font-weight: 680;
}

.sms-live-orders__header p {
  margin: 2px 0 0;
  color: var(--color-text-muted);
  font-size: .6rem;
}

.sms-live-orders__list {
  display: grid;
}

.sms-live-order {
  position: relative;
  display: grid;
  min-width: 980px;
  grid-template-columns: minmax(180px, 1fr) 150px minmax(190px, 1fr) 135px 110px 170px auto;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  border-bottom: 1px solid var(--color-border-subtle);
}

.sms-live-order:last-child {
  border-bottom: 0;
}

.sms-live-order__service,
.sms-live-order__country,
.sms-live-order__phone > span,
.sms-live-order__code > span {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 7px;
}

.sms-live-order__service > span:last-child,
.sms-live-order__country > span:last-child {
  display: grid;
  min-width: 0;
  gap: 1px;
}

.sms-live-order strong {
  overflow: hidden;
  color: var(--color-text-primary);
  font-size: .7rem;
  font-weight: 670;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sms-live-order small {
  display: block;
  color: var(--color-text-muted);
  font-size: .57rem;
}

.sms-live-order__phone > small,
.sms-live-order__status > small,
.sms-live-order__expiry > small,
.sms-live-order__code > small {
  margin-bottom: 3px;
}

.sms-live-order__phone code,
.sms-live-order__code code {
  color: var(--color-text-primary);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: .73rem;
  font-weight: 680;
}

.sms-live-order__expiry strong {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: .76rem;
}

.sms-waiting-code {
  color: var(--color-text-muted);
  font-size: .64rem;
}

.sms-live-order__actions {
  display: flex;
  justify-content: flex-end;
  gap: 6px;
}

.sms-live-action {
  min-height: 32px;
  padding: 0 9px;
  border: 1px solid var(--color-primary-border);
  border-radius: 8px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  cursor: pointer;
  font: inherit;
  font-size: .61rem;
  font-weight: 640;
}

.sms-live-action--danger {
  border-color: color-mix(in srgb, var(--color-danger) 30%, var(--color-border));
  background: color-mix(in srgb, var(--color-danger) 6%, var(--color-surface));
  color: var(--color-danger);
}

.sms-live-action:disabled {
  cursor: not-allowed;
  opacity: .45;
}

.sms-live-order__notice {
  grid-column: 1 / -1;
  margin: 0;
  padding: 7px 9px;
  border-radius: 8px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  font-size: .59rem;
  line-height: 1.45;
}

.sms-empty-panel {
  display: flex;
  min-height: 54px;
  align-items: center;
  justify-content: center;
  gap: 7px;
  padding: 10px 14px;
  color: var(--color-text-muted);
  font-size: .68rem;
}

.sms-orders {
  overflow: hidden;
}

.sms-order-filters {
  display: grid;
  grid-template-columns: minmax(300px, 1fr) 210px auto;
  align-items: end;
  gap: 10px;
  padding: 14px;
  border-bottom: 1px solid var(--color-border-subtle);
}

.sms-order-filters > label {
  display: grid;
  gap: 5px;
  color: var(--color-text-secondary);
  font-size: .64rem;
  font-weight: 620;
}

.sms-filter-input {
  position: relative;
}

.sms-filter-input svg {
  position: absolute;
  top: 50%;
  left: 11px;
  color: var(--color-text-muted);
  transform: translateY(-50%);
}

.sms-filter-input input {
  width: 100%;
  min-height: 40px;
  padding: 0 10px 0 35px;
  border: 1px solid var(--color-border-strong);
  border-radius: 9px;
  outline: none;
  background: var(--color-surface);
  color: var(--color-text-primary);
  font: inherit;
  font-size: .7rem;
}

.sms-order-filters__actions {
  display: flex;
  gap: 7px;
}

.sms-filter-primary,
.sms-filter-secondary {
  display: inline-flex;
  min-height: 40px;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 0 12px;
  border-radius: 9px;
  cursor: pointer;
  font: inherit;
  font-size: .68rem;
  font-weight: 650;
}

.sms-filter-primary {
  border: 1px solid var(--color-primary);
  background: var(--color-primary);
  color: white;
}

.sms-filter-secondary {
  border: 1px solid var(--color-border);
  background: var(--color-surface);
  color: var(--color-text-secondary);
}

.sms-orders-state {
  display: flex;
  min-height: 160px;
  align-items: center;
  justify-content: center;
  color: var(--color-text-muted);
  font-size: .7rem;
}

.sms-orders-table-wrap {
  max-width: 100%;
  background: var(--color-surface);
  outline: none;
}

.sms-orders-table {
  border-collapse: collapse;
}

.sms-orders-table thead {
  background: var(--color-surface-soft);
  color: var(--color-text-muted);
  font-size: .61rem;
  font-weight: 650;
  text-transform: uppercase;
}

.sms-orders-table th,
.sms-orders-table td {
  padding: 10px 12px;
  border-bottom: 1px solid var(--color-border-subtle);
  vertical-align: middle;
}

.sms-orders-table tbody tr:last-child td {
  border-bottom: 0;
}

.sms-orders-table tbody tr:hover {
  background: var(--color-surface-soft);
}

.sms-orders-table td {
  color: var(--color-text-secondary);
  font-size: .67rem;
}

.sms-order-id,
.sms-order-phone,
.sms-order-entity,
.sms-order-code,
.sms-order-statuses,
.sms-order-actions {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 6px;
}

.sms-order-id code,
.sms-order-phone code,
.sms-order-code code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

.sms-order-messages {
  display: grid;
  gap: 7px;
}

.sms-order-code code {
  font-weight: 700;
}

.sms-order-message-meta {
  display: flex;
  max-width: 300px;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px 7px;
  margin-top: 3px;
  color: var(--color-text-muted);
  font-size: .57rem;
}

.sms-order-message-text {
  max-width: 300px;
  margin-top: 3px;
  color: var(--color-text-muted);
  font-size: .61rem;
}

.sms-row-button {
  min-height: 30px;
  padding: 0 8px;
  border: 1px solid var(--color-border);
  border-radius: 7px;
  background: var(--color-surface);
  color: var(--color-text-secondary);
  cursor: pointer;
  font: inherit;
  font-size: .59rem;
}

.sms-row-button--icon {
  display: grid;
  width: 30px;
  padding: 0;
  place-items: center;
}

.sms-row-button:disabled {
  cursor: not-allowed;
  opacity: .45;
}

.sms-orders__pagination {
  border-top: 1px solid var(--color-border-subtle);
}

.sms-modal {
  position: fixed;
  z-index: 60;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 18px;
  background: rgba(15, 23, 42, .44);
  -webkit-backdrop-filter: blur(5px);
  backdrop-filter: blur(5px);
}

.sms-modal__dialog {
  display: grid;
  width: min(520px, 100%);
  gap: 14px;
  padding: 18px;
  border: 1px solid var(--color-border);
  border-radius: 16px;
  background: var(--color-surface);
  box-shadow: var(--shadow-xl);
}

.sms-modal__dialog > header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
}

.sms-modal__dialog h2 {
  margin: 0;
  font-size: .92rem;
  font-weight: 680;
}

.sms-modal__dialog p {
  margin: 4px 0 0;
  color: var(--color-text-muted);
  font-size: .66rem;
  line-height: 1.45;
}

.sms-modal__dialog > header > button {
  border: 0;
  background: transparent;
  color: var(--color-text-muted);
  cursor: pointer;
  font: inherit;
  font-size: .66rem;
}

.sms-modal__inline {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 7px;
}

.sms-modal__dialog > footer {
  display: flex;
  justify-content: flex-end;
  gap: 7px;
}

.sms-spin {
  animation: sms-spin .8s linear infinite;
}

@keyframes sms-spin {
  to { transform: rotate(360deg); }
}

@keyframes sms-success-marquee {
  from { transform: translateX(0); }
  to { transform: translateX(-50%); }
}

@media (max-width: 1180px) {
  .sms-workspace {
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  }

  .sms-step--confirm {
    grid-column: 1 / -1;
  }

  .sms-step--confirm {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(300px, .8fr);
    align-items: start;
    gap: 0 16px;
  }

  .sms-step--confirm > .sms-step__header,
  .sms-step--confirm > .sms-selection-summary {
    grid-column: 1;
  }

  .sms-step--confirm > :not(.sms-step__header):not(.sms-selection-summary) {
    grid-column: 2;
  }

  .sms-quote-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 860px) {
  .sms-shell {
    width: min(100% - 28px, 760px);
  }

  .sms-refresh-button span {
    display: none;
  }

  .sms-workspace {
    grid-template-columns: 1fr;
  }

  .sms-step--confirm {
    display: block;
  }

  .sms-country-toolbar {
    grid-template-columns: minmax(0, 1fr) 120px;
  }

  .sms-quote-grid {
    grid-template-columns: 1fr;
  }

  .sms-live-orders {
    overflow-x: auto;
  }

  .sms-order-filters {
    grid-template-columns: 1fr 1fr;
  }

  .sms-order-filters__actions {
    grid-column: 1 / -1;
  }
}

@media (max-width: 560px) {
  .sms-shell {
    width: min(100% - 20px, 520px);
    padding-top: 16px;
  }

  .sms-page-header h1 {
    font-size: 1.55rem;
  }

  .sms-page-header p:not(.sms-eyebrow) {
    font-size: .75rem;
  }

  .sms-success-strip__label {
    padding-inline: 9px;
  }

  .sms-tabs {
    overflow-x: auto;
  }

  .sms-tabs button {
    min-width: max-content;
  }

  .sms-workspace {
    gap: 10px;
  }

  .sms-workspace__left,
  .sms-step--country,
  .sms-step--confirm {
    border-radius: 13px;
  }

  .sms-step {
    padding: 13px;
  }

  .sms-provider-grid {
    grid-template-columns: 1fr;
  }

  .sms-country-toolbar {
    grid-template-columns: 1fr;
  }

  .sms-native-select {
    margin-top: 0;
  }

  .sms-form-grid {
    grid-template-columns: 1fr;
  }

  .sms-selection-summary__entity > span:last-child {
    max-width: 150px;
  }

  .sms-price-summary {
    align-items: flex-start;
    flex-direction: column;
  }

  .sms-quote-card {
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .sms-quote-card > button {
    grid-column: 1 / -1;
    width: 100%;
  }

  .sms-order-filters {
    grid-template-columns: 1fr;
  }

  .sms-order-filters__actions {
    grid-column: auto;
  }

  .sms-filter-primary,
  .sms-filter-secondary {
    flex: 1 1 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .sms-success-track,
  .sms-spin,
  .sms-action-spinner {
    animation: none;
  }

  .sms-provider-card,
  .sms-option-card,
  .sms-country-card {
    transition-duration: 1ms;
  }
}
</style>
