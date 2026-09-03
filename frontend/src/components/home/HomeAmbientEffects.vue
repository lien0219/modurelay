<template>
  <div
    ref="hostRef"
    class="home-ambient-effects"
    :data-state="renderState"
    data-effect="plum-blossoms"
    aria-hidden="true"
  ></div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { loadThree } from '@/utils/threeRuntime'

const props = withDefaults(defineProps<{ enabled?: boolean; progress?: number }>(), {
  enabled: false,
  progress: 0,
})

type ThreeModule = typeof import('three')

interface BlossomState {
  position: import('three').Vector3
  velocity: import('three').Vector3
  rotation: import('three').Euler
  spin: import('three').Vector3
  scale: number
  phase: number
  sway: number
}

const hostRef = ref<HTMLElement | null>(null)
const renderState = ref<'idle' | 'ready' | 'disabled' | 'fallback'>('disabled')

let lifecycleToken = 0
let disposeScene: (() => void) | null = null
let motionQuery: MediaQueryList | null = null
let idleHandle: number | null = null
let idleTimer = 0

function supportsWebGL(): boolean {
  try {
    const canvas = document.createElement('canvas')
    const context = canvas.getContext('webgl2') || canvas.getContext('webgl')
    context?.getExtension?.('WEBGL_lose_context')?.loseContext()
    return Boolean(context)
  } catch {
    return false
  }
}

function createBlossomGeometry(THREE: ThreeModule) {
  const positions: number[] = []
  const colors: number[] = []
  const centerColor = new THREE.Color('#eaa4b8')
  const middleColor = new THREE.Color('#f6cfda')
  const edgeColor = new THREE.Color('#fff4f6')
  const pollenColor = new THREE.Color('#f4c96d')
  const petalPoints = [
    new THREE.Vector2(-0.045, 0.035),
    new THREE.Vector2(-0.15, 0.12),
    new THREE.Vector2(-0.22, 0.27),
    new THREE.Vector2(-0.15, 0.39),
    new THREE.Vector2(-0.055, 0.43),
    new THREE.Vector2(0, 0.37),
    new THREE.Vector2(0.055, 0.43),
    new THREE.Vector2(0.15, 0.39),
    new THREE.Vector2(0.22, 0.27),
    new THREE.Vector2(0.15, 0.12),
    new THREE.Vector2(0.045, 0.035),
  ]
  const petalFaces = THREE.ShapeUtils.triangulateShape(petalPoints, [])

  const pushVertex = (x: number, y: number, z: number, color: import('three').Color) => {
    positions.push(x, y, z)
    colors.push(color.r, color.g, color.b)
  }

  for (let petal = 0; petal < 5; petal += 1) {
    const angle = petal / 5 * Math.PI * 2
    const sin = Math.sin(angle)
    const cos = Math.cos(angle)
    petalFaces.forEach((face) => {
      face.forEach((pointIndex) => {
        const point = petalPoints[pointIndex]
        const x = point.x * cos - point.y * sin
        const y = point.x * sin + point.y * cos
        const petalProgress = THREE.MathUtils.clamp((point.y - 0.02) / 0.42, 0, 1)
        const lift = Math.sin(petalProgress * Math.PI) * 0.055
        const color = centerColor.clone().lerp(middleColor, Math.min(1, petalProgress * 1.6))
        color.lerp(edgeColor, Math.max(0, petalProgress - 0.52) * 1.65)
        pushVertex(x, y, lift, color)
      })
    })
  }

  for (let segment = 0; segment < 14; segment += 1) {
    const angleA = segment / 14 * Math.PI * 2
    const angleB = (segment + 1) / 14 * Math.PI * 2
    pushVertex(0, 0, 0.06, centerColor)
    pushVertex(Math.cos(angleA) * 0.085, Math.sin(angleA) * 0.085, 0.065, middleColor)
    pushVertex(Math.cos(angleB) * 0.085, Math.sin(angleB) * 0.085, 0.065, middleColor)
  }

  for (let stamen = 0; stamen < 10; stamen += 1) {
    const angle = stamen / 10 * Math.PI * 2 + 0.16
    const tangentX = -Math.sin(angle) * 0.006
    const tangentY = Math.cos(angle) * 0.006
    const tipRadius = 0.14 + (stamen % 3) * 0.012
    const tipX = Math.cos(angle) * tipRadius
    const tipY = Math.sin(angle) * tipRadius
    pushVertex(tangentX, tangentY, 0.073, pollenColor)
    pushVertex(-tangentX, -tangentY, 0.073, pollenColor)
    pushVertex(tipX, tipY, 0.09, pollenColor)
    pushVertex(tipX - 0.012, tipY, 0.09, pollenColor)
    pushVertex(tipX + 0.012, tipY, 0.09, pollenColor)
    pushVertex(tipX, tipY + 0.018, 0.095, pollenColor)
  }

  const geometry = new THREE.BufferGeometry()
  geometry.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3))
  geometry.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3))
  geometry.computeVertexNormals()
  geometry.computeBoundingSphere()
  return geometry
}

