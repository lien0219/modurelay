<template>
  <div
    ref="stageRef"
    class="at-world-scene"
    role="img"
    aria-label="ModuRelay immersive relay infrastructure visualization"
  >
    <div ref="canvasHostRef" class="at-world-canvas" aria-hidden="true"></div>

    <div v-if="hasFallback" class="at-world-fallback" aria-hidden="true">
      <span class="at-fallback-ring at-fallback-ring-one"></span>
      <span class="at-fallback-ring at-fallback-ring-two"></span>
      <span class="at-fallback-core">M</span>
    </div>

    <div class="at-world-vignette" aria-hidden="true"></div>
    <div class="at-world-grain" aria-hidden="true"></div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { isDarkTheme, loadThree, observeTheme } from '@/utils/threeRuntime'

const emit = defineEmits<{
  ready: []
}>()

const stageRef = ref<HTMLElement | null>(null)
const canvasHostRef = ref<HTMLElement | null>(null)
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
    hasFallback.value = true
    emit('ready')
    return
  }

  void initializeScene(stage, host)
    .then((cleanup) => {
      if (token !== sceneToken) {
        cleanup()
        return
      }
      cleanupScene = cleanup
      emit('ready')
    })
    .catch((error) => {
      if (token !== sceneToken) return
      console.warn('[HomeHeroScene] Immersive scene failed, using fallback.', error)
      hasFallback.value = true
      emit('ready')
    })
})

onBeforeUnmount(() => {
  sceneToken += 1
  cleanupScene?.()
  cleanupScene = null
})

