<template>
  <div
    ref="stageRef"
    class="hero-orbit-stage"
    role="img"
    aria-label="ModuRelay Relay Core interactive visualization"
  >
    <div class="scene-shell" :class="{ 'scene-is-loading': isLoading, 'scene-has-fallback': hasFallback }">
      <div class="scene-atmosphere" aria-hidden="true">
        <span class="scene-atmosphere-glow scene-atmosphere-glow-one"></span>
        <span class="scene-atmosphere-glow scene-atmosphere-glow-two"></span>
        <span class="scene-depth-vignette"></span>
        <span class="scene-depth-sweep scene-depth-sweep-one"></span>
        <span class="scene-depth-sweep scene-depth-sweep-two"></span>
        <i class="scene-atmosphere-star scene-atmosphere-star-one"></i>
        <i class="scene-atmosphere-star scene-atmosphere-star-two"></i>
        <i class="scene-atmosphere-star scene-atmosphere-star-three"></i>
        <i class="scene-atmosphere-star scene-atmosphere-star-four"></i>
      </div>

      <div ref="canvasHostRef" class="three-canvas-host" aria-hidden="true"></div>

      <div v-if="isLoading" class="scene-loading" aria-hidden="true">
        <span></span>
        <small>BUILDING RELAY FIELD</small>
      </div>

      <div v-if="hasFallback" class="fallback-core" aria-hidden="true">
        <div class="fallback-aura"></div>
        <div class="fallback-shadow"></div>
        <div class="fallback-ring fallback-ring-one"></div>
        <div class="fallback-ring fallback-ring-two"></div>
        <span class="fallback-signal fallback-signal-one"></span>
        <span class="fallback-signal fallback-signal-two"></span>
        <span class="fallback-signal fallback-signal-three"></span>
        <div class="fallback-emblem"><span>M</span><small>RELAY CORE</small></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { isDarkTheme, loadThree, observeTheme } from '@/utils/threeRuntime'

type Disposable = { dispose: () => void }

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

function readThemeToken(name: string, fallback: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback
}

function colorHex(THREE: any, value: string, fallback: string): number {
  const color = new THREE.Color()
  try {
    color.set(value || fallback)
  } catch {
    color.set(fallback)
  }
  return color.getHex()
}

function createEmblemTexture(THREE: any, dark: boolean) {
  const canvas = document.createElement('canvas')
  canvas.width = 512
  canvas.height = 512
  const context = canvas.getContext('2d')

  if (!context) throw new Error('Unable to create Relay Core texture canvas')

  const texture = new THREE.CanvasTexture(canvas)
  texture.colorSpace = THREE.SRGBColorSpace
  texture.anisotropy = 8

  const redraw = (isDark: boolean) => {
    const gradient = context.createLinearGradient(72, 48, 440, 470)
    gradient.addColorStop(0, isDark ? '#1b2345' : '#11182d')
    gradient.addColorStop(0.48, isDark ? '#172b59' : '#14234a')
    gradient.addColorStop(1, isDark ? '#071b2c' : '#081628')

    context.clearRect(0, 0, 512, 512)
    context.fillStyle = gradient
    context.fillRect(0, 0, 512, 512)

    context.strokeStyle = isDark ? 'rgba(165, 180, 252, 0.84)' : 'rgba(148, 163, 184, 0.76)'
    context.lineWidth = 8
    context.strokeRect(26, 26, 460, 460)

    const centerGlow = context.createRadialGradient(180, 130, 8, 260, 248, 260)
    centerGlow.addColorStop(0, isDark ? 'rgba(34, 211, 238, 0.32)' : 'rgba(99, 102, 241, 0.24)')
    centerGlow.addColorStop(0.62, 'rgba(9, 12, 18, 0)')
    context.fillStyle = centerGlow
    context.fillRect(0, 0, 512, 512)

    context.fillStyle = isDark ? '#f8fafc' : '#eef2ff'
    context.textAlign = 'center'
    context.textBaseline = 'middle'
    context.font = '900 250px Inter, system-ui, sans-serif'
    context.shadowColor = isDark ? 'rgba(103, 232, 249, 0.7)' : 'rgba(99, 102, 241, 0.52)'
    context.shadowBlur = 30
    context.fillText('M', 256, 238)

    context.shadowBlur = 0
    context.fillStyle = isDark ? 'rgba(226, 232, 240, 0.84)' : 'rgba(224, 231, 255, 0.86)'
    context.font = '700 26px Inter, system-ui, sans-serif'
    context.fillText('RELAY CORE', 256, 408)

    texture.needsUpdate = true
  }

  redraw(dark)
  return { texture, redraw }
}

