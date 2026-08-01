<template>
  <div
    ref="stageRef"
    class="hero-orbit-stage"
    role="img"
    aria-label="ModuRelay digital service network"
    @pointermove="handlePointerMove"
    @pointerleave="resetPointer"
  >
    <div class="scene-shell">
      <div class="scene-grid"></div>
      <div class="scene-aurora scene-aurora-one"></div>
      <div class="scene-aurora scene-aurora-two"></div>

      <div class="globe" aria-hidden="true">
        <span class="globe-line globe-line-one"></span>
        <span class="globe-line globe-line-two"></span>
        <span class="globe-line globe-line-three"></span>
      </div>

      <div class="orbit orbit-one" aria-hidden="true">
        <span class="orbit-node node-one"></span>
        <span class="orbit-node node-two"></span>
      </div>
      <div class="orbit orbit-two" aria-hidden="true">
        <span class="orbit-node node-three"></span>
      </div>
      <div class="orbit orbit-three" aria-hidden="true"></div>

      <div class="core-platform" aria-hidden="true">
        <div class="platform-ring platform-ring-one"></div>
        <div class="platform-ring platform-ring-two"></div>
        <div class="platform-beam"></div>
      </div>

      <div class="cube-wrap" aria-hidden="true">
        <div class="cube">
          <div class="cube-face cube-front">
            <span>M</span>
          </div>
          <div class="cube-face cube-back"></div>
          <div class="cube-face cube-right"><i></i><i></i><i></i></div>
          <div class="cube-face cube-left"></div>
          <div class="cube-face cube-top"></div>
          <div class="cube-face cube-bottom"></div>
        </div>
      </div>

      <article
        v-for="(service, index) in services.slice(0, 5)"
        :key="service.label"
        class="service-float"
        :class="`service-float-${index + 1}`"
        :data-tone="service.tone"
      >
        <span class="service-float-icon">
          <Icon :name="service.icon" size="sm" :stroke-width="1.8" />
        </span>
        <span>
          <strong>{{ service.label }}</strong>
          <small>{{ service.caption }}</small>
        </span>
      </article>

      <div class="scene-dashboard" aria-hidden="true">
        <div class="dashboard-head">
          <span></span><span></span><span></span>
        </div>
        <div class="dashboard-bars">
          <i></i><i></i><i></i><i></i><i></i>
        </div>
        <div class="dashboard-line">
          <svg viewBox="0 0 140 42" preserveAspectRatio="none">
            <path d="M2 35 C22 32, 29 16, 45 22 S73 34, 88 17 S113 12, 138 4" />
          </svg>
        </div>
      </div>

      <div class="scene-status" aria-hidden="true">
        <span class="status-dot"></span>
        <span>NETWORK ONLINE</span>
        <strong>99.99%</strong>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'

type HomeIcon = 'link' | 'server' | 'shield' | 'chat' | 'chart'
type ServiceTone = 'teal' | 'blue' | 'violet' | 'cyan' | 'amber'

interface SceneService {
  label: string
  caption: string
  icon: HomeIcon
  tone: ServiceTone
}

defineProps<{
  services: SceneService[]
}>()

const stageRef = ref<HTMLElement | null>(null)

function handlePointerMove(event: PointerEvent) {
  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return
  const element = stageRef.value
  if (!element) return

  const rect = element.getBoundingClientRect()
  const x = (event.clientX - rect.left) / rect.width - 0.5
  const y = (event.clientY - rect.top) / rect.height - 0.5

  element.style.setProperty('--scene-rotate-y', `${x * 8}deg`)
  element.style.setProperty('--scene-rotate-x', `${y * -6}deg`)
  element.style.setProperty('--scene-shift-x', `${x * 10}px`)
  element.style.setProperty('--scene-shift-y', `${y * 8}px`)
}

function resetPointer() {
  const element = stageRef.value
  if (!element) return
  element.style.setProperty('--scene-rotate-y', '0deg')
  element.style.setProperty('--scene-rotate-x', '0deg')
  element.style.setProperty('--scene-shift-x', '0px')
  element.style.setProperty('--scene-shift-y', '0px')
}
</script>

<style scoped>
.hero-orbit-stage {
  --scene-rotate-x: 0deg;
  --scene-rotate-y: 0deg;
  --scene-shift-x: 0px;
  --scene-shift-y: 0px;
  position: relative;
  width: min(100%, 700px);
  min-height: 520px;
  perspective: 1100px;
  isolation: isolate;
}

