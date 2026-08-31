<template>
  <div ref="stageRef" class="kinetic-scene" role="img" aria-label="ModuRelay living signal field">
    <div ref="canvasHostRef" class="kinetic-scene-canvas" aria-hidden="true"></div>
    <div v-if="hasFallback" class="kinetic-scene-fallback" aria-hidden="true">
      <div class="kinetic-fallback-fibers kinetic-fallback-fibers-left"></div>
      <div class="kinetic-fallback-fibers kinetic-fallback-fibers-right"></div>
      <div class="kinetic-fallback-orbit"><span>M</span></div>
      <div class="kinetic-fallback-dust"></div>
    </div>
    <div class="kinetic-scene-vignette" aria-hidden="true"></div>
    <div class="kinetic-scene-chromatic" aria-hidden="true"></div>
    <div class="kinetic-scene-grain" aria-hidden="true"></div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { loadThree } from '@/utils/threeRuntime'

const props = withDefaults(defineProps<{ progress?: number }>(), { progress: 0 })
const emit = defineEmits<{ ready: [] }>()
const stageRef = ref<HTMLElement | null>(null)
const canvasHostRef = ref<HTMLElement | null>(null)
const hasFallback = ref(false)

let cleanupScene: (() => void) | null = null
let sceneToken = 0
let requestedProgress = 0

