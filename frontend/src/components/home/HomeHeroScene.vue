<template>
  <div
    ref="stageRef"
    class="hero-orbit-stage"
    role="img"
    aria-label="ModuRelay Relay Core"
  >
    <div class="scene-shell" :class="{ 'scene-is-loading': isLoading, 'scene-has-fallback': hasFallback }">
      <div ref="canvasHostRef" class="three-canvas-host" aria-hidden="true"></div>

      <div v-if="isLoading" class="scene-loading" aria-hidden="true">
        <span></span>
        <small>INITIALIZING 3D NETWORK</small>
      </div>

      <div v-if="hasFallback" class="fallback-core" aria-hidden="true">
        <div class="fallback-ring fallback-ring-one"></div>
        <div class="fallback-ring fallback-ring-two"></div>
        <span class="fallback-bubble fallback-bubble-one"></span>
        <span class="fallback-bubble fallback-bubble-two"></span>
        <span class="fallback-bubble fallback-bubble-three"></span>
        <span class="fallback-bubble fallback-bubble-four"></span>
        <div class="fallback-cube"><span>M</span></div>
      </div>

    </div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { isDarkTheme, loadThree, observeTheme } from '@/utils/threeRuntime'

type Disposable = { dispose: () => void }
type OrbitChild = {
  userData: { speed?: number }
  rotation: { z: number }
}

const stageRef = ref<HTMLElement | null>(null)
const canvasHostRef = ref<HTMLElement | null>(null)
const isLoading = ref(true)
const hasFallback = ref(false)
let cleanupScene: (() => void) | null = null
let sceneToken = 0

function supportsWebGL(): boolean {
  try {
    const canvas = document.createElement('canvas')
    return Boolean(canvas.getContext('webgl2') || canvas.getContext('webgl'))
  } catch {
    return false
  }
}

onMounted(() => {
  const token = ++sceneToken
  const stage = stageRef.value
  const host = canvasHostRef.value

  if (!stage || !host || !supportsWebGL()) {
    isLoading.value = false
    hasFallback.value = true
    return
  }

  void initializeScene(stage, host)
    .then((cleanup) => {
      if (token !== sceneToken) {
        cleanup()
        return
      }
      cleanupScene = cleanup
      isLoading.value = false
    })
    .catch((error) => {
      if (token !== sceneToken) return
      console.warn('[HomeHeroScene] Three.js scene failed, using fallback.', error)
      isLoading.value = false
      hasFallback.value = true
    })
})

onBeforeUnmount(() => {
  sceneToken += 1
  cleanupScene?.()
  cleanupScene = null
})

function createBrandTexture(THREE: any, dark: boolean) {
  const canvas = document.createElement('canvas')
  canvas.width = 512
  canvas.height = 512
  const context = canvas.getContext('2d')

  if (!context) throw new Error('Unable to create brand texture canvas')

  const texture = new THREE.CanvasTexture(canvas)
  texture.colorSpace = THREE.SRGBColorSpace
  texture.anisotropy = 8

  const redraw = (isDark: boolean) => {
    const gradient = context.createLinearGradient(0, 0, 512, 512)
    gradient.addColorStop(0, isDark ? '#3730a3' : '#4f46e5')
    gradient.addColorStop(0.55, isDark ? '#155e75' : '#0891b2')
    gradient.addColorStop(1, isDark ? '#1e1b4b' : '#6366f1')

    context.clearRect(0, 0, 512, 512)
    context.fillStyle = gradient
    context.fillRect(0, 0, 512, 512)

    context.strokeStyle = isDark ? 'rgba(165, 243, 252, 0.78)' : 'rgba(255, 255, 255, 0.72)'
    context.lineWidth = 12
    context.strokeRect(18, 18, 476, 476)

    context.fillStyle = isDark ? '#f8fafc' : '#ffffff'
    context.textAlign = 'center'
    context.textBaseline = 'middle'
    context.font = '900 252px Inter, system-ui, sans-serif'
    context.shadowColor = isDark ? 'rgba(103, 232, 249, 0.65)' : 'rgba(8, 145, 178, 0.35)'
    context.shadowBlur = 34
    context.fillText('M', 256, 246)

    context.shadowBlur = 0
    context.fillStyle = isDark ? 'rgba(226, 232, 240, 0.82)' : 'rgba(255, 255, 255, 0.9)'
    context.font = '700 28px Inter, system-ui, sans-serif'
    context.letterSpacing = '8px'
    context.fillText('MODURELAY', 256, 414)

    texture.needsUpdate = true
  }

  redraw(dark)
  return { texture, redraw }
}