function createFlowMaterial(THREE: any, primary: string, accent: string, opacity: number) {
  return new THREE.ShaderMaterial({
    uniforms: {
      uTime: { value: 0 },
      uPrimary: { value: new THREE.Color(primary) },
      uAccent: { value: new THREE.Color(accent) },
      uOpacity: { value: opacity },
      uMotion: { value: 1 },
    },
    vertexShader: `
      varying vec2 vUv;

      void main() {
        vUv = uv;
        gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
      }
    `,
    fragmentShader: `
      uniform float uTime;
      uniform vec3 uPrimary;
      uniform vec3 uAccent;
      uniform float uOpacity;
      uniform float uMotion;
      varying vec2 vUv;

      void main() {
        float flow = 0.5 + 0.5 * sin(vUv.x * 26.0 - uTime * 2.3 * uMotion);
        float highlight = smoothstep(0.58, 1.0, flow);
        vec3 color = mix(uPrimary, uAccent, clamp(vUv.x * 0.78 + highlight * 0.22, 0.0, 1.0));
        float edge = 0.42 + 0.58 * pow(abs(sin(vUv.y * 3.14159)), 0.42);
        float alpha = uOpacity * edge * (0.48 + highlight * 0.52);
        gl_FragColor = vec4(color, alpha);
      }
    `,
    transparent: true,
    depthWrite: false,
    blending: THREE.AdditiveBlending,
    side: THREE.DoubleSide,
  })
}

function createShellMaterial(THREE: any, primary: string, accent: string) {
  return new THREE.ShaderMaterial({
    uniforms: {
      uTime: { value: 0 },
      uPointer: { value: new THREE.Vector2(0, 0) },
      uPrimary: { value: new THREE.Color(primary) },
      uAccent: { value: new THREE.Color(accent) },
      uOpacity: { value: 0.86 },
      uMotion: { value: 1 },
    },
    vertexShader: `
      uniform float uTime;
      uniform vec2 uPointer;
      uniform float uMotion;
      varying vec3 vNormal;
      varying vec3 vWorldPosition;

      void main() {
        vec3 displaced = position;
        float wave = sin(position.y * 4.8 + uTime * 0.78 * uMotion + position.x * 2.1);
        float swell = sin(position.z * 5.2 - uTime * 0.46 * uMotion + position.y * 1.7);
        float touch = dot(normalize(position), vec3(uPointer * 0.13, 0.0));
        displaced += normal * (wave * 0.035 + swell * 0.026 + touch * 0.065);

        vec4 worldPosition = modelMatrix * vec4(displaced, 1.0);
        vWorldPosition = worldPosition.xyz;
        vNormal = normalize(normalMatrix * normal);
        gl_Position = projectionMatrix * viewMatrix * worldPosition;
      }
    `,
    fragmentShader: `
      uniform vec3 uPrimary;
      uniform vec3 uAccent;
      uniform float uOpacity;
      varying vec3 vNormal;
      varying vec3 vWorldPosition;

      void main() {
        vec3 viewDirection = normalize(cameraPosition - vWorldPosition);
        float fresnel = pow(1.0 - max(dot(normalize(vNormal), viewDirection), 0.0), 2.45);
        float band = 0.5 + 0.5 * sin(vWorldPosition.y * 8.0 + vWorldPosition.x * 2.8);
        vec3 color = mix(uPrimary, uAccent, clamp(fresnel * 0.8 + band * 0.2, 0.0, 1.0));
        float alpha = (0.035 + fresnel * 0.56 + band * 0.045) * uOpacity;
        gl_FragColor = vec4(color, alpha);
      }
    `,
    transparent: true,
    depthWrite: false,
    blending: THREE.AdditiveBlending,
    side: THREE.FrontSide,
  })
}