watch(() => props.progress, (value) => {
  requestedProgress = Math.min(1, Math.max(0, Number(value) || 0))
}, { immediate: true })

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
      console.warn('[HomeHeroScene] Living scene failed, using fallback.', error)
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
  let elapsed = 0
  let progress = requestedProgress
  let previousProgress = progress
  let pointerTargetX = 0
  let pointerTargetY = 0
  let pointerX = 0
  let pointerY = 0
  let pointerEnergy = 0
  let pointerInside = 0

  const scene = new THREE.Scene()
  scene.background = new THREE.Color(0x04090b)
  scene.fog = new THREE.FogExp2(0x071012, 0.052)
  const camera = new THREE.PerspectiveCamera(42, 1, 0.08, 80)
  camera.position.set(0, 0.25, 8.2)
  scene.add(camera)

  const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: false, powerPreference: 'high-performance' })
  renderer.outputColorSpace = THREE.SRGBColorSpace
  renderer.toneMapping = THREE.ACESFilmicToneMapping
  renderer.toneMappingExposure = 1.05
  renderer.setClearColor(0x04090b, 1)
  renderer.domElement.className = 'kinetic-webgl-canvas'
  host.appendChild(renderer.domElement)

  let composer: any = null
  let bloomPass: any = null
  try {
    const [{ EffectComposer }, { RenderPass }, { UnrealBloomPass }, { OutputPass }] = await Promise.all([
      import('three/examples/jsm/postprocessing/EffectComposer.js'),
      import('three/examples/jsm/postprocessing/RenderPass.js'),
      import('three/examples/jsm/postprocessing/UnrealBloomPass.js'),
      import('three/examples/jsm/postprocessing/OutputPass.js')
    ])
    composer = new EffectComposer(renderer)
    composer.addPass(new RenderPass(scene, camera))
    bloomPass = new UnrealBloomPass(new THREE.Vector2(1, 1), 0.72, 0.7, 0.72)
    composer.addPass(bloomPass)
    composer.addPass(new OutputPass())
  } catch (error) {
    console.warn('[HomeHeroScene] Bloom unavailable, continuing with the base renderer.', error)
  }

  const root = new THREE.Group()
  scene.add(root)
  scene.add(new THREE.HemisphereLight(0xbfe7dc, 0x030507, 0.7))
  const keyLight = new THREE.DirectionalLight(0xe6fff5, 3.4)
  keyLight.position.set(-3.2, 5.5, 6.5)
  scene.add(keyLight)
  const cyanLight = new THREE.PointLight(0x40d9e6, 15, 16, 2)
  cyanLight.position.set(4, 1.8, 2.4)
  scene.add(cyanLight)
  const limeLight = new THREE.PointLight(0xb8ff65, 12, 14, 2)
  limeLight.position.set(-4.2, -1.4, 1.2)
  scene.add(limeLight)
  const violetLight = new THREE.PointLight(0x8b7cff, 10, 13, 2)
  violetLight.position.set(0.5, 3.5, -2.5)
  scene.add(violetLight)

  function seededRandom(seed = 2147483647) {
    let state = seed >>> 0
    return () => {
      state += 0x6D2B79F5
      let value = state
      value = Math.imul(value ^ (value >>> 15), value | 1)
      value ^= value + Math.imul(value ^ (value >>> 7), value | 61)
      return ((value ^ (value >>> 14)) >>> 0) / 4294967296
    }
  }
  const random = seededRandom(240821)
  const smoothstep = (edge0: number, edge1: number, value: number) => {
    const normalized = Math.min(1, Math.max(0, (value - edge0) / Math.max(edge1 - edge0, 0.0001)))
    return normalized * normalized * (3 - 2 * normalized)
  }
  const mix = (start: number, end: number, amount: number) => start + (end - start) * amount

  function createFogTexture() {
    const canvas = document.createElement('canvas')
    canvas.width = 256
    canvas.height = 256
    const context = canvas.getContext('2d')
    if (!context) throw new Error('Unable to create atmospheric texture')
    const gradient = context.createRadialGradient(128, 128, 4, 128, 128, 126)
    gradient.addColorStop(0, 'rgba(255,255,255,0.88)')
    gradient.addColorStop(0.22, 'rgba(255,255,255,0.38)')
    gradient.addColorStop(0.58, 'rgba(255,255,255,0.08)')
    gradient.addColorStop(1, 'rgba(255,255,255,0)')
    context.fillStyle = gradient
    context.fillRect(0, 0, 256, 256)
    const texture = new THREE.CanvasTexture(canvas)
    texture.colorSpace = THREE.SRGBColorSpace
    return texture
  }

  const fogTexture = createFogTexture()
  const fogSprites: any[] = []
  const fogSettings = [
    [-4.8, 2.8, -2.5, 8.5, 0x1b675e, 0.24],
    [4.9, 2.1, -3.8, 9.4, 0x244f59, 0.22],
    [-5.2, -2.4, -1.5, 7.2, 0x4b4b22, 0.17],
    [5.4, -2.6, -2.2, 7.8, 0x133e4d, 0.2],
    [0, 3.5, -5.8, 10.8, 0x1b4f49, 0.17],
    [0.8, -3.8, -4.4, 8.4, 0x392a59, 0.14]
  ]
  fogSettings.forEach((setting) => {
    const material = new THREE.SpriteMaterial({
      map: fogTexture,
      color: setting[4],
      transparent: true,
      opacity: setting[5],
      blending: THREE.AdditiveBlending,
      depthWrite: false
    })
    const sprite = new THREE.Sprite(material)
    sprite.position.set(setting[0], setting[1], setting[2])
    sprite.scale.set(setting[3], setting[3], 1)
    root.add(sprite)
    fogSprites.push(sprite)
  })

  const particleCount = window.innerWidth < 700 ? 7200 : 15200
  const positions = new Float32Array(particleCount * 3)
  const colors = new Float32Array(particleCount * 3)
  const sizes = new Float32Array(particleCount)
  const phases = new Float32Array(particleCount)
  const particleColor = new THREE.Color()
  for (let index = 0; index < particleCount; index += 1) {
    const offset = index * 3
    const angle = random() * Math.PI * 2
    const radius = 1.25 + Math.pow(random(), 0.7) * 7.2
    const canopy = random()
    const sideBias = random() > 0.5 ? 1 : -1
    let x = Math.cos(angle) * radius
    let y = Math.sin(angle) * radius * 0.58
    let z = (random() - 0.5) * 6.8 - 1.7
    if (canopy < 0.48) {
      x = sideBias * (1.5 + Math.pow(random(), 0.58) * 6.4)
      y = -2.6 + Math.pow(random(), 0.56) * 3.4 + Math.sin(x * 1.4) * 0.3
      z = (random() - 0.5) * 5.8 - random() * 2
    } else if (canopy < 0.78) {
      x = sideBias * (1.2 + Math.pow(random(), 0.72) * 6.8)
      y = 1.2 + Math.pow(random(), 0.78) * 3.8
      z = (random() - 0.5) * 7 - 1.8
    }
    positions[offset] = x
    positions[offset + 1] = y
    positions[offset + 2] = z
    const palette = random()
    if (palette < 0.52) particleColor.setHSL(0.23 + random() * 0.18, 0.55 + random() * 0.28, 0.38 + random() * 0.3)
    else if (palette < 0.84) particleColor.setHSL(0.48 + random() * 0.17, 0.5 + random() * 0.35, 0.42 + random() * 0.34)
    else if (palette < 0.95) particleColor.setHSL(0.72 + random() * 0.12, 0.42 + random() * 0.28, 0.48 + random() * 0.28)
    else particleColor.setRGB(0.9 + random() * 0.1, 0.94 + random() * 0.06, 0.9 + random() * 0.1)
    colors[offset] = particleColor.r
    colors[offset + 1] = particleColor.g
    colors[offset + 2] = particleColor.b
    sizes[index] = 1.2 + Math.pow(random(), 2.2) * 7.8
    phases[index] = random() * Math.PI * 2
  }

  const particleGeometry = new THREE.BufferGeometry()
  particleGeometry.setAttribute('position', new THREE.BufferAttribute(positions, 3))
  particleGeometry.setAttribute('color', new THREE.BufferAttribute(colors, 3))
  particleGeometry.setAttribute('aSize', new THREE.BufferAttribute(sizes, 1))
  particleGeometry.setAttribute('aPhase', new THREE.BufferAttribute(phases, 1))
  const particleMaterial = new THREE.ShaderMaterial({
    uniforms: {
      uTime: { value: 0 },
      uOpacity: { value: 0.78 },
      uPointer: { value: new THREE.Vector2(0, 0) },
      uPixelRatio: { value: 1 }
    },
    vertexShader: [
      'attribute float aSize;',
      'attribute float aPhase;',
      'attribute vec3 color;',
      'uniform float uTime;',
      'uniform vec2 uPointer;',
      'uniform float uPixelRatio;',
      'varying vec3 vColor;',
      'varying float vPulse;',
      'void main() {',
      '  vec3 p = position;',
      '  float drift = sin(uTime * 0.34 + aPhase + p.x * 0.23) * 0.11;',
      '  p.y += drift;',
      '  p.x += cos(uTime * 0.21 + aPhase * 1.37 + p.y * 0.18) * 0.07;',
      '  vec2 pointerWorld = uPointer * vec2(4.8, 2.9);',
      '  float pointerDistance = distance(p.xy, pointerWorld);',
      '  float influence = exp(-pointerDistance * pointerDistance * 0.36);',
      '  vec2 direction = normalize(p.xy - pointerWorld + vec2(0.0001));',
      '  p.xy += direction * influence * (0.18 + sin(uTime * 1.4 + aPhase) * 0.08);',
      '  p.z += influence * sin(uTime * 1.8 + aPhase) * 0.48;',
      '  vec4 mvPosition = modelViewMatrix * vec4(p, 1.0);',
      '  gl_Position = projectionMatrix * mvPosition;',
      '  gl_PointSize = min(13.0, aSize * uPixelRatio * (5.5 / max(1.0, -mvPosition.z)));',
      '  vColor = color;',
      '  vPulse = 0.72 + sin(uTime * 0.85 + aPhase) * 0.28;',
      '}'
    ].join('\n'),
    fragmentShader: [
      'uniform float uOpacity;',
      'varying vec3 vColor;',
      'varying float vPulse;',
      'void main() {',
      '  vec2 centered = gl_PointCoord - 0.5;',
      '  float distanceFromCenter = length(centered);',
      '  float alpha = smoothstep(0.5, 0.06, distanceFromCenter);',
      '  float core = smoothstep(0.19, 0.0, distanceFromCenter);',
      '  vec3 color = vColor + core * 0.38;',
      '  gl_FragColor = vec4(color, alpha * uOpacity * vPulse);',
      '}'
    ].join('\n'),
    transparent: true,
    depthWrite: false,
    vertexColors: true,
    blending: THREE.AdditiveBlending
  })
  const particleField = new THREE.Points(particleGeometry, particleMaterial)
  particleField.frustumCulled = false
  root.add(particleField)

  const organism = new THREE.Group()
  organism.position.y = 0.15
  root.add(organism)
  const silverMaterial = new THREE.MeshPhysicalMaterial({
    color: 0xdff7ec, emissive: 0x1d3b34, emissiveIntensity: 0.68, metalness: 0.78,
    roughness: 0.16, clearcoat: 1, clearcoatRoughness: 0.08, iridescence: 0.85,
    iridescenceIOR: 1.45, iridescenceThicknessRange: [110, 680], transparent: true, opacity: 0.92
  })
  const cyanMaterial = new THREE.MeshPhysicalMaterial({
    color: 0x8de9e0, emissive: 0x145961, emissiveIntensity: 0.92, metalness: 0.66,
    roughness: 0.2, clearcoat: 0.9, transparent: true, opacity: 0.88
  })
  const violetMaterial = new THREE.MeshPhysicalMaterial({
    color: 0xc8baff, emissive: 0x32206d, emissiveIntensity: 0.72, metalness: 0.7,
    roughness: 0.19, clearcoat: 1, iridescence: 0.72, transparent: true, opacity: 0.86
  })
  const fiberMaterials = [silverMaterial, cyanMaterial, violetMaterial]

  function createTube(points: any[], radius: number, material: any, segments = 64) {
    const curve = new THREE.CatmullRomCurve3(points)
    const mesh = new THREE.Mesh(new THREE.TubeGeometry(curve, segments, radius, 5, false), material)
    organism.add(mesh)
    return mesh
  }
  createTube([
    new THREE.Vector3(-0.72, -4.2, -0.8), new THREE.Vector3(0.56, -2.45, -0.22),
    new THREE.Vector3(-0.45, -0.3, 0.03), new THREE.Vector3(0.6, 1.72, -0.2),
    new THREE.Vector3(-0.52, 4.4, -1.3)
  ], 0.017, silverMaterial, 108)
  createTube([
    new THREE.Vector3(0.72, -4.2, -0.9), new THREE.Vector3(-0.56, -2.45, -0.2),
    new THREE.Vector3(0.45, -0.3, 0), new THREE.Vector3(-0.6, 1.72, -0.22),
    new THREE.Vector3(0.52, 4.4, -1.4)
  ], 0.012, cyanMaterial, 108)

  const fiberGroups: any[] = []
  function createFiberFan(side: number) {
    const group = new THREE.Group()
    organism.add(group)
    fiberGroups.push(group)
    for (let index = 0; index < 22; index += 1) {
      const normalized = (index - 10.5) / 10.5
      const lift = normalized * 1.9
      const depth = -0.2 - random() * 2.6
      const curve = new THREE.CatmullRomCurve3([
        new THREE.Vector3(side * (0.74 + random() * 0.16), 0.22 + normalized * 0.22, 0),
        new THREE.Vector3(side * (1.5 + random() * 0.55), 0.42 + normalized * 0.72, -0.12),
        new THREE.Vector3(side * (3.15 + random() * 1.05), 0.48 + lift + (random() - 0.5) * 0.55, depth * 0.42),
        new THREE.Vector3(side * (5.2 + random() * 2.4), 0.6 + lift * 1.4 + (random() - 0.5) * 1.1, depth)
      ])
      const geometry = new THREE.TubeGeometry(curve, 48, 0.008 + random() * 0.018, 4, false)
      const fiber = new THREE.Mesh(geometry, fiberMaterials[index % fiberMaterials.length])
      fiber.rotation.z = normalized * 0.035
      group.add(fiber)
    }
  }
  createFiberFan(-1)
  createFiberFan(1)

  const core = new THREE.Group()
  organism.add(core)
  const outerRing = new THREE.Mesh(new THREE.TorusGeometry(1.02, 0.086, 24, 180), silverMaterial)
  const cyanRing = new THREE.Mesh(new THREE.TorusGeometry(0.83, 0.027, 14, 150), cyanMaterial)
  cyanRing.position.z = 0.035
  const orbitRing = new THREE.Mesh(new THREE.TorusGeometry(1.19, 0.011, 8, 180), violetMaterial)
  orbitRing.position.z = -0.035
  core.add(outerRing, cyanRing, orbitRing)
  const coreDiscMaterial = new THREE.MeshPhysicalMaterial({
    color: 0x061113, emissive: 0x061f22, emissiveIntensity: 0.55, roughness: 0.2,
    metalness: 0.56, transmission: 0.16, thickness: 0.4, transparent: true, opacity: 0.9
  })
  const coreDisc = new THREE.Mesh(new THREE.CircleGeometry(0.7, 96), coreDiscMaterial)
  coreDisc.position.z = -0.02
  core.add(coreDisc)

  const emblemCanvas = document.createElement('canvas')
  emblemCanvas.width = 512
  emblemCanvas.height = 512
  const emblemContext = emblemCanvas.getContext('2d')
  if (!emblemContext) throw new Error('Unable to create the ModuRelay emblem')
  emblemContext.strokeStyle = '#eafffb'
  emblemContext.lineWidth = 23
  emblemContext.lineCap = 'round'
  emblemContext.lineJoin = 'round'
  emblemContext.beginPath()
  emblemContext.moveTo(122, 354)
  emblemContext.lineTo(122, 158)
  emblemContext.lineTo(256, 296)
  emblemContext.lineTo(390, 158)
  emblemContext.lineTo(390, 354)
  emblemContext.stroke()
  emblemContext.strokeStyle = '#71e4e6'
  emblemContext.lineWidth = 7
  emblemContext.beginPath()
  emblemContext.arc(256, 256, 186, 0, Math.PI * 2)
  emblemContext.stroke()
  const emblemTexture = new THREE.CanvasTexture(emblemCanvas)
  emblemTexture.colorSpace = THREE.SRGBColorSpace
  const emblem = new THREE.Mesh(
    new THREE.PlaneGeometry(1.12, 1.12),
    new THREE.MeshBasicMaterial({ map: emblemTexture, transparent: true, opacity: 0.94, blending: THREE.AdditiveBlending, depthWrite: false })
  )
  emblem.position.z = 0.12
  core.add(emblem)

  const crown = new THREE.Group()
  core.add(crown)
  for (let index = 0; index < 28; index += 1) {
    const angle = (index / 28) * Math.PI * 2
    const shard = new THREE.Mesh(
      new THREE.BoxGeometry(0.014 + random() * 0.012, 0.12 + random() * 0.42, 0.012),
      index % 3 === 0 ? cyanMaterial : silverMaterial
    )
    shard.position.set(Math.cos(angle) * 1.34, Math.sin(angle) * 1.34, -0.1 + random() * 0.16)
    shard.rotation.z = angle - Math.PI / 2
    crown.add(shard)
  }

  const workSculpture = new THREE.Group()
  workSculpture.position.set(0.4, 0, -0.8)
  root.add(workSculpture)
  const sculptureMaterial = silverMaterial.clone()
  sculptureMaterial.opacity = 0
  const sculptureAccentMaterial = violetMaterial.clone()
  sculptureAccentMaterial.opacity = 0
  const sculptureMeshes: any[] = []
  for (let index = 0; index < 17; index += 1) {
    const size = 0.24 + random() * 0.26
    const geometry = index % 2 === 0 ? new THREE.OctahedronGeometry(size, 1) : new THREE.IcosahedronGeometry(size, 1)
    const mesh = new THREE.Mesh(geometry, index % 3 === 0 ? sculptureAccentMaterial : sculptureMaterial)
    const angle = index * 1.71
    mesh.position.set(Math.sin(angle) * (0.18 + random() * 0.42), (index - 8) * 0.46, Math.cos(angle) * (0.18 + random() * 0.4))
    mesh.rotation.set(random() * Math.PI, random() * Math.PI, random() * Math.PI)
    workSculpture.add(mesh)
    sculptureMeshes.push(mesh)
  }
  const workCloudGeometry = new THREE.BufferGeometry()
  const workCloudPositions = new Float32Array(2600 * 3)
  for (let index = 0; index < 2600; index += 1) {
    const offset = index * 3
    const angle = random() * Math.PI * 2
    const radius = 0.5 + random() * 2.2
    workCloudPositions[offset] = Math.cos(angle) * radius * 0.72
    workCloudPositions[offset + 1] = (random() - 0.5) * 7.4
    workCloudPositions[offset + 2] = Math.sin(angle) * radius
  }
  workCloudGeometry.setAttribute('position', new THREE.BufferAttribute(workCloudPositions, 3))
  const workCloudMaterial = new THREE.PointsMaterial({
    color: 0xbca7ff, size: 0.025, transparent: true, opacity: 0, depthWrite: false, blending: THREE.AdditiveBlending
  })
  workSculpture.add(new THREE.Points(workCloudGeometry, workCloudMaterial))

  const lab = new THREE.Group()
  lab.position.set(0, 0.1, -0.7)
  root.add(lab)
  const labKnotGeometry = new THREE.TorusKnotGeometry(1.34, 0.42, 220, 24, 2, 3)
  const labPointsMaterial = new THREE.PointsMaterial({
    color: 0xa9dfff, size: 0.025, transparent: true, opacity: 0, blending: THREE.AdditiveBlending, depthWrite: false
  })
  lab.add(new THREE.Points(labKnotGeometry, labPointsMaterial))
  const cageMaterial = new THREE.MeshPhysicalMaterial({
    color: 0x9aabd1, metalness: 0.85, roughness: 0.2, transparent: true, opacity: 0
  })
  const cageTop = new THREE.Mesh(new THREE.TorusGeometry(2.15, 0.025, 8, 120), cageMaterial)
  cageTop.rotation.x = Math.PI / 2
  cageTop.position.y = 2.35
  const cageBottom = cageTop.clone()
  cageBottom.position.y = -2.35
  lab.add(cageTop, cageBottom)
  for (let index = 0; index < 9; index += 1) {
    const angle = (index / 9) * Math.PI * 2
    const rail = new THREE.Mesh(new THREE.CylinderGeometry(0.012, 0.012, 4.7, 5), cageMaterial)
    rail.position.set(Math.cos(angle) * 2.15, 0, Math.sin(angle) * 2.15)
    lab.add(rail)
  }

  const trailCount = window.innerWidth < 700 ? 24 : 42
  const trailHistory = Array.from({ length: trailCount + 1 }, () => new THREE.Vector2(0, 0))
  const trailMeshes: any[] = []
  const trailMaterials: any[] = []
  const trailGeometry = new THREE.PlaneGeometry(1, 1)
  const trailGroup = new THREE.Group()
  trailGroup.position.z = 1.45
  scene.add(trailGroup)
  ;[0xf2fff8, 0x74ecdd, 0x9888ff].forEach((color: number, strandIndex: number) => {
    const material = new THREE.MeshBasicMaterial({
      color, transparent: true, opacity: 0, blending: THREE.AdditiveBlending, depthWrite: false, side: THREE.DoubleSide
    })
    const mesh = new THREE.InstancedMesh(trailGeometry, material, trailCount)
    mesh.frustumCulled = false
    mesh.renderOrder = 12 + strandIndex
    trailGroup.add(mesh)
    trailMeshes.push(mesh)
    trailMaterials.push(material)
  })
  const trailMatrix = new THREE.Matrix4()
  const trailQuaternion = new THREE.Quaternion()
  const trailPosition = new THREE.Vector3()
  const trailScale = new THREE.Vector3()
  const trailAxis = new THREE.Vector3(0, 0, 1)

  function updateTrail() {
    const target = new THREE.Vector2(pointerX * 5.05, pointerY * 2.96)
    trailHistory[0].lerp(target, reducedMotion ? 1 : 0.42)
    for (let index = 1; index < trailHistory.length; index += 1) {
      trailHistory[index].lerp(trailHistory[index - 1], reducedMotion ? 1 : 0.17 + Math.min(index, 12) * 0.0025)
    }
    trailMeshes.forEach((mesh: any, strandIndex: number) => {
      const side = strandIndex - 1
      for (let index = 0; index < trailCount; index += 1) {
        const start = trailHistory[index]
        const end = trailHistory[index + 1]
        const taper = 1 - index / trailCount
        const wave = Math.sin(index * 0.34 + elapsed * 2.2 + strandIndex) * 0.025 * taper
        const startX = start.x + side * (0.025 + index * 0.0007) + wave
        const startY = start.y + side * Math.cos(index * 0.25 + elapsed) * 0.012
        const endX = end.x + side * (0.025 + index * 0.0007)
        const endY = end.y + side * Math.cos((index + 1) * 0.25 + elapsed) * 0.012
        const deltaX = endX - startX
        const deltaY = endY - startY
        const length = Math.max(0.001, Math.sqrt(deltaX * deltaX + deltaY * deltaY))
        trailPosition.set((startX + endX) * 0.5, (startY + endY) * 0.5, -strandIndex * 0.008)
        trailQuaternion.setFromAxisAngle(trailAxis, Math.atan2(deltaY, deltaX))
        trailScale.set(length * 1.13, (0.012 + strandIndex * 0.004) * taper, 1)
        trailMatrix.compose(trailPosition, trailQuaternion, trailScale)
        mesh.setMatrixAt(index, trailMatrix)
      }
      mesh.instanceMatrix.needsUpdate = true
    })
  }

  function onPointerMove(event: PointerEvent) {
    pointerTargetX = event.clientX / Math.max(window.innerWidth, 1) * 2 - 1
    pointerTargetY = -(event.clientY / Math.max(window.innerHeight, 1) * 2 - 1)
    pointerInside = event.pointerType === 'touch' ? 0.7 : 1
    pointerEnergy = Math.min(1, pointerEnergy + 0.22)
  }
  const onPointerLeave = () => { pointerInside = 0 }
  function onVisibilityChange() {
    visible = !document.hidden
    lastTime = performance.now()
    if (visible && !stopped && !animationFrame) animationFrame = requestAnimationFrame(render)
  }
  const onReducedMotionChange = (event: MediaQueryListEvent) => { reducedMotion = event.matches }

  function resize() {
    const width = Math.max(1, stage.clientWidth)
    const height = Math.max(1, stage.clientHeight)
    const pixelRatio = Math.min(window.devicePixelRatio || 1, width < 700 ? 1.25 : 1.65)
    renderer.setPixelRatio(pixelRatio)
    renderer.setSize(width, height, false)
    composer?.setPixelRatio?.(pixelRatio)
    composer?.setSize(width, height)
    camera.aspect = width / height
    camera.updateProjectionMatrix()
    particleMaterial.uniforms.uPixelRatio.value = pixelRatio
  }

  function updateScene(delta: number) {
    progress += (requestedProgress - progress) * (reducedMotion ? 1 : Math.min(1, delta * 4.8))
    const scrollImpulse = Math.min(1, Math.abs(progress - previousProgress) / Math.max(delta, 0.001) * 0.12)
    previousProgress = progress
    pointerX += (pointerTargetX - pointerX) * (reducedMotion ? 1 : Math.min(1, delta * 5.8))
    pointerY += (pointerTargetY - pointerY) * (reducedMotion ? 1 : Math.min(1, delta * 5.8))
    pointerEnergy += (0 - pointerEnergy) * Math.min(1, delta * 2.4)
    pointerInside += (0 - pointerInside) * Math.min(1, delta * 0.18)
    if (!reducedMotion) elapsed += delta
    particleMaterial.uniforms.uTime.value = elapsed
    particleMaterial.uniforms.uPointer.value.set(pointerX, pointerY)

    const missionOut = 1 - smoothstep(0.34, 0.43, progress)
    const workVisibility = smoothstep(0.25, 0.39, progress) * (1 - smoothstep(0.7, 0.79, progress))
    const labVisibility = smoothstep(0.69, 0.8, progress) * (1 - smoothstep(0.88, 0.95, progress))
    const contactVisibility = smoothstep(0.86, 0.97, progress)
    const organismVisibility = Math.max(missionOut, contactVisibility * 0.82)

    particleMaterial.uniforms.uOpacity.value = mix(0.76, 0.34, workVisibility) + labVisibility * 0.08 + contactVisibility * 0.14
    particleField.rotation.y = elapsed * 0.018 + progress * 0.28
    particleField.rotation.z = Math.sin(elapsed * 0.1) * 0.012
    particleField.scale.setScalar(1 + scrollImpulse * 0.025)
    fogSprites.forEach((sprite: any, index: number) => {
      sprite.material.opacity = Number(fogSettings[index][5]) * (0.75 + organismVisibility * 0.45)
      sprite.position.x += Math.sin(elapsed * 0.08 + index) * delta * 0.018
      sprite.position.y += Math.cos(elapsed * 0.07 + index * 1.7) * delta * 0.012
    })

    const heroShift = smoothstep(0.09, 0.3, progress)
    const workShift = smoothstep(0.29, 0.52, progress)
    const labShift = smoothstep(0.69, 0.82, progress)
    const contactShift = smoothstep(0.86, 0.98, progress)
    core.position.set(mix(0, 1.05, heroShift), mix(0.1, 0.62, heroShift), mix(0, -0.45, heroShift))
    let coreScale = mix(1, 1.12, heroShift)
    if (workShift > 0) {
      core.position.x = mix(core.position.x, -4.4, workShift)
      core.position.y = mix(core.position.y, 1.1, workShift)
      core.position.z = mix(core.position.z, -2.2, workShift)
      coreScale = mix(coreScale, 0.36, workShift)
    }
    if (labShift > 0) {
      core.position.x = mix(core.position.x, 3.9, labShift)
      core.position.y = mix(core.position.y, -1.8, labShift)
      coreScale = mix(coreScale, 0.24, labShift)
    }
    if (contactShift > 0) {
      core.position.x = mix(core.position.x, 0, contactShift)
      core.position.y = mix(core.position.y, 0, contactShift)
      core.position.z = mix(core.position.z, 0.4, contactShift)
      coreScale = mix(coreScale, 1.68, contactShift)
    }
    core.scale.setScalar(coreScale)
    core.rotation.y = elapsed * 0.16 + pointerX * 0.2
    core.rotation.x = -pointerY * 0.15 + Math.sin(elapsed * 0.28) * 0.03
    outerRing.rotation.z = elapsed * 0.13
    cyanRing.rotation.z = -elapsed * 0.22
    orbitRing.rotation.z = elapsed * 0.08
    crown.rotation.z = -elapsed * 0.09

    organism.rotation.y = pointerX * 0.11
    organism.rotation.x = -pointerY * 0.065
    organism.scale.setScalar(1 + pointerEnergy * 0.028 + contactVisibility * 0.11)
    fiberGroups[0].rotation.z = -pointerY * 0.13 - pointerX * 0.045
    fiberGroups[1].rotation.z = pointerY * 0.13 - pointerX * 0.045
    fiberGroups[0].rotation.y = pointerX * 0.12
    fiberGroups[1].rotation.y = pointerX * 0.12
    fiberMaterials.forEach((material: any, index: number) => {
      material.opacity = Math.max(0.04, organismVisibility * (0.82 + index * 0.04))
    })

    sculptureMaterial.opacity = workVisibility * 0.92
    sculptureAccentMaterial.opacity = workVisibility * 0.88
    workCloudMaterial.opacity = workVisibility * 0.54
    workSculpture.visible = workVisibility > 0.015
    workSculpture.scale.setScalar(0.74 + workVisibility * 0.42)
    workSculpture.rotation.y = elapsed * 0.12 + progress * 3.2
    sculptureMeshes.forEach((mesh: any, index: number) => {
      mesh.rotation.x += delta * (0.08 + index % 4 * 0.018)
      mesh.rotation.y += delta * (0.07 + index % 5 * 0.016)
    })

    labPointsMaterial.opacity = labVisibility * 0.82
    cageMaterial.opacity = labVisibility * 0.32
    lab.visible = labVisibility > 0.015
    lab.rotation.y = elapsed * 0.085 + pointerX * 0.18
    lab.rotation.x = Math.sin(elapsed * 0.14) * 0.08 - pointerY * 0.08
    lab.scale.setScalar(1.05 + labVisibility * 0.34)

    const trailVisibility = Math.max(0, missionOut - workVisibility * 0.8) * Math.min(1, pointerInside + pointerEnergy)
    trailMaterials.forEach((material: any, index: number) => {
      material.opacity = trailVisibility * (index === 0 ? 0.72 : 0.44)
    })
    updateTrail()
    camera.position.x = pointerX * 0.18 + Math.sin(elapsed * 0.09) * 0.04
    camera.position.y = 0.24 + pointerY * 0.12
    camera.position.z = 8.2 - heroShift * 0.2 + workVisibility * 0.38 - labVisibility * 0.22 - contactVisibility * 0.68
    camera.lookAt(0, 0.1, -0.7)
    cyanLight.position.x = 3.6 + pointerX * 1.2
    cyanLight.position.y = 1.7 + pointerY * 0.8
    limeLight.intensity = 10 + Math.sin(elapsed * 0.52) * 2.2 + scrollImpulse * 7
    violetLight.intensity = 8 + workVisibility * 8
    if (bloomPass) {
      bloomPass.strength = 0.62 + scrollImpulse * 0.44 + pointerEnergy * 0.2 + workVisibility * 0.16
      bloomPass.radius = 0.62 + labVisibility * 0.12
    }
  }

  function render(now: number) {
    animationFrame = 0
    if (stopped || !visible) return
    const delta = Math.min(0.05, Math.max(0.001, (now - lastTime) / 1000))
    lastTime = now
    updateScene(delta)
    if (composer) composer.render()
    else renderer.render(scene, camera)
    animationFrame = requestAnimationFrame(render)
  }

  resize()
  updateScene(0.016)
  if (composer) composer.render()
  else renderer.render(scene, camera)
  window.addEventListener('resize', resize, { passive: true })
  window.addEventListener('pointermove', onPointerMove, { passive: true })
  window.addEventListener('pointerleave', onPointerLeave)
  document.addEventListener('visibilitychange', onVisibilityChange)
  if (typeof reducedMotionQuery.addEventListener === 'function') reducedMotionQuery.addEventListener('change', onReducedMotionChange)
  else reducedMotionQuery.addListener(onReducedMotionChange)
  animationFrame = requestAnimationFrame(render)

  return () => {
    stopped = true
    cancelAnimationFrame(animationFrame)
    window.removeEventListener('resize', resize)
    window.removeEventListener('pointermove', onPointerMove)
    window.removeEventListener('pointerleave', onPointerLeave)
    document.removeEventListener('visibilitychange', onVisibilityChange)
    if (typeof reducedMotionQuery.removeEventListener === 'function') reducedMotionQuery.removeEventListener('change', onReducedMotionChange)
    else reducedMotionQuery.removeListener(onReducedMotionChange)
    const geometries = new Set<any>()
    const materials = new Set<any>()
    const textures = new Set<any>()
    scene.traverse((object: any) => {
      if (object.geometry) geometries.add(object.geometry)
      const objectMaterials = Array.isArray(object.material) ? object.material : [object.material]
      objectMaterials.filter(Boolean).forEach((material: any) => {
        materials.add(material)
        Object.values(material).forEach((value: any) => {
          if (value?.isTexture) textures.add(value)
        })
      })
    })
    geometries.forEach(geometry => geometry.dispose())
    materials.forEach(material => material.dispose())
    textures.forEach(texture => texture.dispose())
    fogTexture.dispose()
    emblemTexture.dispose()
    composer?.dispose?.()
    renderer.dispose()
    renderer.forceContextLoss()
    renderer.domElement.remove()
    scene.clear()
  }
}
</script>