async function initializeScene(stage: HTMLElement, host: HTMLElement): Promise<() => void> {
  const THREE = await loadThree()
  let stopped = false
  let animationFrame = 0
  let visible = true
  let pointerTargetX = 0
  let pointerTargetY = 0
  let pointerX = 0
  let pointerY = 0
  let scrollTarget = 0
  let scrollValue = 0
  let lastTime = performance.now()

  const resources: Disposable[] = []
  const ringMaterials: Array<{ color: { setHex: (value: number) => void }; opacity: number }> = []
  const bubbleMaterials: Array<{ material: any; darkColor: number; lightColor: number; darkOpacity: number; lightOpacity: number }> = []

  const renderer = new THREE.WebGLRenderer({
    alpha: true,
    antialias: window.devicePixelRatio <= 1.75,
    powerPreference: 'high-performance',
    premultipliedAlpha: true
  })
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, window.innerWidth < 768 ? 1.25 : 1.5))
  renderer.outputColorSpace = THREE.SRGBColorSpace
  renderer.toneMapping = THREE.ACESFilmicToneMapping
  renderer.toneMappingExposure = isDarkTheme() ? 1.34 : 1.04
  renderer.setClearColor(0x000000, 0)
  renderer.domElement.className = 'three-hero-canvas'
  renderer.domElement.setAttribute('aria-hidden', 'true')
  host.appendChild(renderer.domElement)

  const scene = new THREE.Scene()
  scene.fog = new THREE.FogExp2(isDarkTheme() ? 0x090c12 : 0xf5f7fb, isDarkTheme() ? 0.036 : 0.028)

  const camera = new THREE.PerspectiveCamera(42, 1, 0.1, 80)
  camera.position.set(0, 0.65, 10.6)

  const world = new THREE.Group()
  const coreGroup = new THREE.Group()
  const orbitGroup = new THREE.Group()
  const bubbleGroup = new THREE.Group()
  scene.add(world)
  world.add(coreGroup, orbitGroup, bubbleGroup)

  const ambientLight = new THREE.HemisphereLight(0xd9ddff, 0x071426, isDarkTheme() ? 1.55 : 1.65)
  scene.add(ambientLight)

  const keyLight = new THREE.DirectionalLight(0xffffff, isDarkTheme() ? 2.75 : 3)
  keyLight.position.set(4, 6, 7)
  scene.add(keyLight)

  const primaryLight = new THREE.PointLight(0x6366f1, 34, 18, 2)
  primaryLight.position.set(-3.5, 1.8, 3.5)
  scene.add(primaryLight)

  const blueLight = new THREE.PointLight(0x06b6d4, 30, 18, 2)
  blueLight.position.set(4, -1.2, 2.8)
  scene.add(blueLight)

  const violetLight = new THREE.PointLight(0x818cf8, 18, 14, 2)
  violetLight.position.set(0, 4, -1)
  scene.add(violetLight)

  const brandTexture = createBrandTexture(THREE, isDarkTheme())
  resources.push(brandTexture.texture)

  const cubeGeometry = new THREE.BoxGeometry(2.25, 2.25, 2.25, 2, 2, 2)
  resources.push(cubeGeometry)

  const cubeMaterials = Array.from({ length: 6 }, () => new THREE.MeshPhysicalMaterial({
    map: brandTexture.texture,
    emissiveMap: brandTexture.texture,
    emissive: isDarkTheme() ? 0x1e1b4b : 0x312e81,
    emissiveIntensity: isDarkTheme() ? 0.62 : 0.18,
    metalness: 0.5,
    roughness: 0.26,
    clearcoat: 1,
    clearcoatRoughness: 0.12,
    transparent: false
  }))
  resources.push(...cubeMaterials)

  const cube = new THREE.Mesh(cubeGeometry, cubeMaterials)
  cube.rotation.set(-0.22, 0.56, 0.04)
  coreGroup.add(cube)

  const edgeGeometry = new THREE.EdgesGeometry(cubeGeometry, 18)
  const edgeMaterial = new THREE.LineBasicMaterial({
    color: isDarkTheme() ? 0xa5b4fc : 0x4338ca,
    transparent: true,
    opacity: isDarkTheme() ? 0.85 : 0.52,
    blending: THREE.AdditiveBlending
  })
  resources.push(edgeGeometry, edgeMaterial)
  const cubeEdges = new THREE.LineSegments(edgeGeometry, edgeMaterial)
  cubeEdges.scale.setScalar(1.012)
  coreGroup.add(cubeEdges)

  const innerGeometry = new THREE.IcosahedronGeometry(0.9, 1)
  const innerMaterial = new THREE.MeshBasicMaterial({
    color: isDarkTheme() ? 0x67e8f9 : 0x0891b2,
    wireframe: true,
    transparent: true,
    opacity: isDarkTheme() ? 0.24 : 0.12,
    blending: THREE.AdditiveBlending
  })
  resources.push(innerGeometry, innerMaterial)
  const innerCore = new THREE.Mesh(innerGeometry, innerMaterial)
  coreGroup.add(innerCore)

  const bubbleGeometry = new THREE.SphereGeometry(1, 24, 18)
  resources.push(bubbleGeometry)
  const bubbleConfigs = [
    { x: -3.2, y: 1.65, z: 0.4, size: 0.3, phase: 0.4, darkColor: 0x818cf8, lightColor: 0x6366f1, darkOpacity: 0.62, lightOpacity: 0.42 },
    { x: 3.28, y: 1.38, z: 0.1, size: 0.24, phase: 2.2, darkColor: 0x22d3ee, lightColor: 0x0891b2, darkOpacity: 0.58, lightOpacity: 0.38 },
    { x: -3.55, y: -0.85, z: 0.35, size: 0.2, phase: 4.5, darkColor: 0xa78bfa, lightColor: 0x8b5cf6, darkOpacity: 0.54, lightOpacity: 0.34 },
    { x: 3.65, y: -0.72, z: 0.65, size: 0.34, phase: 5.7, darkColor: 0x67e8f9, lightColor: 0x06b6d4, darkOpacity: 0.58, lightOpacity: 0.4 },
  ]
  const bubbles = bubbleConfigs.map((config) => {
    const material = new THREE.MeshPhysicalMaterial({
      color: isDarkTheme() ? config.darkColor : config.lightColor,
      transparent: true,
      opacity: isDarkTheme() ? config.darkOpacity : config.lightOpacity,
      roughness: 0.08,
      metalness: 0.06,
      transmission: 0.22,
      thickness: 0.32,
      clearcoat: 1,
      clearcoatRoughness: 0.08,
      depthWrite: false,
    })
    resources.push(material)
    bubbleMaterials.push({ material, darkColor: config.darkColor, lightColor: config.lightColor, darkOpacity: config.darkOpacity, lightOpacity: config.lightOpacity })
    const mesh = new THREE.Mesh(bubbleGeometry, material)
    mesh.position.set(config.x, config.y, config.z)
    mesh.scale.setScalar(config.size)
    bubbleGroup.add(mesh)
    return { mesh, config }
  })

  const platformGeometry = new THREE.CylinderGeometry(2.55, 3.15, 0.22, 96, 1, true)
  const platformMaterial = new THREE.MeshPhysicalMaterial({
    color: isDarkTheme() ? 0x172554 : 0xe0e7ff,
    emissive: isDarkTheme() ? 0x3730a3 : 0x4f46e5,
    emissiveIntensity: isDarkTheme() ? 0.68 : 0.14,
    metalness: 0.72,
    roughness: 0.25,
    transparent: true,
    opacity: isDarkTheme() ? 0.68 : 0.45,
    side: THREE.DoubleSide
  })
  resources.push(platformGeometry, platformMaterial)
  const platform = new THREE.Mesh(platformGeometry, platformMaterial)
  platform.position.y = -1.82
  coreGroup.add(platform)

  for (let index = 0; index < 5; index += 1) {
    const geometry = new THREE.TorusGeometry(2.2 + index * 0.48, 0.017 + index * 0.003, 8, 180)
    const material = new THREE.MeshBasicMaterial({
      color: index % 3 === 0 ? 0x6366f1 : index % 3 === 1 ? 0x06b6d4 : 0x818cf8,
      transparent: true,
      opacity: isDarkTheme() ? 0.46 - index * 0.05 : 0.24 - index * 0.026,
      depthWrite: false,
      blending: THREE.AdditiveBlending
    })
    resources.push(geometry, material)
    ringMaterials.push(material)
    const ring = new THREE.Mesh(geometry, material)
    ring.rotation.x = Math.PI / 2.25 + index * 0.11
    ring.rotation.y = index * 0.24
    ring.rotation.z = index * 0.33
    ring.userData.speed = (index % 2 === 0 ? 1 : -1) * (0.08 + index * 0.018)
    orbitGroup.add(ring)
  }

  const knotGeometry = new THREE.TorusKnotGeometry(2.28, 0.022, 220, 6, 2, 5)
  const knotMaterial = new THREE.MeshBasicMaterial({
    color: isDarkTheme() ? 0x22d3ee : 0x0891b2,
    transparent: true,
    opacity: isDarkTheme() ? 0.2 : 0.1,
    blending: THREE.AdditiveBlending
  })
  resources.push(knotGeometry, knotMaterial)
  const energyKnot = new THREE.Mesh(knotGeometry, knotMaterial)
  energyKnot.rotation.x = 0.58
  orbitGroup.add(energyKnot)

  const updateTheme = (dark: boolean) => {
    renderer.toneMappingExposure = dark ? 1.34 : 1.04
    scene.fog.color.setHex(dark ? 0x090c12 : 0xf5f7fb)
    scene.fog.density = dark ? 0.036 : 0.028
    ambientLight.intensity = dark ? 1.55 : 1.65
    keyLight.intensity = dark ? 2.75 : 3
    brandTexture.redraw(dark)
    cubeMaterials.forEach((material: any) => {
      material.emissive.setHex(dark ? 0x1e1b4b : 0x312e81)
      material.emissiveIntensity = dark ? 0.62 : 0.18
      material.needsUpdate = true
    })
    edgeMaterial.color.setHex(dark ? 0xa5b4fc : 0x4338ca)
    edgeMaterial.opacity = dark ? 0.85 : 0.52
    innerMaterial.color.setHex(dark ? 0x67e8f9 : 0x0891b2)
    innerMaterial.opacity = dark ? 0.24 : 0.12
    platformMaterial.color.setHex(dark ? 0x172554 : 0xe0e7ff)
    platformMaterial.emissive.setHex(dark ? 0x3730a3 : 0x4f46e5)
    platformMaterial.emissiveIntensity = dark ? 0.68 : 0.14
    platformMaterial.opacity = dark ? 0.68 : 0.45
    knotMaterial.color.setHex(dark ? 0x22d3ee : 0x0891b2)
    knotMaterial.opacity = dark ? 0.2 : 0.1
    bubbleMaterials.forEach(({ material, darkColor, lightColor, darkOpacity, lightOpacity }) => {
      material.color.setHex(dark ? darkColor : lightColor)
      material.opacity = dark ? darkOpacity : lightOpacity
      material.needsUpdate = true
    })
    ringMaterials.forEach((material, index) => {
      const darkColors = [0x6366f1, 0x06b6d4, 0x818cf8]
      const lightColors = [0x4f46e5, 0x0891b2, 0x6366f1]
      material.color.setHex((dark ? darkColors : lightColors)[index % 3])
      material.opacity = dark ? 0.46 - index * 0.05 : 0.24 - index * 0.026
    })
  }
  const stopThemeObserver = observeTheme(updateTheme)

  const resize = () => {
    const rect = host.getBoundingClientRect()
    const width = Math.max(rect.width, 1)
    const height = Math.max(rect.height, 1)
    camera.aspect = width / height
    camera.updateProjectionMatrix()
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, window.innerWidth < 768 ? 1.25 : 1.5))
    renderer.setSize(width, height, false)
  }
  const resizeObserver = new ResizeObserver(resize)
  resizeObserver.observe(host)
  resize()

  const handlePointerMove = (event: PointerEvent) => {
    const rect = stage.getBoundingClientRect()
    pointerTargetX = (event.clientX - rect.left) / Math.max(rect.width, 1) - 0.5
    pointerTargetY = (event.clientY - rect.top) / Math.max(rect.height, 1) - 0.5
  }

  const handlePointerLeave = () => {
    pointerTargetX = 0
    pointerTargetY = 0
  }

  const handleScroll = () => {
    const rect = stage.getBoundingClientRect()
    scrollTarget = Math.max(-1, Math.min(1, -rect.top / Math.max(window.innerHeight, 1)))
  }

  stage.addEventListener('pointermove', handlePointerMove, { passive: true })
  stage.addEventListener('pointerleave', handlePointerLeave)
  window.addEventListener('scroll', handleScroll, { passive: true })
  handleScroll()

  const intersectionObserver = new IntersectionObserver((entries) => {
    visible = entries.some((entry) => entry.isIntersecting)
    lastTime = performance.now()
  }, { rootMargin: '180px' })
  intersectionObserver.observe(stage)

  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  const clock = new THREE.Clock()

  const animate = (time: number) => {
    if (stopped) return
    animationFrame = window.requestAnimationFrame(animate)

    if (!visible || document.hidden) {
      lastTime = time
      return
    }

    const delta = Math.min((time - lastTime) / 1000, 0.05)
    lastTime = time
    const elapsed = clock.getElapsedTime()
    pointerX += (pointerTargetX - pointerX) * Math.min(1, delta * 4.8)
    pointerY += (pointerTargetY - pointerY) * Math.min(1, delta * 4.8)
    scrollValue += (scrollTarget - scrollValue) * Math.min(1, delta * 3.4)

    if (!reducedMotion) {
      cube.rotation.x = -0.22 + Math.sin(elapsed * 0.52) * 0.06 + pointerY * 0.16
      cube.rotation.y = 0.56 + elapsed * 0.17 + pointerX * 0.42
      cube.position.y = Math.sin(elapsed * 1.1) * 0.1
      cubeEdges.rotation.copy(cube.rotation)
      cubeEdges.position.copy(cube.position)
      innerCore.rotation.x = elapsed * 0.18
      innerCore.rotation.y = -elapsed * 0.3
      energyKnot.rotation.z = elapsed * 0.08
      energyKnot.rotation.y = -elapsed * 0.06
      bubbles.forEach(({ mesh, config }) => {
        const floatX = Math.sin(elapsed * 0.64 + config.phase) * 0.09
        const floatY = Math.cos(elapsed * 0.82 + config.phase) * 0.14
        const floatZ = Math.sin(elapsed * 0.46 + config.phase) * 0.12
        const pulse = 1 + Math.sin(elapsed * 1.12 + config.phase) * 0.06
        mesh.position.set(config.x + floatX, config.y + floatY, config.z + floatZ)
        mesh.scale.setScalar(config.size * pulse)
        mesh.rotation.x += delta * 0.24
        mesh.rotation.y -= delta * 0.32
      })

      orbitGroup.children.forEach((child: OrbitChild) => {
        if (typeof child.userData.speed === 'number') {
          child.rotation.z += child.userData.speed * delta
        }
      })

    }

    world.rotation.x = pointerY * -0.1 + scrollValue * 0.04
    world.rotation.y = pointerX * 0.16 + scrollValue * 0.16
    world.position.y = -scrollValue * 0.28
    camera.position.x = pointerX * 0.72
    camera.position.y = 0.65 - pointerY * 0.54 + scrollValue * 0.18
    camera.position.z = 10.6 + Math.abs(scrollValue) * 0.28
    camera.lookAt(0, 0, 0)
    renderer.render(scene, camera)
  }

  if (reducedMotion) renderer.render(scene, camera)
  else animationFrame = window.requestAnimationFrame(animate)

  return () => {
    stopped = true
    window.cancelAnimationFrame(animationFrame)
    stage.removeEventListener('pointermove', handlePointerMove)
    stage.removeEventListener('pointerleave', handlePointerLeave)
    window.removeEventListener('scroll', handleScroll)
    intersectionObserver.disconnect()
    resizeObserver.disconnect()
    stopThemeObserver()
    resources.forEach((resource) => resource.dispose())
    renderer.dispose()
    renderer.domElement.remove()
  }
}
</script>