function createParticleMaterial(THREE: any, primary: string, accent: string, pixelRatio: number) {
  return new THREE.ShaderMaterial({
    uniforms: {
      uTime: { value: 0 },
      uPrimary: { value: new THREE.Color(primary) },
      uAccent: { value: new THREE.Color(accent) },
      uPixelRatio: { value: pixelRatio },
      uMotion: { value: 1 },
    },
    vertexShader: `
      attribute float aSize;
      attribute float aSeed;
      attribute float aAlpha;
      uniform float uTime;
      uniform float uPixelRatio;
      uniform float uMotion;
      varying float vAlpha;
      varying float vSeed;

      void main() {
        vec3 transformed = position;
        float drift = uTime * (0.42 + aSeed * 0.24) * uMotion;
        transformed += vec3(
          sin(drift + aSeed * 18.0) * 0.035,
          cos(drift * 1.2 + aSeed * 9.0) * 0.045,
          sin(drift * 0.8 + aSeed * 14.0) * 0.028
        ) * uMotion;

        vec4 mvPosition = modelViewMatrix * vec4(transformed, 1.0);
        gl_PointSize = aSize * uPixelRatio * (72.0 / max(-mvPosition.z, 1.0));
        gl_Position = projectionMatrix * mvPosition;
        vAlpha = aAlpha * (0.7 + 0.3 * sin(drift + aSeed * 11.0));
        vSeed = aSeed;
      }
    `,
    fragmentShader: `
      uniform vec3 uPrimary;
      uniform vec3 uAccent;
      varying float vAlpha;
      varying float vSeed;

      void main() {
        float distanceToCenter = distance(gl_PointCoord, vec2(0.5));
        float softness = smoothstep(0.5, 0.0, distanceToCenter);
        vec3 color = mix(uPrimary, uAccent, 0.35 + fract(vSeed * 7.0) * 0.5);
        gl_FragColor = vec4(color, softness * vAlpha * 0.9);
      }
    `,
    transparent: true,
    depthWrite: false,
    blending: THREE.AdditiveBlending,
  })
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

  const dark = isDarkTheme()
  const primaryToken = readThemeToken('--color-primary', dark ? '#6366f1' : '#4f46e5')
  const accentToken = readThemeToken('--color-accent', dark ? '#22d3ee' : '#0891b2')
  const primaryHex = colorHex(THREE, primaryToken, dark ? '#6366f1' : '#4f46e5')
  const accentHex = colorHex(THREE, accentToken, dark ? '#22d3ee' : '#0891b2')
  const sceneBackground = colorHex(THREE, '#080b13', '#080b13')
  const pixelRatio = Math.min(window.devicePixelRatio, window.innerWidth < 768 ? 1.2 : 1.45)

  const resources: Disposable[] = []
  const ringMaterials: Array<{ material: any; index: number }> = []
  const nodeMaterials: any[] = []
  const flowMaterials: any[] = []

  const renderer = new THREE.WebGLRenderer({
    alpha: true,
    antialias: window.devicePixelRatio <= 1.5,
    powerPreference: 'high-performance',
    premultipliedAlpha: true,
  })
  renderer.setPixelRatio(pixelRatio)
  renderer.outputColorSpace = THREE.SRGBColorSpace
  renderer.toneMapping = THREE.ACESFilmicToneMapping
  renderer.toneMappingExposure = 1.2
  renderer.setClearColor(sceneBackground, 0)
  renderer.domElement.className = 'three-hero-canvas'
  renderer.domElement.setAttribute('aria-hidden', 'true')
  host.appendChild(renderer.domElement)

  const scene = new THREE.Scene()
  scene.background = null
  scene.fog = new THREE.FogExp2(sceneBackground, 0.044)

  const camera = new THREE.PerspectiveCamera(40, 1, 0.1, 60)
  camera.position.set(0, 0.22, 8.6)

  const world = new THREE.Group()
  const coreGroup = new THREE.Group()
  const ribbonGroup = new THREE.Group()
  const particleGroup = new THREE.Group()
  const orbitGroup = new THREE.Group()
  const signalGroup = new THREE.Group()
  world.add(coreGroup, ribbonGroup, particleGroup, orbitGroup, signalGroup)
  world.position.y = 0.08
  scene.add(world)

  const ambientLight = new THREE.HemisphereLight(0x6173bd, 0x02040a, 1.55)
  const keyLight = new THREE.DirectionalLight(0xf4f7ff, 3.25)
  keyLight.position.set(4.5, 5.8, 7)
  const primaryLight = new THREE.PointLight(primaryHex, 30, 14, 2)
  primaryLight.position.set(-3.1, 1.9, 3.4)
  const accentLight = new THREE.PointLight(accentHex, 28, 15, 2)
  accentLight.position.set(3.5, -1.2, 2.7)
  const topLight = new THREE.PointLight(0xa5b4fc, 15, 12, 2)
  topLight.position.set(0, 4.2, -0.5)
  scene.add(ambientLight, keyLight, primaryLight, accentLight, topLight)

  const emblemTexture = createEmblemTexture(THREE, dark)
  resources.push(emblemTexture.texture)

  const coreGeometry = new THREE.SphereGeometry(1.22, 64, 44)
  const coreMaterial = new THREE.MeshPhysicalMaterial({
    color: 0x111627,
    metalness: 0.82,
    roughness: 0.2,
    clearcoat: 1,
    clearcoatRoughness: 0.08,
    emissive: primaryHex,
    emissiveIntensity: 0.11,
  })
  resources.push(coreGeometry, coreMaterial)
  const coreBody = new THREE.Mesh(coreGeometry, coreMaterial)
  coreBody.scale.set(1, 1, 0.88)
  coreBody.position.y = 0.04
  coreGroup.add(coreBody)

  const shellGeometry = new THREE.IcosahedronGeometry(1.48, 5)
  const shellMaterial = createShellMaterial(THREE, primaryToken, accentToken)
  resources.push(shellGeometry, shellMaterial)
  const shell = new THREE.Mesh(shellGeometry, shellMaterial)
  shell.position.y = 0.04
  shell.renderOrder = 2
  coreGroup.add(shell)

  const emblemBodyGeometry = new THREE.CylinderGeometry(0.79, 0.79, 0.12, 64)
  const emblemBodyMaterial = new THREE.MeshPhysicalMaterial({
    color: 0x090d18,
    metalness: 0.9,
    roughness: 0.18,
    clearcoat: 1,
    clearcoatRoughness: 0.06,
    emissive: accentHex,
    emissiveIntensity: 0.08,
  })
  resources.push(emblemBodyGeometry, emblemBodyMaterial)
  const emblemBody = new THREE.Mesh(emblemBodyGeometry, emblemBodyMaterial)
  emblemBody.rotation.x = Math.PI / 2
  emblemBody.position.set(0, 0.04, 0.93)
  coreGroup.add(emblemBody)

  const emblemFaceGeometry = new THREE.CircleGeometry(0.72, 64)
  const emblemFaceMaterial = new THREE.MeshBasicMaterial({ map: emblemTexture.texture, transparent: true })
  resources.push(emblemFaceGeometry, emblemFaceMaterial)
  const emblemFace = new THREE.Mesh(emblemFaceGeometry, emblemFaceMaterial)
  emblemFace.position.set(0, 0.04, 1.01)
  coreGroup.add(emblemFace)

  const focalRingGeometry = new THREE.TorusGeometry(1.48, 0.064, 24, 180)
  const focalRingMaterial = new THREE.MeshPhysicalMaterial({
    color: accentHex,
    metalness: 0.88,
    roughness: 0.16,
    clearcoat: 1,
    clearcoatRoughness: 0.06,
    emissive: primaryHex,
    emissiveIntensity: 0.46,
  })
  resources.push(focalRingGeometry, focalRingMaterial)
  const focalRing = new THREE.Mesh(focalRingGeometry, focalRingMaterial)
  focalRing.rotation.z = -0.08
  focalRing.position.z = 0.03
  orbitGroup.add(focalRing)
  ringMaterials.push({ material: focalRingMaterial, index: 0 })

  const ringConfigs = [
    { radius: 1.68, tube: 0.018, rotation: [0.12, 0.44, 0.2], color: primaryHex, opacity: 0.62, speed: 0.12 },
    { radius: 1.96, tube: 0.012, rotation: [1.22, -0.32, -0.38], color: accentHex, opacity: 0.48, speed: -0.085 },
    { radius: 2.25, tube: 0.009, rotation: [0.46, 1.16, 0.72], color: 0xa5b4fc, opacity: 0.28, speed: 0.052 },
  ]
  const ringMeshes: Array<{ mesh: any; speed: number }> = []
  ringConfigs.forEach((config, index) => {
    const geometry = new THREE.TorusGeometry(config.radius, config.tube, 12, 200)
    const material = new THREE.MeshBasicMaterial({
      color: config.color,
      transparent: true,
      opacity: config.opacity,
      depthWrite: false,
      blending: THREE.AdditiveBlending,
    })
    resources.push(geometry, material)
    const mesh = new THREE.Mesh(geometry, material)
    mesh.rotation.set(...config.rotation)
    orbitGroup.add(mesh)
    ringMeshes.push({ mesh, speed: config.speed })
    ringMaterials.push({ material, index: index + 1 })
  })

  const shadowGeometry = new THREE.CircleGeometry(2.55, 96)
  const shadowMaterial = new THREE.MeshBasicMaterial({ color: 0x010207, transparent: true, opacity: 0.72, depthWrite: false })
  resources.push(shadowGeometry, shadowMaterial)
  const shadow = new THREE.Mesh(shadowGeometry, shadowMaterial)
  shadow.rotation.x = -Math.PI / 2
  shadow.scale.set(1, 0.28, 1)
  shadow.position.set(0, -1.68, 0.36)
  orbitGroup.add(shadow)

  const shadowRingGeometry = new THREE.TorusGeometry(2.05, 0.012, 8, 160)
  const shadowRingMaterial = new THREE.MeshBasicMaterial({ color: primaryHex, transparent: true, opacity: 0.24, depthWrite: false, blending: THREE.AdditiveBlending })
  resources.push(shadowRingGeometry, shadowRingMaterial)
  const shadowRing = new THREE.Mesh(shadowRingGeometry, shadowRingMaterial)
  shadowRing.rotation.x = Math.PI / 2
  shadowRing.position.set(0, -1.64, 0.34)
  shadowRing.scale.set(1.12, 0.36, 1)
  orbitGroup.add(shadowRing)
  ringMaterials.push({ material: shadowRingMaterial, index: 4 })

  const curves = [
    new THREE.CatmullRomCurve3([
      new THREE.Vector3(-3.7, -2.4, -0.6),
      new THREE.Vector3(-2.6, -1.62, 0.25),
      new THREE.Vector3(-1.38, -0.28, -0.76),
      new THREE.Vector3(-0.72, 1.42, 0.28),
      new THREE.Vector3(0.35, 2.42, -0.38),
      new THREE.Vector3(1.94, 1.54, 0.58),
      new THREE.Vector3(3.45, 0.14, -0.24),
      new THREE.Vector3(2.42, -1.44, 0.44),
      new THREE.Vector3(0.68, -2.58, -0.48),
      new THREE.Vector3(-1.54, -1.9, 0.3),
      new THREE.Vector3(-2.9, -0.74, -0.58),
    ], false, 'catmullrom', 0.62),
    new THREE.CatmullRomCurve3([
      new THREE.Vector3(-3.34, 1.12, 0.38),
      new THREE.Vector3(-2.04, 1.9, -0.34),
      new THREE.Vector3(-0.7, 1.05, 0.62),
      new THREE.Vector3(0.92, -0.46, -0.68),
      new THREE.Vector3(2.3, -1.86, 0.16),
      new THREE.Vector3(3.42, -0.64, -0.5),
      new THREE.Vector3(2.5, 1.06, 0.28),
      new THREE.Vector3(0.94, 2.52, -0.3),
      new THREE.Vector3(-0.62, 1.58, 0.54),
      new THREE.Vector3(-2.14, 0.04, -0.62),
      new THREE.Vector3(-3.44, -1.68, 0.3),
    ], false, 'catmullrom', 0.6),
  ]

  curves.forEach((curve, index) => {
    const material = createFlowMaterial(THREE, index === 0 ? primaryToken : accentToken, index === 0 ? accentToken : primaryToken, index === 0 ? 0.72 : 0.6)
    const geometry = new THREE.TubeGeometry(curve, 180, index === 0 ? 0.026 : 0.02, 10, false)
    resources.push(geometry, material)
    flowMaterials.push(material)
    const ribbon = new THREE.Mesh(geometry, material)
    ribbon.renderOrder = 3
    ribbonGroup.add(ribbon)

    const glowMaterial = new THREE.MeshBasicMaterial({
      color: index === 0 ? accentHex : primaryHex,
      transparent: true,
      opacity: 0.1,
      depthWrite: false,
      blending: THREE.AdditiveBlending,
    })
    const glowGeometry = new THREE.TubeGeometry(curve, 120, index === 0 ? 0.072 : 0.058, 8, false)
    resources.push(glowGeometry, glowMaterial)
    const ribbonGlow = new THREE.Mesh(glowGeometry, glowMaterial)
    ribbonGlow.renderOrder = 1
    ribbonGroup.add(ribbonGlow)
  })

  const particleCount = window.innerWidth < 768 ? 230 : 430
  const particlePositions = new Float32Array(particleCount * 3)
  const particleSizes = new Float32Array(particleCount)
  const particleSeeds = new Float32Array(particleCount)
  const particleAlphas = new Float32Array(particleCount)
  for (let index = 0; index < particleCount; index += 1) {
    const curve = curves[index % curves.length]
    const t = ((index * 0.61803398875) % 1 + (Math.random() - 0.5) * 0.04 + 1) % 1
    const point = curve.getPointAt(t)
    const spread = 0.05 + Math.random() * 0.22
    particlePositions[index * 3] = point.x + (Math.random() - 0.5) * spread
    particlePositions[index * 3 + 1] = point.y + (Math.random() - 0.5) * spread
    particlePositions[index * 3 + 2] = point.z + (Math.random() - 0.5) * spread
    particleSizes[index] = 1.6 + Math.random() * 3.8
    particleSeeds[index] = Math.random()
    particleAlphas[index] = 0.2 + Math.random() * 0.68
  }
  const particleGeometry = new THREE.BufferGeometry()
  particleGeometry.setAttribute('position', new THREE.BufferAttribute(particlePositions, 3))
  particleGeometry.setAttribute('aSize', new THREE.BufferAttribute(particleSizes, 1))
  particleGeometry.setAttribute('aSeed', new THREE.BufferAttribute(particleSeeds, 1))
  particleGeometry.setAttribute('aAlpha', new THREE.BufferAttribute(particleAlphas, 1))
  const particleMaterial = createParticleMaterial(THREE, primaryToken, accentToken, pixelRatio)
  resources.push(particleGeometry, particleMaterial)
  const particleCloud = new THREE.Points(particleGeometry, particleMaterial)
  particleCloud.renderOrder = 4
  particleGroup.add(particleCloud)

  const ambientCount = window.innerWidth < 768 ? 64 : 106
  const ambientPositions = new Float32Array(ambientCount * 3)
  for (let index = 0; index < ambientCount; index += 1) {
    const angle = Math.random() * Math.PI * 2
    const radius = 3.4 + Math.random() * 1.85
    ambientPositions[index * 3] = Math.cos(angle) * radius
    ambientPositions[index * 3 + 1] = (Math.random() - 0.5) * 4.8
    ambientPositions[index * 3 + 2] = Math.sin(angle) * radius - 0.8
  }
  const ambientGeometry = new THREE.BufferGeometry()
  ambientGeometry.setAttribute('position', new THREE.BufferAttribute(ambientPositions, 3))
  const ambientMaterial = new THREE.PointsMaterial({
    color: accentHex,
    size: window.innerWidth < 768 ? 0.025 : 0.032,
    transparent: true,
    opacity: 0.36,
    depthWrite: false,
    blending: THREE.AdditiveBlending,
    sizeAttenuation: true,
  })
  resources.push(ambientGeometry, ambientMaterial)
  const ambientCloud = new THREE.Points(ambientGeometry, ambientMaterial)
  particleGroup.add(ambientCloud)

  const signalGeometry = new THREE.SphereGeometry(1, 24, 18)
  resources.push(signalGeometry)
  const signalConfigs = [
    { curve: curves[0], t: 0.08, color: primaryHex, size: 0.09, phase: 0.2 },
    { curve: curves[0], t: 0.62, color: accentHex, size: 0.12, phase: 1.6 },
    { curve: curves[1], t: 0.26, color: 0xa5b4fc, size: 0.075, phase: 3.2 },
    { curve: curves[1], t: 0.78, color: accentHex, size: 0.1, phase: 4.4 },
  ]
  const signals = signalConfigs.map((config) => {
    const material = new THREE.MeshPhysicalMaterial({
      color: config.color,
      metalness: 0.42,
      roughness: 0.14,
      clearcoat: 1,
      clearcoatRoughness: 0.08,
      emissive: config.color,
      emissiveIntensity: 0.64,
    })
    resources.push(material)
    nodeMaterials.push(material)
    const mesh = new THREE.Mesh(signalGeometry, material)
    mesh.scale.setScalar(config.size)
    const point = config.curve.getPointAt(config.t)
    mesh.position.copy(point)
    signalGroup.add(mesh)
    return { mesh, config, basePoint: point.clone() }
  })

  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  const flowUniforms = flowMaterials.map((material) => material.uniforms)
  const particleUniforms = particleMaterial.uniforms
  const shellUniforms = shellMaterial.uniforms
  const clock = new THREE.Clock()

  const updateTheme = (isDark: boolean) => {
    const nextPrimaryToken = readThemeToken('--color-primary', isDark ? '#6366f1' : '#4f46e5')
    const nextAccentToken = readThemeToken('--color-accent', isDark ? '#22d3ee' : '#0891b2')
    const nextPrimaryHex = colorHex(THREE, nextPrimaryToken, isDark ? '#6366f1' : '#4f46e5')
    const nextAccentHex = colorHex(THREE, nextAccentToken, isDark ? '#22d3ee' : '#0891b2')

    primaryLight.color.setHex(nextPrimaryHex)
    accentLight.color.setHex(nextAccentHex)
    coreMaterial.emissive.setHex(nextPrimaryHex)
    coreMaterial.emissiveIntensity = isDark ? 0.14 : 0.1
    emblemBodyMaterial.emissive.setHex(nextAccentHex)
    focalRingMaterial.color.setHex(nextAccentHex)
    focalRingMaterial.emissive.setHex(nextPrimaryHex)
    shadowRingMaterial.color.setHex(nextPrimaryHex)
    ambientMaterial.color.setHex(nextAccentHex)
    nodeMaterials.forEach((material, index) => material.color.setHex(index % 2 === 0 ? nextPrimaryHex : nextAccentHex))
    ringMaterials.forEach(({ material, index }) => {
      const colors = [nextAccentHex, nextPrimaryHex, 0xa5b4fc, nextAccentHex, nextPrimaryHex]
      material.color.setHex(colors[index % colors.length])
    })
    flowUniforms.forEach((uniforms, index) => {
      uniforms.uPrimary.value.set(nextPrimaryToken)
      uniforms.uAccent.value.set(nextAccentToken)
      uniforms.uOpacity.value = index === 0 ? 0.72 : 0.6
    })
    shellUniforms.uPrimary.value.set(nextPrimaryToken)
    shellUniforms.uAccent.value.set(nextAccentToken)
    particleUniforms.uPrimary.value.set(nextPrimaryToken)
    particleUniforms.uAccent.value.set(nextAccentToken)
    emblemTexture.redraw(isDark)
  }
  const stopThemeObserver = observeTheme(updateTheme)

  const resize = () => {
    const rect = host.getBoundingClientRect()
    const width = Math.max(rect.width, 1)
    const height = Math.max(rect.height, 1)
    camera.aspect = width / height
    camera.updateProjectionMatrix()
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, window.innerWidth < 768 ? 1.2 : 1.45))
    renderer.setSize(width, height, false)
  }
  const resizeObserver = new ResizeObserver(resize)
  resizeObserver.observe(host)
  resize()

  const handlePointerMove = (event: PointerEvent) => {
    if (event.pointerType === 'touch') return
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
  }, { rootMargin: '160px' })
  intersectionObserver.observe(stage)

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
    pointerX += (pointerTargetX - pointerX) * Math.min(1, delta * 4.4)
    pointerY += (pointerTargetY - pointerY) * Math.min(1, delta * 4.4)
    scrollValue += (scrollTarget - scrollValue) * Math.min(1, delta * 3.2)

    shellUniforms.uTime.value = elapsed
    shellUniforms.uPointer.value.set(pointerX, pointerY)
    flowUniforms.forEach((uniforms) => { uniforms.uTime.value = elapsed })
    particleUniforms.uTime.value = elapsed

    if (!reducedMotion) {
      coreGroup.rotation.x = pointerY * -0.08 + Math.sin(elapsed * 0.42) * 0.025
      coreGroup.rotation.y = pointerX * 0.12 + elapsed * 0.11
      coreGroup.position.y = Math.sin(elapsed * 0.82) * 0.055
      shell.rotation.x = elapsed * 0.035 + pointerY * 0.04
      shell.rotation.y = -elapsed * 0.048 + pointerX * 0.06
      emblemFace.rotation.z = elapsed * 0.018
      focalRing.rotation.z = -0.08 + elapsed * 0.09
      ribbonGroup.rotation.y = pointerX * 0.07 + elapsed * 0.028
      ribbonGroup.rotation.x = pointerY * -0.035
      particleGroup.rotation.y = elapsed * 0.012
      particleGroup.rotation.x = pointerY * -0.018
      ambientCloud.rotation.y = -elapsed * 0.009
      shadowRing.rotation.z = elapsed * 0.06
      ringMeshes.forEach(({ mesh, speed }) => {
        mesh.rotation.z += speed * delta
        mesh.rotation.y += speed * delta * 0.34
      })
      signals.forEach(({ mesh, config, basePoint }) => {
        const pulse = 1 + Math.sin(elapsed * 1.45 + config.phase) * 0.2
        mesh.scale.setScalar(config.size * pulse)
        mesh.position.set(
          basePoint.x,
          basePoint.y + Math.sin(elapsed * 1.1 + config.phase) * 0.018,
          basePoint.z,
        )
      })
    }

    world.rotation.x = pointerY * -0.075 + scrollValue * 0.025
    world.rotation.y = pointerX * 0.12 + scrollValue * 0.12
    world.position.y = 0.08 - scrollValue * 0.14
    camera.position.x = pointerX * 0.42
    camera.position.y = 0.22 - pointerY * 0.28 + scrollValue * 0.08
    camera.position.z = 8.6 + Math.abs(scrollValue) * 0.2
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
  width: min(100%, 760px);
  min-height: 632px;
  overflow: hidden;
  isolation: isolate;
  perspective: 1200px;
}

