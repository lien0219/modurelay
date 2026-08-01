<template>
  <div
    ref="stageRef"
    class="hero-orbit-stage"
    role="img"
    aria-label="ModuRelay interactive Three.js service network"
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
        <div class="fallback-cube"><span>M</span></div>
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
        <div class="dashboard-head"><span></span><span></span><span></span></div>
        <div class="dashboard-bars"><i></i><i></i><i></i><i></i><i></i></div>
        <div class="dashboard-line">
          <svg viewBox="0 0 140 42" preserveAspectRatio="none">
            <path d="M2 35 C22 32, 29 16, 45 22 S73 34, 88 17 S113 12, 138 4" />
          </svg>
        </div>
      </div>

      <div class="scene-status" aria-hidden="true">
        <span class="status-dot"></span>
        <span>THREE NETWORK ONLINE</span>
        <strong>99.99%</strong>
      </div>

      <div class="scene-interaction-hint" aria-hidden="true">
        <span></span>
        MOVE / SCROLL TO INTERACT
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { isDarkTheme, loadThree, observeTheme } from '@/utils/threeRuntime'

type HomeIcon = 'link' | 'server' | 'shield' | 'chat' | 'chart'
type ServiceTone = 'teal' | 'blue' | 'violet' | 'cyan' | 'amber'
type Disposable = { dispose: () => void }
type TrailParticle = {
  mesh: {
    position: { copy: (point: unknown) => void }
    scale: { setScalar: (value: number) => void }
  }
  curve: { getPointAt: (progress: number) => unknown }
  offset: number
  speed: number
}
type OrbitChild = {
  userData: { speed?: number }
  rotation: { z: number }
}

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
const canvasHostRef = ref<HTMLElement | null>(null)
const isLoading = ref(true)
const hasFallback = ref(false)
let cleanupScene: (() => void) | null = null

function supportsWebGL(): boolean {
  try {
    const canvas = document.createElement('canvas')
    return Boolean(canvas.getContext('webgl2') || canvas.getContext('webgl'))
  } catch {
    return false
  }
}

onMounted(() => {
  const stage = stageRef.value
  const host = canvasHostRef.value

  if (!stage || !host || !supportsWebGL()) {
    isLoading.value = false
    hasFallback.value = true
    return
  }

  void initializeScene(stage, host)
    .then((cleanup) => {
      cleanupScene = cleanup
      isLoading.value = false
    })
    .catch((error) => {
      console.warn('[HomeHeroScene] Three.js scene failed, using fallback.', error)
      isLoading.value = false
      hasFallback.value = true
    })
})