<style scoped>
.hero-orbit-stage {
  position: relative;
  width: min(100%, 720px);
  min-height: 540px;
  perspective: 1200px;
  isolation: isolate;
}

.scene-shell {
  position: absolute;
  inset: 0;
  overflow: hidden;
  background: transparent;
}

.three-canvas-host,
:deep(.three-hero-canvas) {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}

.three-canvas-host { z-index: 2; }
:deep(.three-hero-canvas) { display: block; touch-action: pan-y; }

.scene-loading {
  position: absolute;
  z-index: 10;
  inset: 0;
  display: grid;
  place-content: center;
  justify-items: center;
  gap: 14px;
  color: var(--mr-primary);
  background: rgba(255, 255, 255, 0.18);
  backdrop-filter: blur(4px);
}

:global(.dark) .scene-loading { color: var(--mr-primary); background: rgba(7, 9, 16, 0.18); }
.scene-loading span { width: 42px; height: 42px; border: 2px solid currentColor; border-right-color: transparent; border-radius: 50%; animation: scene-loader 0.9s linear infinite; }
.scene-loading small { font-size: 9px; font-weight: 700; letter-spacing: 0.18em; }

.fallback-core { position: absolute; z-index: 3; left: 50%; top: 48%; width: 270px; height: 270px; transform: translate(-50%, -50%); }
.fallback-ring { position: absolute; inset: 14%; border: 1px solid color-mix(in srgb, var(--mr-primary) 55%, transparent); border-radius: 50%; transform: rotateX(68deg); animation: fallback-spin 10s linear infinite; }
.fallback-ring-two { inset: 24% 2%; border-color: color-mix(in srgb, var(--mr-secondary) 50%, transparent); animation-direction: reverse; animation-duration: 14s; }
.fallback-bubble { position: absolute; display: block; border: 1px solid color-mix(in srgb, var(--mr-secondary) 55%, transparent); border-radius: 50%; background: radial-gradient(circle at 29% 23%, rgba(255, 255, 255, 0.78), transparent 18%), radial-gradient(circle at 67% 72%, color-mix(in srgb, var(--mr-secondary) 58%, transparent), color-mix(in srgb, var(--mr-primary) 26%, transparent) 58%, transparent 74%); box-shadow: inset -8px -10px 16px rgba(79, 70, 229, 0.16), inset 5px 5px 10px rgba(255, 255, 255, 0.35), 0 14px 24px color-mix(in srgb, var(--mr-primary) 14%, transparent); animation: fallback-bubble-float 5.8s ease-in-out infinite; }
.fallback-bubble-one { top: 22%; left: 17%; width: 36px; height: 36px; animation-delay: -1.2s; }
.fallback-bubble-two { top: 21%; right: 14%; width: 29px; height: 29px; animation-delay: -3.4s; }
.fallback-bubble-three { bottom: 27%; left: 13%; width: 22px; height: 22px; animation-delay: -4.7s; }
.fallback-bubble-four { right: 13%; bottom: 24%; width: 42px; height: 42px; animation-delay: -2.4s; }
.fallback-cube { position: absolute; left: 50%; top: 50%; display: grid; width: 110px; height: 110px; place-items: center; border: 1px solid var(--color-primary-border); color: var(--color-text-primary); background: var(--color-surface-raised); box-shadow: var(--shadow-lg); transform: translate(-50%, -50%) rotateX(-18deg) rotateY(34deg); }
.fallback-cube span { font-size: 44px; font-weight: 800; }

@keyframes scene-loader { to { transform: rotate(360deg); } }
@keyframes fallback-spin { to { transform: rotateX(68deg) rotateZ(360deg); } }
@keyframes fallback-bubble-float { 0%, 100% { transform: translate3d(0, 0, 0) scale(1); } 50% { transform: translate3d(0, -9px, 0) scale(1.08); } }

@media (max-width: 1023px) {
  .hero-orbit-stage { min-height: 510px; }
}

@media (max-width: 640px) {
  .hero-orbit-stage { min-height: 440px; }
}

@media (prefers-reduced-motion: reduce) {
  .scene-loading span,
  .fallback-ring,
  .fallback-bubble { animation: none !important; }
}
</style>