.scene-shell {
  position: absolute;
  inset: 0;
  overflow: hidden;
  color-scheme: dark;
  background: #080b13;
}

.scene-atmosphere {
  position: absolute;
  z-index: 1;
  inset: 0;
  overflow: hidden;
  background:
    radial-gradient(ellipse at 50% 44%, rgba(48, 54, 126, 0.22), transparent 38%),
    radial-gradient(ellipse at 82% 23%, rgba(34, 211, 238, 0.12), transparent 30%),
    radial-gradient(ellipse at 10% 84%, rgba(79, 70, 229, 0.13), transparent 36%),
    #080b13;
  pointer-events: none;
}

.scene-atmosphere::after {
  position: absolute;
  inset: 0;
  background: linear-gradient(112deg, transparent 20%, rgba(165, 180, 252, 0.05) 42%, transparent 66%);
  content: '';
  mix-blend-mode: screen;
  opacity: 0.72;
}

.scene-atmosphere-glow,
.scene-depth-vignette,
.scene-depth-sweep {
  position: absolute;
  display: block;
}

.scene-atmosphere-glow {
  border-radius: 50%;
  filter: blur(12px);
  opacity: 0.52;
  animation: scene-glow-drift 12s ease-in-out infinite alternate;
}

.scene-atmosphere-glow-one {
  top: 18%;
  left: 25%;
  width: 50%;
  height: 50%;
  background: radial-gradient(circle, rgba(99, 102, 241, 0.28), transparent 68%);
}