async function initializeScene(host: HTMLElement, token: number) {
  const THREE = await loadThree() as ThreeModule
  if (token !== lifecycleToken || !host.isConnected || !props.enabled) return

  const isMobile = window.innerWidth < 720
  const renderer = new THREE.WebGLRenderer({
    alpha: true,
    antialias: false,
    powerPreference: 'low-power',
    premultipliedAlpha: true,
  })
  renderer.setClearColor(0x000000, 0)
  renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, isMobile ? 1 : 1.25))
  renderer.outputColorSpace = THREE.SRGBColorSpace
  renderer.toneMapping = THREE.ACESFilmicToneMapping
  renderer.toneMappingExposure = 1.04
  renderer.domElement.className = 'home-ambient-effects-canvas'
  host.appendChild(renderer.domElement)

  const scene = new THREE.Scene()
  const camera = new THREE.PerspectiveCamera(42, 1, 0.1, 30)
  camera.position.z = 8
  scene.add(new THREE.HemisphereLight(0xfff4f6, 0x173229, 2.1))
  const keyLight = new THREE.DirectionalLight(0xffe8ed, 1.25)
  keyLight.position.set(-3, 5, 7)
  scene.add(keyLight)

  const geometry = createBlossomGeometry(THREE)
  const material = new THREE.MeshPhysicalMaterial({
    vertexColors: true,
    transparent: true,
    opacity: isMobile ? 0.76 : 0.86,
    depthWrite: false,
    side: THREE.DoubleSide,
    roughness: 0.72,
    metalness: 0,
    clearcoat: 0.12,
    clearcoatRoughness: 0.78,
    emissive: new THREE.Color(0x3d111f),
    emissiveIntensity: 0.12,
  })
  const blossomCount = isMobile ? 6 : 12
  const blossomMesh = new THREE.InstancedMesh(geometry, material, blossomCount)
  blossomMesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage)
  blossomMesh.frustumCulled = false
  scene.add(blossomMesh)

  const dummy = new THREE.Object3D()
  const blossoms: BlossomState[] = []
  let halfWidth = 1
  let halfHeight = 1
  let animationFrame = 0
  let resizeFrame = 0
  let disposed = false
  let visible = !document.hidden
  let lastFrame = performance.now()
  let elapsed = Math.random() * 20
  let lastProgress = Number(props.progress) || 0
  let scrollGust = 0

  const randomBetween = (minimum: number, maximum: number) => minimum + Math.random() * (maximum - minimum)

  const resetBlossom = (state: BlossomState, initial = false) => {
    state.position.set(
      randomBetween(-halfWidth * 1.08, halfWidth * 1.08),
      initial ? randomBetween(-halfHeight * 1.08, halfHeight * 1.16) : halfHeight + randomBetween(0.3, 2.6),
      randomBetween(-1.35, 1.45),
    )
    state.velocity.set(randomBetween(-0.045, 0.045), randomBetween(-0.24, -0.13), 0)
    state.rotation.set(Math.random() * Math.PI, Math.random() * Math.PI, Math.random() * Math.PI)
    state.spin.set(randomBetween(-0.48, 0.48), randomBetween(-0.42, 0.42), randomBetween(-0.36, 0.36))
    state.scale = randomBetween(isMobile ? 0.24 : 0.27, isMobile ? 0.42 : 0.49)
    state.phase = Math.random() * Math.PI * 2
    state.sway = randomBetween(0.75, 1.35)
  }

  const updateViewport = () => {
    const width = Math.max(host.clientWidth || window.innerWidth, 1)
    const height = Math.max(host.clientHeight || window.innerHeight, 1)
    renderer.setSize(width, height, false)
    camera.aspect = width / height
    camera.updateProjectionMatrix()
    halfHeight = Math.tan(THREE.MathUtils.degToRad(camera.fov * 0.5)) * camera.position.z
    halfWidth = halfHeight * camera.aspect
  }

  const scheduleResize = () => {
    if (resizeFrame) return
    resizeFrame = requestAnimationFrame(() => {
      resizeFrame = 0
      updateViewport()
    })
  }

  const render = (now: number) => {
    animationFrame = 0
    if (disposed || !visible) return

    const frameInterval = 1000 / (isMobile ? 30 : 45)
    if (now - lastFrame < frameInterval) {
      animationFrame = requestAnimationFrame(render)
      return
    }

    const delta = Math.min((now - lastFrame) / 1000, 0.05)
    lastFrame = now
    elapsed += delta
    const progress = Number(props.progress) || 0
    const progressVelocity = Math.min(1, Math.abs(progress - lastProgress) / Math.max(delta, 0.001))
    lastProgress = progress
    scrollGust += (progressVelocity - scrollGust) * (1 - Math.exp(-delta * 4.2))

    blossoms.forEach((blossom, index) => {
      const airNoise = Math.sin(elapsed * 0.72 + blossom.phase) * 0.7
        + Math.sin(elapsed * 0.29 + blossom.phase * 1.91) * 0.3
      const lateralTarget = airNoise * blossom.sway * (0.09 + scrollGust * 0.08)
      blossom.velocity.x += (lateralTarget - blossom.velocity.x) * (1 - Math.exp(-delta * 1.7))
      blossom.velocity.y -= (0.045 + blossom.scale * 0.035) * delta
      blossom.velocity.y *= Math.exp(-delta * 0.075)
      blossom.position.x += blossom.velocity.x * delta
      blossom.position.y += blossom.velocity.y * delta
      blossom.rotation.x += (blossom.spin.x + airNoise * 0.08) * delta
      blossom.rotation.y += (blossom.spin.y - airNoise * 0.06) * delta
      blossom.rotation.z += (blossom.spin.z + blossom.velocity.x * 0.36) * delta

      if (blossom.position.y < -halfHeight - 0.8 || Math.abs(blossom.position.x) > halfWidth * 1.35) {
        resetBlossom(blossom)
      }

      const topFade = THREE.MathUtils.smoothstep(halfHeight + 0.65 - blossom.position.y, 0, 1.15)
      const bottomFade = THREE.MathUtils.smoothstep(blossom.position.y + halfHeight + 0.72, 0, 1.1)
      const edgeScale = Math.max(0.01, Math.min(topFade, bottomFade))
      dummy.position.copy(blossom.position)
      dummy.rotation.copy(blossom.rotation)
      dummy.scale.setScalar(blossom.scale * edgeScale)
      dummy.updateMatrix()
      blossomMesh.setMatrixAt(index, dummy.matrix)
    })
    blossomMesh.instanceMatrix.needsUpdate = true
    renderer.render(scene, camera)
    animationFrame = requestAnimationFrame(render)
  }

  const handleVisibility = () => {
    visible = !document.hidden
    if (visible && !animationFrame && !disposed) {
      lastFrame = performance.now()
      animationFrame = requestAnimationFrame(render)
    } else if (!visible) {
      cancelAnimationFrame(animationFrame)
      animationFrame = 0
    }
  }

  const handleContextLoss = (event: Event) => {
    event.preventDefault()
    renderState.value = 'fallback'
    disposeScene?.()
    disposeScene = null
  }

  const dispose = () => {
    if (disposed) return
    disposed = true
    cancelAnimationFrame(animationFrame)
    cancelAnimationFrame(resizeFrame)
    window.removeEventListener('resize', scheduleResize)
    document.removeEventListener('visibilitychange', handleVisibility)
    renderer.domElement.removeEventListener('webglcontextlost', handleContextLoss)
    geometry.dispose()
    material.dispose()
    scene.clear()
    renderer.dispose()
    renderer.domElement.remove()
  }

  disposeScene = dispose
  updateViewport()
  for (let index = 0; index < blossomCount; index += 1) {
    const blossom: BlossomState = {
      position: new THREE.Vector3(),
      velocity: new THREE.Vector3(),
      rotation: new THREE.Euler(),
      spin: new THREE.Vector3(),
      scale: 1,
      phase: 0,
      sway: 1,
    }
    resetBlossom(blossom, true)
    blossoms.push(blossom)
  }
  window.addEventListener('resize', scheduleResize, { passive: true })
  document.addEventListener('visibilitychange', handleVisibility)
  renderer.domElement.addEventListener('webglcontextlost', handleContextLoss, { once: true })
  renderState.value = 'ready'
  animationFrame = requestAnimationFrame(render)
}

