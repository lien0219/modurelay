import '@/styles/home-three.css'
import { isDarkTheme, loadThree, observeTheme } from '@/utils/threeRuntime'

type Disposable = { dispose: () => void }
type StreamParticle = {
  mesh: { position: { copy: (point: unknown) => void } }
  curve: { getPointAt: (progress: number) => unknown }
  offset: number
  speed: number
}

const BACKGROUND_CANVAS_CLASS = 'home-three-background-canvas'

function prefersReducedMotion(): boolean {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

function supportsWebGL(): boolean {
  try {
    const canvas = document.createElement('canvas')
    return Boolean(
      canvas.getContext('webgl2')
      || canvas.getContext('webgl')
      || canvas.getContext('experimental-webgl')
    )
  } catch {
    return false
  }
}

export function mountHomeThreeBackground(): () => void {
  let disposed = false
  let mountFrame = 0
  let cleanupScene: (() => void) | null = null
  let attempts = 0

  const tryMount = () => {
    if (disposed) return

    const host = document.querySelector<HTMLElement>('.home-background')
    if (!host) {
      attempts += 1
      if (attempts < 12) mountFrame = window.requestAnimationFrame(tryMount)
      return
    }

    void initializeScene(host).then((cleanup) => {
      if (disposed) cleanup()
      else cleanupScene = cleanup
    })
  }

  mountFrame = window.requestAnimationFrame(tryMount)

  return () => {
    disposed = true
    window.cancelAnimationFrame(mountFrame)
    cleanupScene?.()
    cleanupScene = null
  }
}

async function initializeScene(host: HTMLElement): Promise<() => void> {
  const existing = host.querySelector<HTMLCanvasElement>(`.${BACKGROUND_CANVAS_CLASS}`)
  existing?.remove()

  if (!supportsWebGL()) {
    host.classList.add('home-three-runtime-fallback')
    return () => host.classList.remove('home-three-runtime-fallback')
  }

  let THREE
  try {
    THREE = await loadThree()
  } catch (error) {
    console.warn('[HomeThree] Three.js runtime unavailable, using CSS fallback.', error)
    host.classList.add('home-three-runtime-fallback')
    return () => host.classList.remove('home-three-runtime-fallback')
  }

  let stopped = false
  let animationFrame = 0
  let currentPointerX = 0
  let currentPointerY = 0
  let targetPointerX = 0
  let targetPointerY = 0
  let targetScroll = 0
  let currentScroll = 0
  let lastFrameTime = performance.now()

  const resources: Disposable[] = []
  const streamParticles: StreamParticle[] = []
  const canvas = document.createElement('canvas')
  canvas.className = BACKGROUND_CANVAS_CLASS
  canvas.setAttribute('aria-hidden', 'true')
  host.prepend(canvas)

  const renderer = new THREE.WebGLRenderer({
    canvas,
    alpha: true,
    antialias: window.devicePixelRatio <= 1.5,
    powerPreference: 'high-performance',
    premultipliedAlpha: true
  })
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 1.6))
  renderer.setSize(window.innerWidth, window.innerHeight, false)
  renderer.outputColorSpace = THREE.SRGBColorSpace
  renderer.setClearColor(0x000000, 0)

  const scene = new THREE.Scene()
  const camera = new THREE.PerspectiveCamera(48, window.innerWidth / window.innerHeight, 0.1, 80)
  camera.position.set(0, 0.6, 12)

  const world = new THREE.Group()
  scene.add(world)

  const pointCount = window.innerWidth < 768 ? 520 : 1050
  const pointPositions = new Float32Array(pointCount * 3)
  const pointSizes = new Float32Array(pointCount)
  for (let index = 0; index < pointCount; index += 1) {
    const radius = 4.5 + Math.random() * 14
    const theta = Math.random() * Math.PI * 2
    const phi = Math.acos(2 * Math.random() - 1)
    pointPositions[index * 3] = Math.sin(phi) * Math.cos(theta) * radius
    pointPositions[index * 3 + 1] = Math.cos(phi) * radius * 0.65
    pointPositions[index * 3 + 2] = Math.sin(phi) * Math.sin(theta) * radius - 7
    pointSizes[index] = 0.6 + Math.random() * 1.8
  }

  const pointGeometry = new THREE.BufferGeometry()
  pointGeometry.setAttribute('position', new THREE.BufferAttribute(pointPositions, 3))
  pointGeometry.setAttribute('aSize', new THREE.BufferAttribute(pointSizes, 1))
  resources.push(pointGeometry)

  const pointMaterial = new THREE.ShaderMaterial({
    transparent: true,
    depthWrite: false,
    blending: THREE.AdditiveBlending,
    uniforms: {
      uColor: { value: new THREE.Color(isDarkTheme() ? 0x4fd1c5 : 0x0f766e) },
      uPixelRatio: { value: Math.min(window.devicePixelRatio, 1.6) },
      uOpacity: { value: isDarkTheme() ? 0.72 : 0.34 }
    },
    vertexShader: `
      attribute float aSize;
      uniform float uPixelRatio;
      varying float vFade;
      void main() {
        vec4 modelPosition = modelViewMatrix * vec4(position, 1.0);
        gl_Position = projectionMatrix * modelPosition;
        gl_PointSize = aSize * uPixelRatio * (42.0 / max(1.0, -modelPosition.z));
        vFade = smoothstep(-24.0, 4.0, position.z);
      }
    `,
    fragmentShader: `
      uniform vec3 uColor;
      uniform float uOpacity;
      varying float vFade;
      void main() {
        float distanceToCenter = distance(gl_PointCoord, vec2(0.5));
        float strength = smoothstep(0.5, 0.02, distanceToCenter);
        gl_FragColor = vec4(uColor, strength * uOpacity * vFade);
      }
    `
  })
  resources.push(pointMaterial)

  const pointCloud = new THREE.Points(pointGeometry, pointMaterial)
  pointCloud.rotation.x = -0.08
  world.add(pointCloud)

  const gridColumns = window.innerWidth < 768 ? 28 : 46
  const gridRows = window.innerWidth < 768 ? 18 : 30
  const gridPositions = new Float32Array(gridColumns * gridRows * 3)
  let gridOffset = 0
  for (let row = 0; row < gridRows; row += 1) {
    for (let column = 0; column < gridColumns; column += 1) {
      const x = (column / (gridColumns - 1) - 0.5) * 24
      const z = (row / (gridRows - 1) - 0.5) * 18
      const y = Math.sin(x * 0.55) * 0.14 + Math.cos(z * 0.7) * 0.12
      gridPositions[gridOffset] = x
      gridPositions[gridOffset + 1] = y - 4.1
      gridPositions[gridOffset + 2] = z - 5
      gridOffset += 3
    }
  }

  const gridGeometry = new THREE.BufferGeometry()
  gridGeometry.setAttribute('position', new THREE.BufferAttribute(gridPositions, 3))
  resources.push(gridGeometry)
  const gridMaterial = new THREE.PointsMaterial({
    color: isDarkTheme() ? 0x22d3ee : 0x0d9488,
    size: window.innerWidth < 768 ? 0.018 : 0.026,
    transparent: true,
    opacity: isDarkTheme() ? 0.34 : 0.17,
    depthWrite: false,
    blending: THREE.AdditiveBlending
  })
  resources.push(gridMaterial)
  const waveGrid = new THREE.Points(gridGeometry, gridMaterial)
  waveGrid.rotation.x = -0.34
  world.add(waveGrid)

  const ringMaterials: Array<{ color: { setHex: (value: number) => void }; opacity: number }> = []
  const ringGroup = new THREE.Group()
  for (let index = 0; index < 5; index += 1) {
    const geometry = new THREE.TorusGeometry(2.7 + index * 0.86, 0.008 + index * 0.002, 6, 160)
    const material = new THREE.MeshBasicMaterial({
      color: index % 2 === 0 ? 0x14b8a6 : 0x38bdf8,
      transparent: true,
      opacity: isDarkTheme() ? 0.16 - index * 0.015 : 0.08 - index * 0.007,
      depthWrite: false,
      blending: THREE.AdditiveBlending
    })
    resources.push(geometry, material)
    ringMaterials.push(material)
    const ring = new THREE.Mesh(geometry, material)
    ring.rotation.x = Math.PI / 2.6 + index * 0.08
    ring.rotation.y = index * 0.21
    ringGroup.add(ring)
  }
  ringGroup.position.set(4.8, 1.1, -5.5)
  world.add(ringGroup)

  const streamCurves = [
    new THREE.CatmullRomCurve3([
      new THREE.Vector3(-12, 3.4, -7),
      new THREE.Vector3(-5, 4.2, -5),
      new THREE.Vector3(1, 1.4, -3),
      new THREE.Vector3(10, 2.8, -8)
    ]),
    new THREE.CatmullRomCurve3([
      new THREE.Vector3(-10, -2.7, -5),
      new THREE.Vector3(-2, -1.3, -3),
      new THREE.Vector3(5, -2.4, -6),
      new THREE.Vector3(12, 0.4, -8)
    ]),
    new THREE.CatmullRomCurve3([
      new THREE.Vector3(-8, 5.8, -10),
      new THREE.Vector3(-1, 2.6, -4),
      new THREE.Vector3(7, 4.5, -7)
    ])
  ]

  streamCurves.forEach((curve, curveIndex) => {
    const curvePoints = curve.getPoints(100)
    const geometry = new THREE.BufferGeometry().setFromPoints(curvePoints)
    const material = new THREE.LineBasicMaterial({
      color: curveIndex === 1 ? 0x8b5cf6 : curveIndex === 2 ? 0x38bdf8 : 0x14b8a6,
      transparent: true,
      opacity: isDarkTheme() ? 0.24 : 0.12,
      depthWrite: false,
      blending: THREE.AdditiveBlending
    })
    resources.push(geometry, material)
    const line = new THREE.Line(geometry, material)
    world.add(line)

    const beadGeometry = new THREE.SphereGeometry(0.055, 10, 10)
    const beadMaterial = new THREE.MeshBasicMaterial({
      color: curveIndex === 1 ? 0xc084fc : curveIndex === 2 ? 0x67e8f9 : 0x5eead4,
      transparent: true,
      opacity: 0.95,
      blending: THREE.AdditiveBlending
    })
    resources.push(beadGeometry, beadMaterial)

    for (let beadIndex = 0; beadIndex < 4; beadIndex += 1) {
      const bead = new THREE.Mesh(beadGeometry, beadMaterial)
      world.add(bead)
      streamParticles.push({
        mesh: bead,
        curve,
        offset: beadIndex / 4 + curveIndex * 0.17,
        speed: 0.035 + curveIndex * 0.008
      })
    }
  })

  const updateTheme = (dark: boolean) => {
    pointMaterial.uniforms.uColor.value.setHex(dark ? 0x67e8f9 : 0x0f766e)
    pointMaterial.uniforms.uOpacity.value = dark ? 0.72 : 0.34
    gridMaterial.color.setHex(dark ? 0x2dd4bf : 0x0d9488)
    gridMaterial.opacity = dark ? 0.34 : 0.17
    ringMaterials.forEach((material, index) => {
      material.color.setHex(index % 2 === 0 ? (dark ? 0x2dd4bf : 0x0f766e) : (dark ? 0x60a5fa : 0x0284c7))
      material.opacity = dark ? 0.16 - index * 0.015 : 0.08 - index * 0.007
    })
  }
  const stopThemeObserver = observeTheme(updateTheme)

  const handleResize = () => {
    const width = window.innerWidth
    const height = window.innerHeight
    camera.aspect = width / Math.max(height, 1)
    camera.updateProjectionMatrix()
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 1.6))
    renderer.setSize(width, height, false)
    pointMaterial.uniforms.uPixelRatio.value = Math.min(window.devicePixelRatio, 1.6)
  }

  const handlePointerMove = (event: PointerEvent) => {
    targetPointerX = event.clientX / Math.max(window.innerWidth, 1) - 0.5
    targetPointerY = event.clientY / Math.max(window.innerHeight, 1) - 0.5
  }

  const handleScroll = () => {
    const maxScroll = Math.max(document.documentElement.scrollHeight - window.innerHeight, 1)
    targetScroll = Math.min(1, Math.max(0, window.scrollY / maxScroll))
  }

  const handleVisibility = () => {
    lastFrameTime = performance.now()
  }

  window.addEventListener('resize', handleResize, { passive: true })
  window.addEventListener('pointermove', handlePointerMove, { passive: true })
  window.addEventListener('scroll', handleScroll, { passive: true })
  document.addEventListener('visibilitychange', handleVisibility)
  handleScroll()

  const reducedMotion = prefersReducedMotion()
  const clock = new THREE.Clock()

  const render = (time: number) => {
    if (stopped) return
    animationFrame = window.requestAnimationFrame(render)
    if (document.hidden) return

    const delta = Math.min((time - lastFrameTime) / 1000, 0.05)
    lastFrameTime = time
    const elapsed = clock.getElapsedTime()

    currentPointerX += (targetPointerX - currentPointerX) * Math.min(1, delta * 2.8)
    currentPointerY += (targetPointerY - currentPointerY) * Math.min(1, delta * 2.8)
    currentScroll += (targetScroll - currentScroll) * Math.min(1, delta * 2.4)

    if (!reducedMotion) {
      pointCloud.rotation.y = elapsed * 0.018 + currentPointerX * 0.16
      pointCloud.rotation.x = -0.08 + currentPointerY * 0.08
      waveGrid.position.x = Math.sin(elapsed * 0.18) * 0.34
      waveGrid.rotation.z = Math.sin(elapsed * 0.12) * 0.025
      ringGroup.rotation.z = elapsed * 0.07
      ringGroup.rotation.y = currentPointerX * 0.22 + currentScroll * 0.34

      streamParticles.forEach((particle) => {
        const progress = (elapsed * particle.speed + particle.offset) % 1
        particle.mesh.position.copy(particle.curve.getPointAt(progress))
      })
    }

    world.rotation.y = currentPointerX * 0.08 + currentScroll * 0.16
    world.position.y = currentScroll * 1.6 - currentPointerY * 0.32
    camera.position.x = currentPointerX * 0.58
    camera.position.y = 0.6 - currentPointerY * 0.42
    camera.lookAt(0, 0, -4)
    renderer.render(scene, camera)
  }

  if (reducedMotion) {
    renderer.render(scene, camera)
  } else {
    animationFrame = window.requestAnimationFrame(render)
  }

  return () => {
    stopped = true
    window.cancelAnimationFrame(animationFrame)
    window.removeEventListener('resize', handleResize)
    window.removeEventListener('pointermove', handlePointerMove)
    window.removeEventListener('scroll', handleScroll)
    document.removeEventListener('visibilitychange', handleVisibility)
    stopThemeObserver()
    resources.forEach((resource) => resource.dispose())
    renderer.dispose()
    canvas.remove()
    host.classList.remove('home-three-runtime-fallback')
  }
}