<style scoped>
.kinetic-scene {
  position: absolute;
  inset: 0;
  overflow: hidden;
  background: #04090b;
  isolation: isolate;
}
.kinetic-scene-canvas,
.kinetic-scene-canvas :deep(canvas) {
  width: 100%;
  height: 100%;
}
.kinetic-scene-canvas :deep(canvas) {
  display: block;
}
.kinetic-scene-vignette,
.kinetic-scene-chromatic,
.kinetic-scene-grain {
  position: absolute;
  inset: 0;
  pointer-events: none;
}
.kinetic-scene-vignette {
  background:
    radial-gradient(circle at 50% 43%, transparent 26%, rgba(3, 8, 10, 0.16) 58%, rgba(3, 7, 9, 0.72) 118%),
    linear-gradient(90deg, rgba(9, 38, 39, 0.18), transparent 24%, transparent 76%, rgba(13, 38, 43, 0.2));
}
.kinetic-scene-chromatic {
  opacity: 0.2;
  background:
    radial-gradient(circle at 8% 9%, rgba(78, 191, 176, 0.2), transparent 21%),
    radial-gradient(circle at 89% 79%, rgba(81, 95, 160, 0.17), transparent 25%);
  mix-blend-mode: screen;
}
.kinetic-scene-grain {
  z-index: 4;
  opacity: 0.08;
  background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 180 180' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='.93' numOctaves='3' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)' opacity='.62'/%3E%3C/svg%3E");
  mix-blend-mode: soft-light;
  transform: scale(1.4);
}
.kinetic-scene-fallback {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  overflow: hidden;
}
.kinetic-fallback-orbit {
  position: relative;
  z-index: 2;
  display: grid;
  width: min(26vw, 210px);
  aspect-ratio: 1;
  place-items: center;
  border: 2px solid rgba(220, 255, 244, 0.82);
  border-radius: 50%;
  color: #ecfff9;
  box-shadow: 0 0 0 14px rgba(95, 230, 222, 0.08), 0 0 52px rgba(100, 220, 213, 0.16), inset 0 0 40px rgba(98, 125, 255, 0.14);
  font: 700 clamp(34px, 6vw, 72px)/1 ui-monospace, SFMono-Regular, Menlo, monospace;
}
.kinetic-fallback-orbit::after {
  position: absolute;
  inset: 14%;
  border: 1px solid rgba(120, 228, 225, 0.55);
  border-radius: inherit;
  content: '';
}
.kinetic-fallback-fibers {
  position: absolute;
  top: 15%;
  width: 48%;
  height: 70%;
  border: 1px solid rgba(220, 255, 244, 0.28);
  border-radius: 50%;
  filter: drop-shadow(0 0 7px rgba(171, 255, 232, 0.18));
}
.kinetic-fallback-fibers-left {
  left: -10%;
  transform: rotate(18deg);
}
.kinetic-fallback-fibers-right {
  right: -10%;
  transform: rotate(-18deg);
}
.kinetic-fallback-dust {
  position: absolute;
  inset: 10%;
  opacity: 0.46;
  background-image: radial-gradient(circle, rgba(186, 255, 183, 0.78) 0 1px, transparent 1.5px);
  background-size: 23px 27px;
  mask-image: radial-gradient(circle, transparent 0 14%, #000 58%, transparent 94%);
}
@media (max-width: 700px) {
  .kinetic-scene-vignette {
    background: radial-gradient(circle at 50% 38%, transparent 20%, rgba(3, 8, 10, 0.2) 58%, rgba(3, 7, 9, 0.72) 110%);
  }
  .kinetic-scene-chromatic { opacity: 0.13; }
}
@media (prefers-reduced-motion: reduce) {
  .kinetic-scene-grain { display: none; }
}
</style>