function cancelScheduledStart() {
  const cancelIdle = Reflect.get(window, 'cancelIdleCallback') as ((handle: number) => void) | undefined
  if (idleHandle !== null && typeof cancelIdle === 'function') {
    cancelIdle.call(window, idleHandle)
  }
  idleHandle = null
  window.clearTimeout(idleTimer)
  idleTimer = 0
}

function startScene() {
  cancelScheduledStart()
  lifecycleToken += 1
  disposeScene?.()
  disposeScene = null

  const host = hostRef.value
  if (!props.enabled || !host || motionQuery?.matches) {
    renderState.value = 'disabled'
    return
  }
  if (!supportsWebGL()) {
    renderState.value = 'fallback'
    return
  }

  renderState.value = 'idle'
  const token = lifecycleToken
  const begin = () => {
    idleHandle = null
    idleTimer = 0
    void initializeScene(host, token).catch(() => {
      if (token === lifecycleToken) renderState.value = 'fallback'
    })
  }

  const requestIdle = Reflect.get(window, 'requestIdleCallback') as ((
    callback: (deadline: { didTimeout: boolean; timeRemaining: () => number }) => void,
    options?: { timeout: number },
  ) => number) | undefined
  if (typeof requestIdle === 'function') {
    idleHandle = requestIdle.call(window, begin, { timeout: 1200 })
  } else {
    idleTimer = window.setTimeout(begin, 180)
  }
}

function handleMotionPreference() {
  startScene()
}

watch(() => props.enabled, startScene, { flush: 'post' })

onMounted(() => {
  motionQuery = window.matchMedia('(prefers-reduced-motion: reduce)')
  motionQuery.addEventListener?.('change', handleMotionPreference)
  startScene()
})

onBeforeUnmount(() => {
  lifecycleToken += 1
  cancelScheduledStart()
  motionQuery?.removeEventListener?.('change', handleMotionPreference)
  motionQuery = null
  disposeScene?.()
  disposeScene = null
})
</script>

<style scoped>
.home-ambient-effects {
  overflow: hidden;
  contain: strict;
}

.home-ambient-effects :deep(.home-ambient-effects-canvas) {
  display: block;
  width: 100%;
  height: 100%;
  opacity: 0.9;
}

.home-ambient-effects[data-state='disabled'],
.home-ambient-effects[data-state='fallback'] {
  display: none;
}

@media (max-width: 720px) {
  .home-ambient-effects :deep(.home-ambient-effects-canvas) {
    opacity: 0.76;
  }
}

@media (prefers-reduced-motion: reduce) {
  .home-ambient-effects {
    display: none;
  }
}
</style>