.scene-atmosphere-glow-two {
  right: 7%;
  bottom: 10%;
  width: 34%;
  height: 34%;
  background: radial-gradient(circle, rgba(34, 211, 238, 0.2), transparent 70%);
  animation-delay: -4s;
}

.scene-depth-vignette {
  z-index: 3;
  inset: 0;
  background: radial-gradient(ellipse at center, transparent 38%, rgba(1, 3, 8, 0.38) 100%);
}

.scene-depth-sweep {
  z-index: 2;
  width: 74%;
  height: 1px;
  opacity: 0.44;
  background: linear-gradient(90deg, transparent, rgba(103, 232, 249, 0.72), transparent);
  box-shadow: 0 0 26px rgba(34, 211, 238, 0.28);
  transform: rotate(-19deg);
  animation: scene-sweep 8s ease-in-out infinite;
}

.scene-depth-sweep-one { top: 28%; left: -14%; }
.scene-depth-sweep-two { right: -12%; bottom: 31%; animation-delay: -4s; transform: rotate(18deg); }

.scene-atmosphere-star {
  position: absolute;
  z-index: 3;
  display: block;
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: #67e8f9;
  box-shadow: 0 0 0 4px rgba(103, 232, 249, 0.08), 0 0 18px rgba(103, 232, 249, 0.56);
  animation: scene-star-pulse 4.4s ease-in-out infinite;
}