.scene-shell {
  position: absolute;
  inset: 0;
  overflow: hidden;
  border: 1px solid rgba(15, 118, 110, 0.16);
  border-radius: 34px;
  background:
    radial-gradient(circle at 50% 48%, rgba(13, 148, 136, 0.2), transparent 28%),
    radial-gradient(circle at 82% 10%, rgba(14, 165, 233, 0.14), transparent 36%),
    linear-gradient(145deg, rgba(255, 255, 255, 0.88), rgba(240, 253, 250, 0.7));
  box-shadow:
    0 32px 90px rgba(15, 23, 42, 0.13),
    inset 0 1px 0 rgba(255, 255, 255, 0.86);
  transform: rotateX(var(--scene-rotate-x)) rotateY(var(--scene-rotate-y));
  transform-style: preserve-3d;
  transition: transform 180ms ease-out, background 300ms ease, border-color 300ms ease;
}

:global(.dark) .scene-shell {
  border-color: rgba(45, 212, 191, 0.16);
  background:
    radial-gradient(circle at 50% 48%, rgba(13, 148, 136, 0.22), transparent 28%),
    radial-gradient(circle at 82% 10%, rgba(14, 165, 233, 0.13), transparent 36%),
    linear-gradient(145deg, rgba(7, 17, 34, 0.96), rgba(3, 10, 24, 0.92));
  box-shadow:
    0 36px 100px rgba(0, 0, 0, 0.44),
    inset 0 1px 0 rgba(255, 255, 255, 0.04),
    0 0 80px rgba(13, 148, 136, 0.08);
}

.scene-grid {
  position: absolute;
  inset: 0;
  opacity: 0.38;
  background-image:
    linear-gradient(rgba(13, 148, 136, 0.08) 1px, transparent 1px),
    linear-gradient(90deg, rgba(13, 148, 136, 0.08) 1px, transparent 1px);
  background-size: 42px 42px;
  mask-image: linear-gradient(to bottom, transparent, black 22%, black 84%, transparent);
  transform: translate3d(var(--scene-shift-x), var(--scene-shift-y), -20px) rotateX(60deg) scale(1.45);
}

.scene-aurora {
  position: absolute;
  border-radius: 999px;
  filter: blur(54px);
  opacity: 0.4;
}

.scene-aurora-one {
  right: -10%;
  top: 8%;
  width: 48%;
  height: 42%;
  background: rgba(14, 165, 233, 0.25);
}

.scene-aurora-two {
  bottom: -12%;
  left: 20%;
  width: 56%;
  height: 38%;
  background: rgba(20, 184, 166, 0.28);
}

.globe {
  position: absolute;
  left: 50%;
  top: 47%;
  width: 330px;
  height: 330px;
  border: 1px solid rgba(14, 165, 233, 0.22);
  border-radius: 50%;
  background:
    radial-gradient(circle at 40% 36%, rgba(255, 255, 255, 0.38), transparent 14%),
    radial-gradient(circle, rgba(14, 165, 233, 0.13), transparent 68%);
  box-shadow: inset 0 0 60px rgba(14, 165, 233, 0.12), 0 0 70px rgba(14, 165, 233, 0.12);
  transform: translate(-50%, -50%) translateZ(-36px);
}

:global(.dark) .globe {
  border-color: rgba(56, 189, 248, 0.28);
  background:
    radial-gradient(circle at 40% 36%, rgba(125, 211, 252, 0.16), transparent 14%),
    radial-gradient(circle, rgba(14, 165, 233, 0.12), transparent 68%);
}

.globe::before,
.globe::after,
.globe-line {
  content: '';
  position: absolute;
  inset: 12%;
  border: 1px solid rgba(14, 165, 233, 0.18);
  border-radius: 50%;
}

.globe::before { transform: scaleX(0.46); }
.globe::after { transform: scaleY(0.42); }
.globe-line-one { transform: rotate(32deg) scaleX(0.48); }
.globe-line-two { transform: rotate(-32deg) scaleX(0.48); }
.globe-line-three { inset: 34% 3%; }

.orbit {
  position: absolute;
  left: 50%;
  top: 49%;
  border: 1px solid rgba(20, 184, 166, 0.34);
  border-radius: 50%;
  transform-style: preserve-3d;
}

.orbit-one {
  width: 420px;
  height: 152px;
  animation: orbit-spin 16s linear infinite;
}

.orbit-two {
  width: 360px;
  height: 116px;
  border-color: rgba(59, 130, 246, 0.28);
  animation: orbit-spin-reverse 21s linear infinite;
}

.orbit-three {
  width: 286px;
  height: 88px;
  border-style: dashed;
  border-color: rgba(168, 85, 247, 0.28);
  animation: orbit-spin 26s linear infinite;
}

.orbit-node {
  position: absolute;
  width: 12px;
  height: 12px;
  border: 2px solid rgba(255, 255, 255, 0.8);
  border-radius: 50%;
  background: #14b8a6;
  box-shadow: 0 0 16px #14b8a6;
}

