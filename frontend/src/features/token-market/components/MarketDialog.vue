<template>
  <Transition name="tm-fade">
    <div v-if="dialog" class="overlay" @click.self="emit('close')">
      <section class="dialog tm-glass">
        <button class="close" type="button" aria-label="关闭" @click="emit('close')">×</button>

        <template v-if="dialog.type === 'exchange'">
          <span class="eyebrow">QUICK EXCHANGE</span>
          <h2>确认兑换</h2>
          <div class="summary">
            <strong>{{ sourceText }}</strong><span>→</span><strong>{{ destinationText }}</strong>
          </div>
          <p>当前通过 TokenMarketService 完成报价与执行；后续切换真实 API 时保持相同交互契约。</p>
          <div class="actions">
            <button class="tm-secondary-button" type="button" @click="emit('close')">取消</button>
            <button class="tm-primary-button" type="button" :disabled="loading" @click="emit('confirm-exchange')">{{ loading ? '处理中...' : '确认兑换' }}</button>
          </div>
        </template>

        <template v-else-if="dialog.type === 'product'">
          <span class="eyebrow">FEATURED PRODUCT</span>
          <h2>{{ dialog.title }}</h2>
          <p>商品详情、SKU、库存与 Token 支付接口将在 Commerce API 接入后由此入口承接。</p>
          <div class="price">{{ dialog.price }} T</div>
          <div class="actions">
            <button class="tm-secondary-button" type="button" @click="emit('close')">继续浏览</button>
            <button class="tm-primary-button" type="button" @click="emit('add-cart', dialog.title)">加入购物车</button>
          </div>
        </template>

        <template v-else>
          <span class="eyebrow">TOKEN MARKET</span>
          <h2>{{ dialog.title }}</h2>
          <p>{{ dialog.message }}</p>
          <button class="tm-primary-button full" type="button" @click="emit('close')">知道了</button>
        </template>
      </section>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import type { TokenMarketDialogState } from '../ui'

defineProps<{
  dialog: TokenMarketDialogState | null
  sourceText: string
  destinationText: string
  loading: boolean
}>()

const emit = defineEmits<{
  close: []
  'confirm-exchange': []
  'add-cart': [title: string]
}>()
</script>

<style scoped>
.overlay{position:fixed;z-index:100;inset:0;display:grid;place-items:center;padding:20px;background:rgba(0,6,18,.74);backdrop-filter:blur(10px)}.dialog{position:relative;width:min(470px,100%);padding:28px;border-radius:20px;background:linear-gradient(180deg,#10224d,#07152f 56%,#040d1f)}.close{position:absolute;top:14px;right:14px;width:32px;height:32px;border:1px solid rgba(106,136,202,.25);border-radius:50%;color:#9db0d4;background:#13244a;font-size:19px;cursor:pointer}.eyebrow{color:#7289b6;font-size:9px;font-weight:900;letter-spacing:.14em}.dialog h2{margin:6px 0 14px;font-size:24px}.dialog p{color:#8b9dbd;font-size:12px;line-height:1.75}.summary{display:grid;grid-template-columns:1fr 34px 1fr;align-items:center;gap:8px;margin:20px 0}.summary strong{display:grid;place-items:center;min-height:68px;border:1px solid rgba(105,136,205,.18);border-radius:11px;background:rgba(6,19,46,.74)}.summary span{text-align:center;color:#7087b5}.price{margin:18px 0;color:#ff82aa;font-size:27px;font-weight:900}.actions{display:grid;grid-template-columns:1fr 1fr;gap:10px;margin-top:20px}.full{width:100%;margin-top:14px}
</style>