async function initializeScene(stage: HTMLElement, host: HTMLElement): Promise<() => void> {
  const THREE = await loadThree()
  const reducedMotionQuery = window.matchMedia('(prefers-reduced-motion: reduce)')
  let reducedMotion = reducedMotionQuery.matches
  let stopped = false
  let visible = !document.hidden
  let animationFrame = 0
  let lastTime = performance.now()
  let scrollTarget = 0
  let scrollCurrent = 0
  let pointerTargetX = 0
  let pointerTargetY = 0
  let pointerX = 0
  let pointerY = 0
  let trailVisible = 0

  const scene = new THREE.Scene()
  const camera = new THREE.PerspectiveCamera(38, 1, 0.08, 120)
  camera.position.set(0, 2.3, 7.2)
  scene.add(camera)

  const renderer = new THREE.WebGLRenderer({
    antialias: true,
    alpha: false,
    powerPreference: 'high-performance'
  })
  renderer.outputColorSpace = THREE.SRGBColorSpace
  renderer.toneMapping = THREE.ACESFilmicToneMapping
  renderer.toneMappingExposure = 0.92
  renderer.shadowMap.enabled = true
  renderer.shadowMap.type = THREE.PCFSoftShadowMap
  renderer.setClearAlpha(1)
  host.appendChild(renderer.domElement)

  const world = new THREE.Group()
  scene.add(world)

  const frameMaterial = new THREE.MeshStandardMaterial({
    color: 0x171b28,
    metalness: 0.82,
    roughness: 0.34
  })
  const floorMaterial = new THREE.MeshStandardMaterial({
    color: 0x0b0e16,
    metalness: 0.46,
    roughness: 0.48
  })
  const glassMaterial = new THREE.MeshPhysicalMaterial({
    color: 0x6f7ff7,
    transparent: true,
    opacity: 0.15,
    roughness: 0.12,
    metalness: 0.12,
    transmission: 0.34,
    thickness: 0.7,
    side: THREE.DoubleSide,
    depthWrite: false
  })
  const primaryMaterial = new THREE.MeshStandardMaterial({
    color: 0x6670ed,
    emissive: 0x222a7e,
    emissiveIntensity: 1.15,
    metalness: 0.5,
    roughness: 0.28
  })
  const accentMaterial = new THREE.MeshStandardMaterial({
    color: 0x55cee9,
    emissive: 0x0c5c74,
    emissiveIntensity: 0.9,
    metalness: 0.42,
    roughness: 0.3
  })
  const dimMaterial = new THREE.MeshStandardMaterial({
    color: 0x272d3d,
    emissive: 0x0d1020,
    emissiveIntensity: 0.3,
    metalness: 0.55,
    roughness: 0.58
  })

  const ambient = new THREE.HemisphereLight(0xaab9ff, 0x080a10, 1.24)
  scene.add(ambient)

  const keyLight = new THREE.DirectionalLight(0xdde5ff, 2.2)
  keyLight.position.set(4, 8, 8)
  keyLight.castShadow = true
  keyLight.shadow.mapSize.set(1024, 1024)
  scene.add(keyLight)

  const chapterLights: any[] = []
  const chapterDepths = [0, -7, -14, -21, -28, -35]
  chapterDepths.forEach((z: number, index: number) => {
    const light = new THREE.PointLight(index % 2 === 0 ? 0x6975ff : 0x40cce9, 8.5, 13, 2)
    light.position.set(index % 2 === 0 ? -2.8 : 2.8, 2.5 + (index % 3) * 0.4, z)
    world.add(light)
    chapterLights.push(light)
  })

  const floor = new THREE.Mesh(new THREE.PlaneGeometry(18, 56, 1, 1), floorMaterial)
  floor.rotation.x = -Math.PI / 2
  floor.position.set(0, -1.14, -17.5)
  floor.receiveShadow = true
  world.add(floor)

  const ceiling = new THREE.Mesh(new THREE.PlaneGeometry(18, 56, 1, 1), dimMaterial)
  ceiling.rotation.x = Math.PI / 2
  ceiling.position.set(0, 6.8, -17.5)
  world.add(ceiling)

  const wallGeometry = new THREE.BoxGeometry(0.18, 8, 56)
  const leftWall = new THREE.Mesh(wallGeometry, frameMaterial)
  leftWall.position.set(-7.4, 2.8, -17.5)
  const rightWall = leftWall.clone()
  rightWall.position.x = 7.4
  world.add(leftWall, rightWall)

  const frameHorizontalGeometry = new THREE.BoxGeometry(14.6, 0.16, 0.22)
  const frameVerticalGeometry = new THREE.BoxGeometry(0.16, 7.7, 0.22)
  const frames: any[] = []
  for (let index = 0; index < 13; index += 1) {
    const frame = new THREE.Group()
    const top = new THREE.Mesh(frameHorizontalGeometry, frameMaterial)
    const bottom = new THREE.Mesh(frameHorizontalGeometry, frameMaterial)
    const left = new THREE.Mesh(frameVerticalGeometry, frameMaterial)
    const right = new THREE.Mesh(frameVerticalGeometry, frameMaterial)
    top.position.y = 6.65
    bottom.position.y = -1.05
    left.position.set(-7.2, 2.8, 0)
    right.position.set(7.2, 2.8, 0)
    frame.add(top, bottom, left, right)
    frame.position.z = 3.5 - index * 3.45
    frame.rotation.z = (index % 2 === 0 ? 1 : -1) * 0.004 * index
    world.add(frame)
    frames.push(frame)
  }

  const laneGeometry = new THREE.BoxGeometry(0.035, 0.012, 53)
  ;[-3.6, 0, 3.6].forEach((x: number, index: number) => {
    const lane = new THREE.Mesh(laneGeometry, index === 1 ? accentMaterial : primaryMaterial)
    lane.position.set(x, -1.045, -17.5)
    world.add(lane)
  })

  function addLightPanel(x: number, y: number, z: number, width: number, colorMaterial: any) {
    const panel = new THREE.Mesh(new THREE.BoxGeometry(width, 0.035, 0.04), colorMaterial)
    panel.position.set(x, y, z)
    world.add(panel)
    return panel
  }

  chapterDepths.forEach((z: number, index: number) => {
    addLightPanel(index % 2 === 0 ? 4.9 : -4.9, 5.65, z + 0.4, 3.2, index % 2 === 0 ? primaryMaterial : accentMaterial)
    addLightPanel(index % 2 === 0 ? -5.8 : 5.8, 0.25, z - 1.4, 1.4, index % 2 === 0 ? accentMaterial : primaryMaterial)
  })

  const emblemCanvas = document.createElement('canvas')
  emblemCanvas.width = 768
  emblemCanvas.height = 768
  const emblemCanvasContext = emblemCanvas.getContext('2d')
  if (!emblemCanvasContext) throw new Error('Unable to create the Relay Core emblem')
  const emblemContext: CanvasRenderingContext2D = emblemCanvasContext
  const emblemTexture = new THREE.CanvasTexture(emblemCanvas)
  emblemTexture.colorSpace = THREE.SRGBColorSpace
  emblemTexture.anisotropy = Math.min(8, renderer.capabilities.getMaxAnisotropy())

  function redrawEmblem(dark: boolean) {
    emblemContext.clearRect(0, 0, 768, 768)
    emblemContext.fillStyle = dark ? '#0d111c' : '#edf1fb'
    emblemContext.fillRect(0, 0, 768, 768)
    emblemContext.strokeStyle = dark ? '#7580ff' : '#4f46e5'
    emblemContext.lineWidth = 14
    emblemContext.strokeRect(34, 34, 700, 700)
    emblemContext.fillStyle = dark ? '#f5f7ff' : '#111526'
    emblemContext.textAlign = 'center'
    emblemContext.textBaseline = 'middle'
    emblemContext.font = '800 368px system-ui, sans-serif'
    emblemContext.fillText('M', 384, 340)
    emblemContext.fillStyle = dark ? '#8d96b0' : '#596074'
    emblemContext.font = '700 38px ui-monospace, monospace'
    emblemContext.fillText('RELAY CORE', 384, 612)
    emblemTexture.needsUpdate = true
  }

  const core = new THREE.Group()
  core.position.set(0, 2.45, 0)
  const coreBody = new THREE.Mesh(new THREE.CylinderGeometry(1.5, 1.72, 3.55, 6, 1, false), frameMaterial)
  coreBody.rotation.y = Math.PI / 6
  coreBody.castShadow = true
  const coreShell = new THREE.Mesh(new THREE.IcosahedronGeometry(2.32, 2), glassMaterial)
  const coreBand = new THREE.Mesh(new THREE.TorusGeometry(2.55, 0.035, 12, 180), primaryMaterial)
  coreBand.rotation.x = Math.PI / 2.7
  coreBand.rotation.y = Math.PI / 5
  const coreBandTwo = new THREE.Mesh(new THREE.TorusGeometry(2.72, 0.022, 10, 160), accentMaterial)
  coreBandTwo.rotation.set(Math.PI / 2.1, Math.PI / 3, 0)
  const emblem = new THREE.Mesh(
    new THREE.PlaneGeometry(1.68, 1.68),
    new THREE.MeshBasicMaterial({ map: emblemTexture, transparent: true, side: THREE.DoubleSide })
  )
  emblem.position.set(0, 0.12, 1.525)
  core.add(coreBody, coreShell, coreBand, coreBandTwo, emblem)
  world.add(core)

  const endpointGroup = new THREE.Group()
  endpointGroup.position.set(0, 2.1, -7)
  for (let index = 0; index < 5; index += 1) {
    const plate = new THREE.Mesh(new THREE.BoxGeometry(2.2, 0.055, 0.9), index % 2 ? glassMaterial : dimMaterial)
    plate.position.set((index - 2) * 1.46, (index % 2) * 0.58 - 0.35, (index % 3) * 0.26)
    plate.rotation.y = (index - 2) * -0.08
    const status = new THREE.Mesh(new THREE.BoxGeometry(0.62, 0.032, 0.04), index % 2 ? accentMaterial : primaryMaterial)
    status.position.set(0, 0.06, 0.48)
    plate.add(status)
    endpointGroup.add(plate)
  }
  world.add(endpointGroup)

  const routingGroup = new THREE.Group()
  routingGroup.position.set(0, 2.15, -14)
  const routingCore = new THREE.Mesh(new THREE.OctahedronGeometry(0.72, 1), primaryMaterial)
  routingGroup.add(routingCore)
  const routeTargets = [
    new THREE.Vector3(-3.7, 1.7, -0.6),
    new THREE.Vector3(-2.4, -1.45, 0.2),
    new THREE.Vector3(2.6, 1.65, 0.1),
    new THREE.Vector3(3.9, -1.2, -0.5)
  ]
  routeTargets.forEach((target: any, index: number) => {
    const curve = new THREE.CatmullRomCurve3([
      new THREE.Vector3(0, 0, 0),
      new THREE.Vector3(target.x * 0.45, target.y * 0.18, 0.65 * (index % 2 ? 1 : -1)),
      target
    ])
    const tube = new THREE.Mesh(
      new THREE.TubeGeometry(curve, 48, index % 2 ? 0.025 : 0.035, 8, false),
      index % 2 ? accentMaterial : primaryMaterial
    )
    const node = new THREE.Mesh(new THREE.IcosahedronGeometry(0.3 + index * 0.025, 1), index % 2 ? accentMaterial : glassMaterial)
    node.position.copy(target)
    routingGroup.add(tube, node)
  })
  world.add(routingGroup)

  const observabilityGroup = new THREE.Group()
  observabilityGroup.position.set(0, -1.02, -21)
  const towerHeights = [1.2, 2.15, 1.62, 3.15, 2.48, 3.82, 2.9, 4.45, 3.4]
  towerHeights.forEach((height, index) => {
    const bar = new THREE.Mesh(
      new THREE.BoxGeometry(0.5, height, 0.5),
      index === towerHeights.length - 1 ? accentMaterial : index % 3 === 0 ? primaryMaterial : dimMaterial
    )
    bar.position.set((index - 4) * 0.82, height / 2, Math.sin(index * 1.8) * 0.38)
    bar.castShadow = true
    observabilityGroup.add(bar)
  })
  world.add(observabilityGroup)

  const learningGroup = new THREE.Group()
  learningGroup.position.set(0, 2.25, -28)
  const learningCore = new THREE.Mesh(new THREE.IcosahedronGeometry(1.08, 2), glassMaterial)
  const learningInner = new THREE.Mesh(new THREE.OctahedronGeometry(0.54, 1), accentMaterial)
  learningGroup.add(learningCore, learningInner)
  const learningOrbits: any[] = []
  for (let index = 0; index < 4; index += 1) {
    const orbit = new THREE.Group()
    orbit.rotation.set(index * 0.38, index * 0.72, index * 0.24)
    const ring = new THREE.Mesh(new THREE.TorusGeometry(2.0 + index * 0.42, 0.018, 8, 140), index % 2 ? primaryMaterial : accentMaterial)
    const node = new THREE.Mesh(new THREE.SphereGeometry(0.12 + index * 0.018, 14, 14), index % 2 ? accentMaterial : primaryMaterial)
    node.position.x = 2.0 + index * 0.42
    orbit.add(ring, node)
    learningGroup.add(orbit)
    learningOrbits.push(orbit)
  }
  world.add(learningGroup)

  const portalGroup = new THREE.Group()
  portalGroup.position.set(0, 2.4, -35)
  const portalOuter = new THREE.Mesh(new THREE.TorusGeometry(3.35, 0.12, 18, 180), frameMaterial)
  const portalMiddle = new THREE.Mesh(new THREE.TorusGeometry(2.95, 0.045, 12, 180), primaryMaterial)
  const portalInner = new THREE.Mesh(new THREE.TorusGeometry(2.58, 0.026, 10, 180), accentMaterial)
  portalGroup.add(portalOuter, portalMiddle, portalInner)
  world.add(portalGroup)

  const trailLength = 34
  const trailPositionsA = new Float32Array(trailLength * 3)
  const trailPositionsB = new Float32Array(trailLength * 3)
  const trailGeometryA = new THREE.BufferGeometry()
  const trailGeometryB = new THREE.BufferGeometry()
  trailGeometryA.setAttribute('position', new THREE.BufferAttribute(trailPositionsA, 3))
  trailGeometryB.setAttribute('position', new THREE.BufferAttribute(trailPositionsB, 3))
  const trailMaterialA = new THREE.LineBasicMaterial({ color: 0x7883ff, transparent: true, opacity: 0, depthTest: false })
  const trailMaterialB = new THREE.LineBasicMaterial({ color: 0x45d5ef, transparent: true, opacity: 0, depthTest: false })
  const trailLineA = new THREE.Line(trailGeometryA, trailMaterialA)
  const trailLineB = new THREE.Line(trailGeometryB, trailMaterialB)
  trailLineA.frustumCulled = false
  trailLineB.frustumCulled = false
  const cursorHeadA = new THREE.Mesh(new THREE.SphereGeometry(0.035, 12, 12), new THREE.MeshBasicMaterial({ color: 0x8991ff, transparent: true, opacity: 0, depthTest: false }))
  const cursorHeadB = new THREE.Mesh(new THREE.SphereGeometry(0.023, 12, 12), new THREE.MeshBasicMaterial({ color: 0x70e8ff, transparent: true, opacity: 0, depthTest: false }))
  const trailGroup = new THREE.Group()
  trailGroup.position.z = -3.7
  trailGroup.renderOrder = 999
  trailGroup.add(trailLineA, trailLineB, cursorHeadA, cursorHeadB)
  camera.add(trailGroup)

  function updateScrollTarget() {
    const scrollable = Math.max(document.documentElement.scrollHeight - window.innerHeight, 1)
    scrollTarget = Math.min(1, Math.max(0, window.scrollY / scrollable))
  }

  function onPointerMove(event: PointerEvent) {
    if (reducedMotion || event.pointerType === 'touch') return
    const rect = stage.getBoundingClientRect()
    pointerTargetX = ((event.clientX - rect.left) / Math.max(rect.width, 1)) * 2 - 1
    pointerTargetY = -(((event.clientY - rect.top) / Math.max(rect.height, 1)) * 2 - 1)
    trailVisible = 1
  }

  function onPointerLeave() {
    pointerTargetX = 0
    pointerTargetY = 0
    trailVisible = 0
  }

  function onVisibilityChange() {
    visible = !document.hidden
    if (visible && !animationFrame && !stopped && !reducedMotion) {
      lastTime = performance.now()
      animationFrame = requestAnimationFrame(renderFrame)
    }
  }

  function onReducedMotionChange(event: MediaQueryListEvent) {
    reducedMotion = event.matches
    if (reducedMotion) {
      cancelAnimationFrame(animationFrame)
      animationFrame = 0
      renderStaticFrame()
    } else if (!animationFrame && visible && !stopped) {
      lastTime = performance.now()
      animationFrame = requestAnimationFrame(renderFrame)
    }
  }

  function resize() {
    const rect = stage.getBoundingClientRect()
    const width = Math.max(1, rect.width)
    const height = Math.max(1, rect.height)
    renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, width < 720 ? 1.25 : 1.65))
    renderer.setSize(width, height, false)
    camera.aspect = width / height
    camera.fov = width < 720 ? 50 : 38
    camera.updateProjectionMatrix()
    if (reducedMotion) renderStaticFrame()
  }

  function updateTrail(elapsed: number) {
    pointerX += (pointerTargetX - pointerX) * 0.095
    pointerY += (pointerTargetY - pointerY) * 0.095
    const targetHeadX = pointerX * (camera.aspect > 1 ? 2.75 : 1.62)
    const targetHeadY = pointerY * 1.6

    for (let index = trailLength - 1; index > 0; index -= 1) {
      const offset = index * 3
      const previous = (index - 1) * 3
      trailPositionsA[offset] = trailPositionsA[previous]
      trailPositionsA[offset + 1] = trailPositionsA[previous + 1]
      trailPositionsA[offset + 2] = 0
      trailPositionsB[offset] = trailPositionsB[previous]
      trailPositionsB[offset + 1] = trailPositionsB[previous + 1]
      trailPositionsB[offset + 2] = -0.012
    }

    trailPositionsA[0] += (targetHeadX - trailPositionsA[0]) * 0.28
    trailPositionsA[1] += (targetHeadY - trailPositionsA[1]) * 0.28
    trailPositionsB[0] += (targetHeadX + Math.sin(elapsed * 2.1) * 0.035 - trailPositionsB[0]) * 0.21
    trailPositionsB[1] += (targetHeadY + Math.cos(elapsed * 1.8) * 0.035 - trailPositionsB[1]) * 0.21

    trailGeometryA.attributes.position.needsUpdate = true
    trailGeometryB.attributes.position.needsUpdate = true
    trailMaterialA.opacity += ((trailVisible ? 0.48 : 0) - trailMaterialA.opacity) * 0.12
    trailMaterialB.opacity += ((trailVisible ? 0.34 : 0) - trailMaterialB.opacity) * 0.12
    cursorHeadA.material.opacity = trailMaterialA.opacity * 1.5
    cursorHeadB.material.opacity = trailMaterialB.opacity * 1.7
    cursorHeadA.position.set(trailPositionsA[0], trailPositionsA[1], 0)
    cursorHeadB.position.set(trailPositionsB[0], trailPositionsB[1], -0.012)
  }

  function positionCamera(progress: number, elapsed = 0) {
    const chapterProgress = progress * (chapterDepths.length - 1)
    const cameraZ = 7.2 - progress * 35
    const drift = Math.sin(chapterProgress * 1.18) * 0.38
    const lift = Math.cos(chapterProgress * 0.82) * 0.2
    camera.position.set(drift + pointerX * 0.16, 2.32 + lift + pointerY * 0.11, cameraZ)
    camera.lookAt(drift * 0.18, 2.05 + lift * 0.2, cameraZ - 7.2)

    core.rotation.y = elapsed * 0.13 + progress * 0.7
    coreShell.rotation.x = elapsed * 0.07
    coreShell.rotation.z = elapsed * -0.05
    coreBand.rotation.z = elapsed * 0.16
    coreBandTwo.rotation.z = elapsed * -0.11
    endpointGroup.rotation.y = Math.sin(elapsed * 0.24) * 0.08
    routingCore.rotation.x = elapsed * 0.28
    routingCore.rotation.y = elapsed * 0.36
    learningInner.rotation.x = elapsed * 0.38
    learningInner.rotation.y = elapsed * 0.46
    learningOrbits.forEach((orbit, index) => {
      orbit.rotation.z += 0.0007 * (index % 2 ? -1 : 1)
      orbit.rotation.y += 0.00045 * (index + 1)
    })
    portalMiddle.rotation.z = elapsed * 0.05
    portalInner.rotation.z = elapsed * -0.08
    frames.forEach((frame, index) => {
      frame.rotation.z += Math.sin(elapsed * 0.18 + index) * 0.000008
    })
    chapterLights.forEach((light, index) => {
      light.intensity = 7.6 + Math.sin(elapsed * 1.4 + index * 1.7) * 1.1
    })
  }

  function renderStaticFrame() {
    pointerX = 0
    pointerY = 0
    trailMaterialA.opacity = 0
    trailMaterialB.opacity = 0
    cursorHeadA.material.opacity = 0
    cursorHeadB.material.opacity = 0
    positionCamera(0)
    renderer.render(scene, camera)
  }

  function renderFrame(time: number) {
    animationFrame = 0
    if (stopped || !visible || reducedMotion) return
    const delta = Math.min(48, Math.max(0, time - lastTime))
    lastTime = time
    const elapsed = time * 0.001
    const scrollEase = 1 - Math.pow(0.001, delta / 1000)
    scrollCurrent += (scrollTarget - scrollCurrent) * Math.min(0.18, scrollEase * 4.2)
    updateTrail(elapsed)
    positionCamera(scrollCurrent, elapsed)
    renderer.render(scene, camera)
    animationFrame = requestAnimationFrame(renderFrame)
  }

  function applyTheme(dark: boolean) {
    const background = dark ? 0x080a10 : 0xdfe5f2
    const fog = dark ? 0x090b12 : 0xd8deeb
    scene.background = new THREE.Color(background)
    scene.fog = new THREE.FogExp2(fog, dark ? 0.032 : 0.026)
    frameMaterial.color.setHex(dark ? 0x171b28 : 0x727b8f)
    floorMaterial.color.setHex(dark ? 0x0b0e16 : 0xcbd2df)
    dimMaterial.color.setHex(dark ? 0x272d3d : 0x919aad)
    dimMaterial.emissive.setHex(dark ? 0x0d1020 : 0x31384a)
    glassMaterial.color.setHex(dark ? 0x6f7ff7 : 0x4f46e5)
    primaryMaterial.color.setHex(dark ? 0x6975ff : 0x4f46e5)
    primaryMaterial.emissive.setHex(dark ? 0x222a7e : 0x282071)
    accentMaterial.color.setHex(dark ? 0x55cee9 : 0x0891b2)
    accentMaterial.emissive.setHex(dark ? 0x0c5c74 : 0x074656)
    ambient.color.setHex(dark ? 0xaab9ff : 0xffffff)
    ambient.groundColor.setHex(dark ? 0x080a10 : 0x8b94a7)
    keyLight.color.setHex(dark ? 0xdde5ff : 0xffffff)
    renderer.toneMappingExposure = dark ? 0.92 : 0.84
    redrawEmblem(dark)
    if (reducedMotion) renderStaticFrame()
  }

  const stopThemeObserver = observeTheme(applyTheme)
  const resizeObserver = new ResizeObserver(resize)
  resizeObserver.observe(stage)
  window.addEventListener('scroll', updateScrollTarget, { passive: true })
  window.addEventListener('pointermove', onPointerMove, { passive: true })
  document.documentElement.addEventListener('pointerleave', onPointerLeave)
  document.addEventListener('visibilitychange', onVisibilityChange)
  if (typeof reducedMotionQuery.addEventListener === 'function') {
    reducedMotionQuery.addEventListener('change', onReducedMotionChange)
  }

  resize()
  updateScrollTarget()
  applyTheme(isDarkTheme())
  if (reducedMotion) {
    renderStaticFrame()
  } else {
    animationFrame = requestAnimationFrame(renderFrame)
  }

  return () => {
    stopped = true
    cancelAnimationFrame(animationFrame)
    animationFrame = 0
    resizeObserver.disconnect()
    stopThemeObserver()
    window.removeEventListener('scroll', updateScrollTarget)
    window.removeEventListener('pointermove', onPointerMove)
    document.documentElement.removeEventListener('pointerleave', onPointerLeave)
    document.removeEventListener('visibilitychange', onVisibilityChange)
    if (typeof reducedMotionQuery.removeEventListener === 'function') {
      reducedMotionQuery.removeEventListener('change', onReducedMotionChange)
    }

    const disposedGeometries = new Set<any>()
    const disposedMaterials = new Set<any>()
    const disposedTextures = new Set<any>()
    scene.traverse((object: any) => {
      if (object.geometry?.dispose && !disposedGeometries.has(object.geometry)) {
        disposedGeometries.add(object.geometry)
        object.geometry.dispose()
      }
      const materials = Array.isArray(object.material) ? object.material : object.material ? [object.material] : []
      materials.forEach((material: any) => {
        if (disposedMaterials.has(material)) return
        disposedMaterials.add(material)
        Object.values(material).forEach((value: any) => {
          if (value?.isTexture && !disposedTextures.has(value)) {
            disposedTextures.add(value)
            value.dispose()
          }
        })
        material.dispose?.()
      })
    })
    renderer.dispose()
    renderer.forceContextLoss?.()
    host.replaceChildren()
  }
}
</script>

