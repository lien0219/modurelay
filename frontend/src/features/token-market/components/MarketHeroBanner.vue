<template>
  <section class="hero-banner">
    <DesignSprite class="hero-art" :x="0" :y="55" :width="770" :height="310" :radius="0" />
    <div class="hero-overlay" />
    <div class="hero-copy">
      <h1>用 Token<br>解锁更多精彩</h1>
      <p>一个 Token，畅享多元数字生活</p>
      <span>商城购物 · 外卖美食 · 数字商品 · AI额度 · 全球优质服务</span>
    </div>
  </section>
</template>
<script setup lang="ts">
import DesignSprite from './DesignSprite.vue'
</script>
<style scoped>
.hero-banner{position:relative;min-height:280px;border-radius:24px;overflow:hidden;border:1px solid rgba(102,130,225,.18);background:#06142d;box-shadow:inset 0 1px rgba(255,255,255,.03)}
.hero-art{position:absolute;inset:0}.hero-overlay{position:absolute;inset:0;background:linear-gradient(90deg,rgba(3,10,25,.3),rgba(3,10,25,.03) 48%,rgba(3,10,25,.1))}.hero-copy{position:relative;z-index:2;padding:42px 34px;color:#fff;text-shadow:0 3px 18px rgba(0,0,0,.4)}.hero-copy h1{margin:0;font-size:clamp(34px,3vw,58px);line-height:1.05;letter-spacing:-.04em}.hero-copy p{margin:18px 0 8px;font-size:18px}.hero-copy span{color:#b6c9ec;font-size:11px}
@media(max-width:760px){.hero-banner{min-height:230px}.hero-copy{padding:28px 22px}.hero-copy p{font-size:14px}.hero-copy span{font-size:9px}}
</style>