.scene-atmosphere-star-one { top: 21%; left: 18%; }
.scene-atmosphere-star-two { top: 34%; right: 15%; width: 3px; height: 3px; animation-delay: -1.5s; }
.scene-atmosphere-star-three { bottom: 22%; left: 28%; width: 3px; height: 3px; animation-delay: -2.4s; }
.scene-atmosphere-star-four { right: 26%; bottom: 17%; width: 2px; height: 2px; animation-delay: -3.1s; }

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
  color: #a5b4fc;
  pointer-events: none;
}

.scene-loading span {
  width: 38px;
  height: 38px;
  border: 1px solid rgba(165, 180, 252, 0.68);
  border-right-color: transparent;
  border-radius: 50%;
  box-shadow: 0 0 26px rgba(99, 102, 241, 0.28);
  animation: scene-loader 0.9s linear infinite;
}

.scene-loading small {
  color: rgba(226, 232, 240, 0.68);
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.18em;
}

.fallback-core {
  position: absolute;
  z-index: 4;
  top: 50%;
  left: 50%;
  width: 330px;
  height: 330px;
  transform: translate(-50%, -50%);
}

.fallback-aura,
.fallback-shadow,
.fallback-ring,
.fallback-emblem,
.fallback-signal {
  position: absolute;
  display: block;
}