<style scoped>
.at-world-scene {
  position: absolute;
  inset: 0;
  overflow: hidden;
  background: var(--color-bg-deep, #090c12);
}

.at-world-canvas,
.at-world-canvas :deep(canvas),
.at-world-vignette,
.at-world-grain,
.at-world-fallback {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}

.at-world-canvas :deep(canvas) {
  display: block;
  outline: none;
}

.at-world-vignette {
  box-shadow: inset 0 0 13vw rgba(2, 4, 10, 0.36), inset 0 -20vh 22vh rgba(3, 5, 11, 0.16);
  pointer-events: none;
}

.at-world-grain {
  opacity: 0.045;
  background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 180 180' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='.88' numOctaves='3' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)' opacity='.72'/%3E%3C/svg%3E");
  pointer-events: none;
}

.at-world-fallback {
  display: grid;
  place-items: center;
  color: var(--color-primary, #6366f1);
}

.at-fallback-ring,
.at-fallback-core {
  position: absolute;
  display: grid;
  place-items: center;
  border: 1px solid currentColor;
  border-radius: 50%;
}

.at-fallback-ring-one {
  width: min(56vw, 540px);
  height: min(56vw, 540px);
  opacity: 0.24;
  transform: rotateX(62deg) rotateZ(18deg);
}

.at-fallback-ring-two {
  width: min(38vw, 360px);
  height: min(38vw, 360px);
  opacity: 0.42;
  transform: rotateY(58deg) rotateZ(-24deg);
}

.at-fallback-core {
  width: 118px;
  height: 118px;
  border-radius: 30px;
  color: var(--color-text-primary, #f5f7ff);
  background: var(--color-surface, #131720);
  box-shadow: var(--shadow-lg, 0 20px 60px rgba(0, 0, 0, 0.24));
  font: 800 52px/1 system-ui, sans-serif;
}

:global(html:not(.dark)) .at-world-vignette {
  box-shadow: inset 0 0 12vw rgba(75, 84, 110, 0.18), inset 0 -20vh 22vh rgba(86, 96, 118, 0.12);
}

@media (prefers-reduced-motion: reduce) {
  .at-world-grain {
    display: none;
  }
}
</style>