.node-one { left: 12%; top: 16%; }
.node-two { right: 8%; bottom: 18%; background: #38bdf8; box-shadow: 0 0 16px #38bdf8; }
.node-three { right: 18%; top: 2%; background: #a855f7; box-shadow: 0 0 16px #a855f7; }

.core-platform {
  position: absolute;
  left: 50%;
  top: 55%;
  width: 260px;
  height: 82px;
  transform: translate(-50%, -50%) translateZ(34px);
}

.platform-ring {
  position: absolute;
  inset: 0;
  border: 1px solid rgba(45, 212, 191, 0.54);
  border-radius: 50%;
  background: radial-gradient(ellipse, rgba(20, 184, 166, 0.3), transparent 62%);
  box-shadow: 0 0 36px rgba(20, 184, 166, 0.32), inset 0 0 25px rgba(20, 184, 166, 0.18);
}

.platform-ring-two {
  inset: 14px 34px;
  border-color: rgba(56, 189, 248, 0.7);
  animation: platform-pulse 2.8s ease-in-out infinite;
}

.platform-beam {
  position: absolute;
  left: 50%;
  bottom: 36px;
  width: 120px;
  height: 160px;
  background: linear-gradient(to top, rgba(20, 184, 166, 0.25), transparent);
  clip-path: polygon(28% 100%, 72% 100%, 100% 0, 0 0);
  filter: blur(6px);
  transform: translateX(-50%);
}

.cube-wrap {
  position: absolute;
  left: 50%;
  top: 43%;
  width: 112px;
  height: 112px;
  transform: translate(-50%, -50%) translateZ(80px);
  transform-style: preserve-3d;
}

.cube {
  position: relative;
  width: 100%;
  height: 100%;
  transform-style: preserve-3d;
  animation: cube-float 5.4s ease-in-out infinite;
}

.cube-face {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  overflow: hidden;
  border: 1px solid rgba(153, 246, 228, 0.68);
  background: linear-gradient(145deg, rgba(13, 148, 136, 0.92), rgba(3, 105, 161, 0.88));
  box-shadow: inset 0 0 30px rgba(255, 255, 255, 0.12), 0 0 26px rgba(20, 184, 166, 0.28);
  backface-visibility: hidden;
}

.cube-front { transform: translateZ(56px); }
.cube-back { transform: rotateY(180deg) translateZ(56px); }
.cube-right { transform: rotateY(90deg) translateZ(56px); }
.cube-left { transform: rotateY(-90deg) translateZ(56px); }
.cube-top { transform: rotateX(90deg) translateZ(56px); }
.cube-bottom { transform: rotateX(-90deg) translateZ(56px); }

.cube-front span {
  color: white;
  font-size: 46px;
  font-weight: 800;
  text-shadow: 0 0 20px rgba(255, 255, 255, 0.5);
}

.cube-right {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 25px 20px;
}

.cube-right i {
  width: 100%;
  height: 6px;
  border-radius: 999px;
  background: rgba(207, 250, 254, 0.82);
  box-shadow: 0 0 8px rgba(207, 250, 254, 0.6);
}

.service-float {
  position: absolute;
  z-index: 4;
  display: flex;
  min-width: 164px;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  border: 1px solid rgba(15, 118, 110, 0.18);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.76);
  box-shadow: 0 14px 36px rgba(15, 23, 42, 0.12);
  backdrop-filter: blur(14px);
  transform: translate3d(var(--scene-shift-x), var(--scene-shift-y), 70px);
  animation: card-float 5.8s ease-in-out infinite;
}

:global(.dark) .service-float {
  border-color: rgba(94, 234, 212, 0.18);
  background: rgba(8, 20, 38, 0.76);
  box-shadow: 0 18px 42px rgba(0, 0, 0, 0.36);
}

.service-float-1 { left: 5%; top: 13%; animation-delay: -1s; }
.service-float-2 { right: 5%; top: 12%; animation-delay: -3.2s; }
.service-float-3 { left: 1.5%; bottom: 18%; animation-delay: -2.2s; }
.service-float-4 { right: 2%; bottom: 22%; animation-delay: -4.4s; }
.service-float-5 { right: 8%; top: 48%; animation-delay: -5.1s; }

.service-float-icon {
  display: grid;
  width: 34px;
  height: 34px;
  flex: 0 0 auto;
  place-items: center;
  border-radius: 10px;
  color: #0f766e;
  background: rgba(20, 184, 166, 0.14);
}

.service-float[data-tone='blue'] .service-float-icon { color: #2563eb; background: rgba(59, 130, 246, 0.14); }
.service-float[data-tone='violet'] .service-float-icon { color: #7c3aed; background: rgba(139, 92, 246, 0.14); }
.service-float[data-tone='cyan'] .service-float-icon { color: #0891b2; background: rgba(6, 182, 212, 0.14); }
.service-float[data-tone='amber'] .service-float-icon { color: #d97706; background: rgba(245, 158, 11, 0.14); }

.service-float strong,
.service-float small { display: block; }
.service-float strong { color: #0f172a; font-size: 13px; line-height: 1.35; }
.service-float small { margin-top: 2px; color: #64748b; font-size: 10px; white-space: nowrap; }
:global(.dark) .service-float strong { color: #f8fafc; }
:global(.dark) .service-float small { color: #94a3b8; }

.scene-dashboard {
  position: absolute;
  right: 4%;
  bottom: 5%;
  width: 154px;
  height: 84px;
  padding: 11px;
  border: 1px solid rgba(14, 165, 233, 0.18);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.64);
  box-shadow: 0 14px 30px rgba(15, 23, 42, 0.1);
  backdrop-filter: blur(14px);
  transform: translateZ(32px);
}

:global(.dark) .scene-dashboard {
  background: rgba(5, 16, 31, 0.72);
  box-shadow: 0 14px 34px rgba(0, 0, 0, 0.32);
}

.dashboard-head { display: flex; gap: 4px; }
.dashboard-head span { width: 4px; height: 4px; border-radius: 50%; background: #14b8a6; }
.dashboard-bars { position: absolute; left: 12px; bottom: 12px; display: flex; height: 44px; align-items: end; gap: 5px; }
.dashboard-bars i { width: 5px; border-radius: 3px 3px 0 0; background: linear-gradient(to top, #0f766e, #5eead4); }
.dashboard-bars i:nth-child(1) { height: 30%; }
.dashboard-bars i:nth-child(2) { height: 58%; }
.dashboard-bars i:nth-child(3) { height: 42%; }
.dashboard-bars i:nth-child(4) { height: 78%; }
.dashboard-bars i:nth-child(5) { height: 62%; }
.dashboard-line { position: absolute; right: 10px; bottom: 11px; width: 82px; height: 42px; }
.dashboard-line path { fill: none; stroke: #38bdf8; stroke-width: 2; filter: drop-shadow(0 0 4px rgba(56, 189, 248, 0.6)); }

.scene-status {
  position: absolute;
  left: 5%;
  bottom: 5%;
  display: flex;
  align-items: center;
  gap: 7px;
  color: #64748b;
  font-size: 9px;
  letter-spacing: 0.12em;
}

.scene-status strong { color: #0f766e; font-size: 11px; letter-spacing: 0; }
.status-dot { width: 7px; height: 7px; border-radius: 50%; background: #10b981; box-shadow: 0 0 10px #10b981; animation: status-pulse 1.8s ease-in-out infinite; }

@keyframes orbit-spin {
  from { transform: translate(-50%, -50%) rotateZ(0deg) rotateX(66deg); }
  to { transform: translate(-50%, -50%) rotateZ(360deg) rotateX(66deg); }
}

@keyframes orbit-spin-reverse {
  from { transform: translate(-50%, -50%) rotateZ(360deg) rotateX(68deg); }
  to { transform: translate(-50%, -50%) rotateZ(0deg) rotateX(68deg); }
}

@keyframes cube-float {
  0%, 100% { transform: rotateX(-16deg) rotateY(30deg) translateY(0); }
  50% { transform: rotateX(-10deg) rotateY(44deg) translateY(-12px); }
}

@keyframes card-float {
  0%, 100% { margin-top: 0; }
  50% { margin-top: -8px; }
}

@keyframes platform-pulse {
  0%, 100% { opacity: 0.72; transform: scale(0.96); }
  50% { opacity: 1; transform: scale(1.08); }
}

@keyframes status-pulse {
  0%, 100% { opacity: 0.55; }
  50% { opacity: 1; }
}

@media (max-width: 1023px) {
  .hero-orbit-stage { min-height: 500px; }
}

@media (max-width: 640px) {
  .hero-orbit-stage { min-height: 430px; }
  .scene-shell { border-radius: 24px; }
  .globe { width: 270px; height: 270px; }
  .orbit-one { width: 330px; }
  .orbit-two { width: 290px; }
  .service-float { min-width: 0; padding: 9px; }
  .service-float small { display: none; }
  .service-float-1 { left: 3%; top: 8%; }
  .service-float-2 { right: 3%; top: 10%; }
  .service-float-3 { left: 3%; bottom: 16%; }
  .service-float-4 { right: 3%; bottom: 17%; }
  .service-float-5 { display: none; }
  .scene-dashboard { right: 5%; bottom: 4%; width: 132px; opacity: 0.72; }
  .scene-status { display: none; }
}

@media (prefers-reduced-motion: reduce) {
  .scene-shell { transition: none; }
  .orbit,
  .cube,
  .service-float,
  .platform-ring-two,
  .status-dot { animation: none !important; }
}
</style>