onBeforeUnmount(() => {
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
    gradient.addColorStop(0, isDark ? '#0f766e' : '#0d9488')
    gradient.addColorStop(0.55, isDark ? '#075985' : '#0284c7')
    gradient.addColorStop(1, isDark ? '#172554' : '#2563eb')

    context.clearRect(0, 0, 512, 512)
    context.fillStyle = gradient
    context.fillRect(0, 0, 512, 512)

    context.strokeStyle = isDark ? 'rgba(153, 246, 228, 0.78)' : 'rgba(255, 255, 255, 0.72)'
    context.lineWidth = 12
    context.strokeRect(18, 18, 476, 476)

    context.fillStyle = isDark ? '#f8fafc' : '#ffffff'
    context.textAlign = 'center'
    context.textBaseline = 'middle'
    context.font = '900 252px Inter, system-ui, sans-serif'
    context.shadowColor = isDark ? 'rgba(94, 234, 212, 0.65)' : 'rgba(15, 118, 110, 0.35)'
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
  const trailParticles: TrailParticle[] = []
  const trailMaterials: Array<{ opacity: number }> = []
  const ringMaterials: Array<{ color: { setHex: (value: number) => void }; opacity: number }> = []

  const renderer = new THREE.WebGLRenderer({
    alpha: true,
    antialias: window.devicePixelRatio <= 1.75,
    powerPreference: 'high-performance',
    premultipliedAlpha: true
  })
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 1.8))
  renderer.outputColorSpace = THREE.SRGBColorSpace
  renderer.toneMapping = THREE.ACESFilmicToneMapping
  renderer.toneMappingExposure = isDarkTheme() ? 1.2 : 1.04
  renderer.setClearColor(0x000000, 0)
  renderer.domElement.className = 'three-hero-canvas'
  renderer.domElement.setAttribute('aria-hidden', 'true')
  host.appendChild(renderer.domElement)

  const scene = new THREE.Scene()
  scene.fog = new THREE.FogExp2(isDarkTheme() ? 0x030914 : 0xecfdfb, isDarkTheme() ? 0.046 : 0.028)

  const camera = new THREE.PerspectiveCamera(42, 1, 0.1, 80)
  camera.position.set(0, 0.65, 10.6)

  const world = new THREE.Group()
  const coreGroup = new THREE.Group()
  const orbitGroup = new THREE.Group()
  const trailGroup = new THREE.Group()
  scene.add(world)
  world.add(coreGroup, orbitGroup, trailGroup)

  const ambientLight = new THREE.HemisphereLight(0xd5fffb, 0x071426, isDarkTheme() ? 1.25 : 1.65)
  scene.add(ambientLight)

  const keyLight = new THREE.DirectionalLight(0xffffff, isDarkTheme() ? 2.35 : 3)
  keyLight.position.set(4, 6, 7)
  scene.add(keyLight)

  const tealLight = new THREE.PointLight(0x2dd4bf, 34, 18, 2)
  tealLight.position.set(-3.5, 1.8, 3.5)
  scene.add(tealLight)

  const blueLight = new THREE.PointLight(0x38bdf8, 30, 18, 2)
  blueLight.position.set(4, -1.2, 2.8)
  scene.add(blueLight)

  const violetLight = new THREE.PointLight(0xa855f7, 18, 14, 2)
  violetLight.position.set(0, 4, -1)
  scene.add(violetLight)

  const brandTexture = createBrandTexture(THREE, isDarkTheme())
  resources.push(brandTexture.texture)

  const cubeGeometry = new THREE.BoxGeometry(2.25, 2.25, 2.25, 2, 2, 2)
  resources.push(cubeGeometry)

  const cubeMaterials = Array.from({ length: 6 }, () => new THREE.MeshPhysicalMaterial({
    map: brandTexture.texture,
    emissiveMap: brandTexture.texture,
    emissive: isDarkTheme() ? 0x063c44 : 0x032f35,
    emissiveIntensity: isDarkTheme() ? 0.48 : 0.18,
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
    color: isDarkTheme() ? 0x99f6e4 : 0x0f766e,
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
    color: isDarkTheme() ? 0x67e8f9 : 0x0284c7,
    wireframe: true,
    transparent: true,
    opacity: isDarkTheme() ? 0.24 : 0.12,
    blending: THREE.AdditiveBlending
  })
  resources.push(innerGeometry, innerMaterial)
  const innerCore = new THREE.Mesh(innerGeometry, innerMaterial)
  coreGroup.add(innerCore)

  const platformGeometry = new THREE.CylinderGeometry(2.55, 3.15, 0.22, 96, 1, true)
  const platformMaterial = new THREE.MeshPhysicalMaterial({
    color: isDarkTheme() ? 0x082f49 : 0xcffafe,
    emissive: isDarkTheme() ? 0x0f766e : 0x0d9488,
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
      color: index % 3 === 0 ? 0x2dd4bf : index % 3 === 1 ? 0x38bdf8 : 0xa855f7,
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

  const endpoints = [
    new THREE.Vector3(-4.7, 2.5, -0.8),
    new THREE.Vector3(4.7, 2.6, -1),
    new THREE.Vector3(-4.8, -2.1, -0.6),
    new THREE.Vector3(4.8, -2.2, -0.8),
    new THREE.Vector3(5.1, 0.1, -1.5)
  ]

  endpoints.forEach((end: any, index: number) => {
    const bend = new THREE.Vector3(
      end.x * 0.48,
      end.y * 0.42 + (index % 2 === 0 ? 0.65 : -0.35),
      1.2 + index * 0.08
    )
    const curve = new THREE.CatmullRomCurve3([
      new THREE.Vector3(0, 0, 0.2),
      bend,
      end
    ])

    const tubeGeometry = new THREE.TubeGeometry(curve, 72, 0.014 + index * 0.0015, 6, false)
    const tubeMaterial = new THREE.MeshBasicMaterial({
      color: index === 2 ? 0xa855f7 : index === 4 ? 0xf59e0b : index % 2 === 0 ? 0x2dd4bf : 0x38bdf8,
      transparent: true,
      opacity: isDarkTheme() ? 0.3 : 0.16,
      blending: THREE.AdditiveBlending,
      depthWrite: false
    })
    resources.push(tubeGeometry, tubeMaterial)
    trailMaterials.push(tubeMaterial)
    trailGroup.add(new THREE.Mesh(tubeGeometry, tubeMaterial))

    const beadGeometry = new THREE.SphereGeometry(0.066, 12, 12)
    const beadMaterial = new THREE.MeshBasicMaterial({
      color: index === 2 ? 0xc084fc : index === 4 ? 0xfbbf24 : index % 2 === 0 ? 0x99f6e4 : 0x7dd3fc,
      transparent: true,
      opacity: 1,
      blending: THREE.AdditiveBlending
    })
    resources.push(beadGeometry, beadMaterial)

    for (let beadIndex = 0; beadIndex < 3; beadIndex += 1) {
      const bead = new THREE.Mesh(beadGeometry, beadMaterial)
      trailGroup.add(bead)
      trailParticles.push({
        mesh: bead,
        curve,
        offset: beadIndex / 3 + index * 0.11,
        speed: 0.095 + index * 0.009
      })
    }
  })

  const shardGeometry = new THREE.TetrahedronGeometry(0.09, 0)
  const shardMaterial = new THREE.MeshBasicMaterial({
    color: isDarkTheme() ? 0x5eead4 : 0x0d9488,
    transparent: true,
    opacity: isDarkTheme() ? 0.64 : 0.32,
    blending: THREE.AdditiveBlending
  })
  resources.push(shardGeometry, shardMaterial)

  const shardCount = window.innerWidth < 768 ? 10 : 18
  const shards = new THREE.InstancedMesh(shardGeometry, shardMaterial, shardCount)
  const shardDummy = new THREE.Object3D()
  const shardStates = Array.from({ length: shardCount }, (_, index) => ({
    radius: 3 + Math.random() * 2.2,
    angle: (index / shardCount) * Math.PI * 2,
    speed: (0.1 + Math.random() * 0.16) * (index % 2 === 0 ? 1 : -1),
    height: (Math.random() - 0.5) * 3.8,
    scale: 0.65 + Math.random() * 1.05
  }))
  world.add(shards)

  const updateTheme = (dark: boolean) => {
    renderer.toneMappingExposure = dark ? 1.2 : 1.04
    scene.fog.color.setHex(dark ? 0x030914 : 0xecfdfb)
    scene.fog.density = dark ? 0.046 : 0.028
    ambientLight.intensity = dark ? 1.25 : 1.65
    keyLight.intensity = dark ? 2.35 : 3
    brandTexture.redraw(dark)
    cubeMaterials.forEach((material: any) => {
      material.emissive.setHex(dark ? 0x063c44 : 0x032f35)
      material.emissiveIntensity = dark ? 0.48 : 0.18
      material.needsUpdate = true
    })
    edgeMaterial.color.setHex(dark ? 0x99f6e4 : 0x0f766e)
    edgeMaterial.opacity = dark ? 0.85 : 0.52
    innerMaterial.color.setHex(dark ? 0x67e8f9 : 0x0284c7)
    innerMaterial.opacity = dark ? 0.24 : 0.12
    platformMaterial.color.setHex(dark ? 0x082f49 : 0xcffafe)
    platformMaterial.emissive.setHex(dark ? 0x0f766e : 0x0d9488)
    platformMaterial.emissiveIntensity = dark ? 0.68 : 0.14
    platformMaterial.opacity = dark ? 0.68 : 0.45
    knotMaterial.color.setHex(dark ? 0x22d3ee : 0x0891b2)
    knotMaterial.opacity = dark ? 0.2 : 0.1
    shardMaterial.color.setHex(dark ? 0x5eead4 : 0x0d9488)
    shardMaterial.opacity = dark ? 0.64 : 0.32
    ringMaterials.forEach((material, index) => {
      const darkColors = [0x2dd4bf, 0x38bdf8, 0xa855f7]
      const lightColors = [0x0f766e, 0x0284c7, 0x7c3aed]
      material.color.setHex((dark ? darkColors : lightColors)[index % 3])
      material.opacity = dark ? 0.46 - index * 0.05 : 0.24 - index * 0.026
    })
    trailMaterials.forEach((material) => {
      material.opacity = dark ? 0.3 : 0.16
    })
  }
  const stopThemeObserver = observeTheme(updateTheme)

  const resize = () => {
    const rect = host.getBoundingClientRect()
    const width = Math.max(rect.width, 1)
    const height = Math.max(rect.height, 1)
    camera.aspect = width / height
    camera.updateProjectionMatrix()
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 1.8))
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

      orbitGroup.children.forEach((child: OrbitChild) => {
        if (typeof child.userData.speed === 'number') {
          child.rotation.z += child.userData.speed * delta
        }
      })

      trailParticles.forEach((particle, index) => {
        const progress = (elapsed * particle.speed + particle.offset) % 1
        particle.mesh.position.copy(particle.curve.getPointAt(progress))
        particle.mesh.scale.setScalar(0.75 + Math.sin(elapsed * 4 + index) * 0.22)
      })

      shardStates.forEach((state, index) => {
        const angle = state.angle + elapsed * state.speed
        shardDummy.position.set(
          Math.cos(angle) * state.radius,
          state.height + Math.sin(elapsed * 0.9 + index) * 0.22,
          Math.sin(angle) * state.radius * 0.55
        )
        shardDummy.rotation.set(angle * 0.7, angle, angle * 0.4)
        shardDummy.scale.setScalar(state.scale)
        shardDummy.updateMatrix()
        shards.setMatrixAt(index, shardDummy.matrix)
      })
      shards.instanceMatrix.needsUpdate = true
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
  border: 1px solid rgba(15, 118, 110, 0.2);
  border-radius: 34px;
  background:
    radial-gradient(circle at 50% 45%, rgba(20, 184, 166, 0.11), transparent 34%),
    radial-gradient(circle at 88% 12%, rgba(56, 189, 248, 0.1), transparent 36%),
    linear-gradient(145deg, rgba(248, 255, 254, 0.98), rgba(234, 249, 248, 0.96));
  box-shadow:
    0 34px 94px rgba(15, 23, 42, 0.14),
    inset 0 1px 0 rgba(255, 255, 255, 0.96);
  transition: border-color 240ms ease, box-shadow 240ms ease, background 300ms ease;
}

:global(.dark) .scene-shell {
  border-color: rgba(94, 234, 212, 0.22);
  background:
    radial-gradient(circle at 50% 45%, rgba(13, 148, 136, 0.16), transparent 36%),
    radial-gradient(circle at 88% 12%, rgba(14, 165, 233, 0.11), transparent 38%),
    linear-gradient(145deg, rgba(7, 18, 36, 0.985), rgba(2, 8, 20, 0.98));
  box-shadow:
    0 38px 110px rgba(0, 0, 0, 0.52),
    inset 0 1px 0 rgba(255, 255, 255, 0.06),
    0 0 80px rgba(20, 184, 166, 0.07);
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
  color: #0f766e;
  background: rgba(255, 255, 255, 0.18);
  backdrop-filter: blur(4px);
}

:global(.dark) .scene-loading { color: #5eead4; background: rgba(2, 8, 20, 0.18); }
.scene-loading span { width: 42px; height: 42px; border: 2px solid currentColor; border-right-color: transparent; border-radius: 50%; animation: scene-loader 0.9s linear infinite; }
.scene-loading small { font-size: 9px; font-weight: 700; letter-spacing: 0.18em; }

.service-float {
  position: absolute;
  z-index: 6;
  display: flex;
  min-width: 164px;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  border: 1px solid rgba(15, 118, 110, 0.2);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.82);
  box-shadow: 0 14px 38px rgba(15, 23, 42, 0.12);
  backdrop-filter: blur(14px) saturate(125%);
  animation: service-hover 6s ease-in-out infinite;
  transition: transform 180ms ease, border-color 180ms ease, box-shadow 180ms ease;
}

:global(.dark) .service-float {
  border-color: rgba(94, 234, 212, 0.2);
  background: rgba(7, 18, 36, 0.82);
  box-shadow: 0 18px 44px rgba(0, 0, 0, 0.38), 0 0 26px rgba(20, 184, 166, 0.04);
}

.service-float:hover { transform: translateY(-5px) scale(1.04); border-color: rgba(20, 184, 166, 0.48); box-shadow: 0 22px 52px rgba(15, 23, 42, 0.18), 0 0 28px rgba(20, 184, 166, 0.13); }
.service-float-1 { left: 5%; top: 12%; animation-delay: -1s; }
.service-float-2 { right: 5%; top: 11%; animation-delay: -3.2s; }
.service-float-3 { left: 2%; bottom: 18%; animation-delay: -2.2s; }
.service-float-4 { right: 2%; bottom: 22%; animation-delay: -4.4s; }
.service-float-5 { right: 8%; top: 48%; animation-delay: -5.1s; }

.service-float-icon { display: grid; width: 34px; height: 34px; flex: 0 0 auto; place-items: center; border-radius: 10px; color: #0f766e; background: rgba(20, 184, 166, 0.14); }
.service-float[data-tone='blue'] .service-float-icon { color: #2563eb; background: rgba(59, 130, 246, 0.14); }
.service-float[data-tone='violet'] .service-float-icon { color: #7c3aed; background: rgba(139, 92, 246, 0.14); }
.service-float[data-tone='cyan'] .service-float-icon { color: #0891b2; background: rgba(6, 182, 212, 0.14); }
.service-float[data-tone='amber'] .service-float-icon { color: #d97706; background: rgba(245, 158, 11, 0.14); }
.service-float strong, .service-float small { display: block; }
.service-float strong { color: #0f172a; font-size: 13px; line-height: 1.35; }
.service-float small { margin-top: 2px; color: #64748b; font-size: 10px; white-space: nowrap; }
:global(.dark) .service-float strong { color: #f8fafc; }
:global(.dark) .service-float small { color: #94a3b8; }

.scene-dashboard {
  position: absolute;
  right: 4%;
  bottom: 5%;
  z-index: 7;
  width: 154px;
  height: 84px;
  padding: 11px;
  border: 1px solid rgba(14, 165, 233, 0.2);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.78);
  box-shadow: 0 14px 32px rgba(15, 23, 42, 0.11);
  backdrop-filter: blur(14px);
}
:global(.dark) .scene-dashboard { background: rgba(5, 16, 31, 0.82); box-shadow: 0 14px 36px rgba(0, 0, 0, 0.34); }
.dashboard-head { display: flex; gap: 4px; }
.dashboard-head span { width: 4px; height: 4px; border-radius: 50%; background: #14b8a6; }
.dashboard-bars { position: absolute; left: 12px; bottom: 12px; display: flex; height: 44px; align-items: end; gap: 5px; }
.dashboard-bars i { width: 5px; border-radius: 3px 3px 0 0; background: linear-gradient(to top, #0f766e, #5eead4); animation: dashboard-bar 2.8s ease-in-out infinite alternate; }
.dashboard-bars i:nth-child(1) { height: 30%; animation-delay: -0.2s; }
.dashboard-bars i:nth-child(2) { height: 58%; animation-delay: -1.2s; }
.dashboard-bars i:nth-child(3) { height: 42%; animation-delay: -0.7s; }
.dashboard-bars i:nth-child(4) { height: 78%; animation-delay: -1.8s; }
.dashboard-bars i:nth-child(5) { height: 62%; animation-delay: -1.4s; }
.dashboard-line { position: absolute; right: 10px; bottom: 11px; width: 82px; height: 42px; overflow: hidden; }
.dashboard-line path { fill: none; stroke: #38bdf8; stroke-width: 2; stroke-dasharray: 180; animation: dashboard-flow 3.2s linear infinite; filter: drop-shadow(0 0 4px rgba(56, 189, 248, 0.62)); }

.scene-status { position: absolute; left: 5%; bottom: 5%; z-index: 7; display: flex; align-items: center; gap: 7px; color: #64748b; font-size: 9px; letter-spacing: 0.12em; }
.scene-status strong { color: #0f766e; font-size: 11px; letter-spacing: 0; }
:global(.dark) .scene-status { color: #94a3b8; }
:global(.dark) .scene-status strong { color: #5eead4; }
.status-dot { width: 7px; height: 7px; border-radius: 50%; background: #10b981; box-shadow: 0 0 10px #10b981; animation: status-pulse 1.8s ease-in-out infinite; }

.scene-interaction-hint { position: absolute; z-index: 7; left: 50%; bottom: 5%; display: flex; align-items: center; gap: 7px; color: #64748b; font-size: 8px; font-weight: 700; letter-spacing: 0.12em; transform: translateX(-50%); opacity: 0.68; }
.scene-interaction-hint span { width: 16px; height: 1px; background: linear-gradient(90deg, transparent, #14b8a6); animation: hint-flow 1.4s ease-in-out infinite; }

.fallback-core { position: absolute; z-index: 3; left: 50%; top: 48%; width: 270px; height: 270px; transform: translate(-50%, -50%); }
.fallback-ring { position: absolute; inset: 14%; border: 1px solid rgba(20, 184, 166, 0.42); border-radius: 50%; transform: rotateX(68deg); animation: fallback-spin 10s linear infinite; }
.fallback-ring-two { inset: 24% 2%; border-color: rgba(56, 189, 248, 0.4); animation-direction: reverse; animation-duration: 14s; }
.fallback-cube { position: absolute; left: 50%; top: 50%; display: grid; width: 110px; height: 110px; place-items: center; border: 1px solid rgba(153, 246, 228, 0.6); color: white; background: linear-gradient(145deg, #0f766e, #0369a1); box-shadow: 0 0 50px rgba(20, 184, 166, 0.32); transform: translate(-50%, -50%) rotateX(-18deg) rotateY(34deg); }
.fallback-cube span { font-size: 44px; font-weight: 800; }

@keyframes scene-loader { to { transform: rotate(360deg); } }
@keyframes service-hover { 0%, 100% { margin-top: 0; } 50% { margin-top: -8px; } }
@keyframes dashboard-bar { to { transform: scaleY(0.55); transform-origin: bottom; } }
@keyframes dashboard-flow { to { stroke-dashoffset: -180; } }
@keyframes status-pulse { 0%, 100% { opacity: 0.5; } 50% { opacity: 1; } }
@keyframes hint-flow { 0%, 100% { opacity: 0.3; transform: scaleX(0.6); } 50% { opacity: 1; transform: scaleX(1.2); } }
@keyframes fallback-spin { to { transform: rotateX(68deg) rotateZ(360deg); } }

@media (max-width: 1023px) {
  .hero-orbit-stage { min-height: 510px; }
}

@media (max-width: 640px) {
  .hero-orbit-stage { min-height: 440px; }
  .scene-shell { border-radius: 24px; }
  .service-float { min-width: 0; padding: 9px; }
  .service-float small { display: none; }
  .service-float-1 { left: 3%; top: 8%; }
  .service-float-2 { right: 3%; top: 10%; }
  .service-float-3 { left: 3%; bottom: 16%; }
  .service-float-4 { right: 3%; bottom: 17%; }
  .service-float-5 { display: none; }
  .scene-dashboard { right: 4%; bottom: 4%; width: 132px; opacity: 0.78; }
  .scene-status, .scene-interaction-hint { display: none; }
}

@media (prefers-reduced-motion: reduce) {
  .service-float,
  .dashboard-bars i,
  .dashboard-line path,
  .status-dot,
  .scene-interaction-hint span,
  .fallback-ring { animation: none !important; }
}
</style>