.fallback-aura {
  inset: 12%;
  border: 1px solid rgba(103, 232, 249, 0.32);
  border-radius: 50%;
  background: radial-gradient(circle at 32% 24%, rgba(255, 255, 255, 0.15), transparent 20%), radial-gradient(circle, rgba(99, 102, 241, 0.26), transparent 68%);
  box-shadow: 0 0 0 24px rgba(99, 102, 241, 0.045), 0 0 72px rgba(99, 102, 241, 0.2);
}

.fallback-shadow {
  right: 10%;
  bottom: 16%;
  left: 10%;
  height: 11%;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.58);
  filter: blur(12px);
  transform: rotate(-4deg);
}

.fallback-ring {
  inset: 8%;
  border: 1px solid rgba(165, 180, 252, 0.74);
  border-radius: 50%;
  transform: rotateX(66deg) rotateZ(-12deg);
}

.fallback-ring-one { animation: fallback-spin 10s linear infinite; }
.fallback-ring-two { inset: 19% -2%; border-color: rgba(34, 211, 238, 0.62); transform: rotateY(66deg) rotateZ(28deg); animation: fallback-spin-two 14s linear infinite reverse; }

.fallback-emblem {
  top: 50%;
  left: 50%;
  display: grid;
  width: 130px;
  height: 130px;
  place-content: center;
  justify-items: center;
  border: 1px solid rgba(165, 180, 252, 0.72);
  border-radius: 50%;
  color: #f8fafc;
  background: linear-gradient(145deg, #1b2345, #071b2c);
  box-shadow: inset 0 2px 0 rgba(255, 255, 255, 0.22), 0 24px 46px rgba(0, 0, 0, 0.42), 0 0 42px rgba(99, 102, 241, 0.26);
  transform: translate(-50%, -50%);
}

.fallback-emblem span { font-size: 48px; font-weight: 800; line-height: 1; }
.fallback-emblem small { margin-top: 6px; color: rgba(226, 232, 240, 0.7); font: 700 8px ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.14em; }

.fallback-signal {
  width: 12px;
  height: 12px;
  border: 1px solid rgba(103, 232, 249, 0.84);
  border-radius: 50%;
  background: rgba(34, 211, 238, 0.34);
  box-shadow: 0 0 0 8px rgba(34, 211, 238, 0.06), 0 0 24px rgba(34, 211, 238, 0.5);
  animation: fallback-signal-pulse 3.8s ease-in-out infinite;
}

.fallback-signal-one { top: 28%; left: 11%; }
.fallback-signal-two { top: 25%; right: 11%; width: 9px; height: 9px; animation-delay: -1.4s; }
.fallback-signal-three { right: 18%; bottom: 25%; width: 15px; height: 15px; animation-delay: -2.2s; }

@keyframes scene-loader { to { transform: rotate(360deg); } }
@keyframes scene-glow-drift { from { transform: translate3d(-10px, 7px, 0) scale(0.94); } to { transform: translate3d(12px, -10px, 0) scale(1.08); } }
@keyframes scene-star-pulse { 0%, 100% { opacity: 0.24; transform: scale(0.8); } 50% { opacity: 0.9; transform: scale(1.26); } }
@keyframes scene-sweep { 0%, 100% { opacity: 0; transform: translate3d(-9%, 0, 0) rotate(-19deg); } 32%, 70% { opacity: 0.5; } 50% { transform: translate3d(25%, 100px, 0) rotate(-19deg); } }
@keyframes fallback-spin { to { transform: rotateX(66deg) rotateZ(348deg); } }
@keyframes fallback-spin-two { to { transform: rotateY(66deg) rotateZ(-332deg); } }
@keyframes fallback-signal-pulse { 0%, 100% { opacity: 0.4; transform: scale(0.8); } 50% { opacity: 1; transform: scale(1.2); } }

@media (max-width: 1023px) {
  .hero-orbit-stage { min-height: 560px; }
}

@media (max-width: 640px) {
  .hero-orbit-stage { min-height: 468px; }
  .fallback-core { width: 286px; height: 286px; }
  .fallback-emblem { width: 112px; height: 112px; }
  .fallback-emblem span { font-size: 40px; }
}

@media (prefers-reduced-motion: reduce) {
  .scene-atmosphere-glow,
  .scene-atmosphere-star,
  .scene-depth-sweep,
  .scene-loading span,
  .fallback-ring,
  .fallback-signal { animation: none !important; }
}
</style>
