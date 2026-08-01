import '@/styles/home-advertising-three.css'
import { isDarkTheme, loadThree, observeTheme } from '@/utils/threeRuntime'

type Disposable = { dispose: () => void }
type FlowParticle = {
  mesh: {
    position: { copy: (point: unknown) => void }
    scale: { setScalar: (value: number) => void }
  }
  curve: { getPointAt: (progress: number) => unknown }
  offset: number
  speed: number
}
type OrbitItem = {
  mesh: {
    position: { set: (x: number, y: number, z: number) => void }
    rotation: { x: number; y: number; z: number }
    scale: { setScalar: (value: number) => void }
  }
  radius: number
  height: number
  speed: number
  phase: number
}

const HOST_CLASS = 'home-three-ad-host'

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

function createOverlay(host: HTMLElement): void {
  const headline = document.createElement('div')
  headline.className = 'home-three-ad-headline'
  headline.innerHTML = '<span>PREMIUM MEDIA SLOT</span><strong>LIVE BRAND MATRIX</strong>'

  const status = document.createElement('div')
  status.className = 'home-three-ad-status'
  status.innerHTML = '<i></i><span>AD NETWORK ONLINE</span><strong>99.99%</strong>'

  const hint = document.createElement('div')
  hint.className = 'home-three-ad-hint'
  hint.innerHTML = '<span></span>MOVE TO INTERACT'

  host.append(headline, status, hint)
}

function drawRoundedRect(
  context: CanvasRenderingContext2D,
  x: number,
  y: number,
  width: number,
  height: number,
  radius: number
): void {
  const safeRadius = Math.min(radius, width / 2, height / 2)
  context.beginPath()
  context.moveTo(x + safeRadius, y)
  context.lineTo(x + width - safeRadius, y)
  context.quadraticCurveTo(x + width, y, x + width, y + safeRadius)
  context.lineTo(x + width, y + height - safeRadius)
  context.quadraticCurveTo(x + width, y + height, x + width - safeRadius, y + height)
  context.lineTo(x + safeRadius, y + height)
  context.quadraticCurveTo(x, y + height, x, y + height - safeRadius)
  context.lineTo(x, y + safeRadius)
  context.quadraticCurveTo(x, y, x + safeRadius, y)
  context.closePath()
}

function createAdTexture(THREE: any, renderer: any, dark: boolean) {
  const canvas = document.createElement('canvas')
  canvas.width = 1024
  canvas.height = 640
  const context = canvas.getContext('2d')
  if (!context) throw new Error('Unable to create advertising texture')

  const texture = new THREE.CanvasTexture(canvas)
  texture.colorSpace = THREE.SRGBColorSpace
  texture.anisotropy = Math.min(renderer.capabilities.getMaxAnisotropy(), 8)

  const redraw = (isDark: boolean) => {
    context.clearRect(0, 0, canvas.width, canvas.height)

    const background = context.createLinearGradient(0, 0, canvas.width, canvas.height)
    background.addColorStop(0, isDark ? '#062f37' : '#0f766e')
    background.addColorStop(0.48, isDark ? '#075985' : '#0284c7')
    background.addColorStop(1, isDark ? '#312e81' : '#4f46e5')
    drawRoundedRect(context, 18, 18, 988, 604, 58)
    context.fillStyle = background
    context.fill()

    const glow = context.createRadialGradient(720, 190, 30, 720, 190, 430)
    glow.addColorStop(0, isDark ? 'rgba(103,232,249,0.34)' : 'rgba(255,255,255,0.26)')
    glow.addColorStop(1, 'rgba(255,255,255,0)')
    context.fillStyle = glow
    context.fillRect(18, 18, 988, 604)

    context.save()
    drawRoundedRect(context, 18, 18, 988, 604, 58)
    context.clip()
    context.strokeStyle = isDark ? 'rgba(153,246,228,0.14)' : 'rgba(255,255,255,0.14)'
    context.lineWidth = 2
    for (let x = 40; x < 1024; x += 64) {
      context.beginPath()
      context.moveTo(x, 0)
      context.lineTo(x, 640)
      context.stroke()
    }
    for (let y = 32; y < 640; y += 64) {
      context.beginPath()
      context.moveTo(0, y)
      context.lineTo(1024, y)
      context.stroke()
    }
    context.restore()

    context.strokeStyle = isDark ? 'rgba(153,246,228,0.82)' : 'rgba(255,255,255,0.72)'
    context.lineWidth = 10
    drawRoundedRect(context, 28, 28, 968, 584, 52)
    context.stroke()

    context.textAlign = 'left'
    context.textBaseline = 'alphabetic'
    context.fillStyle = 'rgba(255,255,255,0.7)'
    context.font = '700 28px Inter, system-ui, sans-serif'
    context.fillText('MODURELAY MEDIA', 80, 104)

    context.textAlign = 'center'
    context.textBaseline = 'middle'
    context.fillStyle = '#ffffff'
    context.shadowColor = isDark ? 'rgba(103,232,249,0.78)' : 'rgba(255,255,255,0.46)'
    context.shadowBlur = 42
    context.font = '900 258px Inter, system-ui, sans-serif'
    context.fillText('AD', 512, 334)

    context.shadowBlur = 0
    context.fillStyle = 'rgba(255,255,255,0.82)'
    context.font = '750 31px Inter, system-ui, sans-serif'
    context.letterSpacing = '8px'
    context.fillText('PREMIUM PLACEMENT', 512, 520)

    context.fillStyle = 'rgba(255,255,255,0.58)'
    context.font = '600 20px Inter, system-ui, sans-serif'
    context.letterSpacing = '4px'
    context.fillText('BRAND  ·  TOOLS  ·  DIGITAL SERVICES', 512, 568)

    texture.needsUpdate = true
  }

  redraw(dark)
  return { texture, redraw }
}

export function mountHomeAdvertisingThree(): () => void {
  let disposed = false
  let mountFrame = 0
  let cleanupScene: (() => void) | null = null
  let attempts = 0

  const tryMount = () => {
    if (disposed) return

    const panel = document.querySelector<HTMLElement>('.advertising-panel')
    const visual = panel?.querySelector<HTMLElement>('.ad-visual')
    if (!panel || !visual) {
      attempts += 1
      if (attempts < 16) mountFrame = window.requestAnimationFrame(tryMount)
      return
    }

    void initializeScene(panel, visual).then((cleanup) => {
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

async function initializeScene(panel: HTMLElement, visual: HTMLElement): Promise<() => void> {
  visual.querySelector<HTMLElement>(`.${HOST_CLASS}`)?.remove()
  panel.classList.remove('three-ad-ready')

  if (!supportsWebGL()) return () => undefined

  let THREE
  try {
    THREE = await loadThree()
  } catch (error) {
    console.warn('[HomeAdvertisingThree] Three.js runtime unavailable, keeping CSS fallback.', error)
    return () => undefined
  }

  let stopped = false
  let animationFrame = 0
  let visible = true
  let pointerTargetX = 0
  let pointerTargetY = 0
  let pointerX = 0
  let pointerY = 0
  let scrollTarget = 0
  let scrollValue = 0
  let pulseTarget = 0
  let pulseValue = 0
  let lastFrameTime = performance.now()

  const resources: Disposable[] = []
  const flowParticles: FlowParticle[] = []
  const orbitItems: OrbitItem[] = []
  const ringMaterials: Array<{ color: { setHex: (value: number) => void }; opacity: number }> = []
  const lineMaterials: Array<{ color: { setHex: (value: number) => void }; opacity: number }> = []

  const host = document.createElement('div')
  host.className = HOST_CLASS
  host.setAttribute('aria-hidden', 'true')
  createOverlay(host)
  visual.appendChild(host)

  const canvas = document.createElement('canvas')
  canvas.className = 'home-three-ad-canvas'
  canvas.setAttribute('aria-hidden', 'true')
  host.prepend(canvas)

  const renderer = new THREE.WebGLRenderer({
    canvas,
    alpha: true,
    antialias: window.devicePixelRatio <= 1.75,
    powerPreference: 'high-performance',
    premultipliedAlpha: true
  })
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 1.7))
  renderer.outputColorSpace = THREE.SRGBColorSpace
  renderer.toneMapping = THREE.ACESFilmicToneMapping
  renderer.toneMappingExposure = isDarkTheme() ? 1.24 : 1.08
  renderer.setClearColor(0x000000, 0)

  const scene = new THREE.Scene()
  const camera = new THREE.PerspectiveCamera(38, 1, 0.1, 80)
  camera.position.set(0, 0.35, 11.4)

  const world = new THREE.Group()
  const plateGroup = new THREE.Group()
  const ringGroup = new THREE.Group()
  const nodeGroup = new THREE.Group()
  const flowGroup = new THREE.Group()
  scene.add(world)
  world.add(plateGroup, ringGroup, nodeGroup, flowGroup)

  const ambientLight = new THREE.HemisphereLight(0xd8fffb, 0x071426, isDarkTheme() ? 1.5 : 1.8)
  const keyLight = new THREE.DirectionalLight(0xffffff, isDarkTheme() ? 3.4 : 3.8)
  keyLight.position.set(4.5, 6.5, 7)
  const tealLight = new THREE.PointLight(0x2dd4bf, 42, 20, 2)
  tealLight.position.set(-4, 2.4, 4)
  const blueLight = new THREE.PointLight(0x38bdf8, 38, 20, 2)
  blueLight.position.set(4.8, -1.4, 3.6)
  const violetLight = new THREE.PointLight(0xa78bfa, 24, 16, 2)
  violetLight.position.set(0, 4.5, 0)
  scene.add(ambientLight, keyLight, tealLight, blueLight, violetLight)

  const adTexture = createAdTexture(THREE, renderer, isDarkTheme())
  resources.push(adTexture.texture)

  const sideMaterial = new THREE.MeshPhysicalMaterial({
    color: isDarkTheme() ? 0x062b38 : 0x0e7490,
    emissive: isDarkTheme() ? 0x063f47 : 0x064e5b,
    emissiveIntensity: isDarkTheme() ? 0.72 : 0.28,
    metalness: 0.82,
    roughness: 0.18,
    clearcoat: 1,
    clearcoatRoughness: 0.1
  })
  const faceMaterial = new THREE.MeshPhysicalMaterial({
    map: adTexture.texture,
    emissiveMap: adTexture.texture,
    emissive: isDarkTheme() ? 0x073b49 : 0x063a45,
    emissiveIntensity: isDarkTheme() ? 0.56 : 0.24,
    metalness: 0.38,
    roughness: 0.22,
    clearcoat: 1,
    clearcoatRoughness: 0.08
  })
  resources.push(sideMaterial, faceMaterial)

  const plateGeometry = new THREE.BoxGeometry(5.35, 3.25, 0.48, 4, 4, 2)
  resources.push(plateGeometry)
  const plate = new THREE.Mesh(plateGeometry, [
    sideMaterial,
    sideMaterial,
    sideMaterial,
    sideMaterial,
    faceMaterial,
    faceMaterial
  ])
  plate.rotation.set(-0.24, -0.48, 0.045)
  plateGroup.add(plate)

  const edgeGeometry = new THREE.EdgesGeometry(plateGeometry, 18)
  const edgeMaterial = new THREE.LineBasicMaterial({
    color: isDarkTheme() ? 0x99f6e4 : 0xffffff,
    transparent: true,
    opacity: isDarkTheme() ? 0.92 : 0.66,
    blending: THREE.AdditiveBlending
  })
  resources.push(edgeGeometry, edgeMaterial)
  const plateEdges = new THREE.LineSegments(edgeGeometry, edgeMaterial)
  plateEdges.scale.setScalar(1.008)
  plateEdges.rotation.copy(plate.rotation)
  plateGroup.add(plateEdges)

  const frameGeometry = new THREE.BoxGeometry(6.05, 3.95, 0.16, 1, 1, 1)
  const frameMaterial = new THREE.MeshBasicMaterial({
    color: isDarkTheme() ? 0x22d3ee : 0x0891b2,
    wireframe: true,
    transparent: true,
    opacity: isDarkTheme() ? 0.24 : 0.12,
    blending: THREE.AdditiveBlending
  })
  resources.push(frameGeometry, frameMaterial)
  const hologramFrame = new THREE.Mesh(frameGeometry, frameMaterial)
  hologramFrame.rotation.set(-0.17, -0.35, -0.02)
  plateGroup.add(hologramFrame)

  const platformGeometry = new THREE.CylinderGeometry(3.25, 4.35, 0.18, 96, 1, true)
  const platformMaterial = new THREE.MeshPhysicalMaterial({
    color: isDarkTheme() ? 0x082f49 : 0xcffafe,
    emissive: isDarkTheme() ? 0x0f766e : 0x0d9488,
    emissiveIntensity: isDarkTheme() ? 0.72 : 0.16,
    metalness: 0.7,
    roughness: 0.24,
    transparent: true,
    opacity: isDarkTheme() ? 0.64 : 0.42,
    side: THREE.DoubleSide
  })
  resources.push(platformGeometry, platformMaterial)
  const platform = new THREE.Mesh(platformGeometry, platformMaterial)
  platform.position.set(0, -2.58, -0.15)
  platform.rotation.z = 0.02
  world.add(platform)

  for (let index = 0; index < 4; index += 1) {
    const geometry = new THREE.TorusGeometry(3.05 + index * 0.52, 0.018 + index * 0.003, 8, 180)
    const material = new THREE.MeshBasicMaterial({
      color: index % 3 === 0 ? 0x2dd4bf : index % 3 === 1 ? 0x38bdf8 : 0xa78bfa,
      transparent: true,
      opacity: isDarkTheme() ? 0.43 - index * 0.065 : 0.23 - index * 0.035,
      depthWrite: false,
      blending: THREE.AdditiveBlending
    })
    resources.push(geometry, material)
    ringMaterials.push(material)
    const ring = new THREE.Mesh(geometry, material)
    ring.rotation.x = Math.PI / 2.35 + index * 0.12
    ring.rotation.y = index * 0.27
    ring.rotation.z = index * 0.46
    ring.userData.speed = (index % 2 === 0 ? 1 : -1) * (0.11 + index * 0.025)
    ringGroup.add(ring)
  }

  const nodeGeometry = new THREE.IcosahedronGeometry(0.15, 1)
  const nodeColors = [0x2dd4bf, 0x38bdf8, 0xa78bfa, 0xfbbf24, 0x67e8f9, 0x34d399]
  const nodeMaterials = nodeColors.map((color) => new THREE.MeshPhysicalMaterial({
    color,
    emissive: color,
    emissiveIntensity: isDarkTheme() ? 1.1 : 0.5,
    metalness: 0.35,
    roughness: 0.25,
    transparent: true,
    opacity: 0.94
  }))
  resources.push(nodeGeometry, ...nodeMaterials)

  nodeMaterials.forEach((material, index) => {
    const mesh = new THREE.Mesh(nodeGeometry, material)
    nodeGroup.add(mesh)
    orbitItems.push({
      mesh,
      radius: 3.45 + (index % 3) * 0.58,
      height: -1.3 + (index % 4) * 0.85,
      speed: (0.17 + index * 0.025) * (index % 2 === 0 ? 1 : -1),
      phase: (index / nodeMaterials.length) * Math.PI * 2
    })
  })

  const endpoints = [
    new THREE.Vector3(-4.8, 2.45, -0.2),
    new THREE.Vector3(4.9, 2.35, -0.35),
    new THREE.Vector3(-5.15, -1.85, -0.4),
    new THREE.Vector3(5.1, -1.75, -0.55)
  ]

  endpoints.forEach((end: any, index: number) => {
    const curve = new THREE.CatmullRomCurve3([
      new THREE.Vector3(0, 0, 0.15),
      new THREE.Vector3(end.x * 0.42, end.y * 0.28 + (index % 2 === 0 ? 0.55 : -0.35), 1.2),
      end
    ])

    const tubeGeometry = new THREE.TubeGeometry(curve, 80, 0.016, 6, false)
    const tubeMaterial = new THREE.MeshBasicMaterial({
      color: index === 2 ? 0xa78bfa : index === 3 ? 0xf59e0b : index % 2 === 0 ? 0x2dd4bf : 0x38bdf8,
      transparent: true,
      opacity: isDarkTheme() ? 0.38 : 0.2,
      depthWrite: false,
      blending: THREE.AdditiveBlending
    })
    resources.push(tubeGeometry, tubeMaterial)
    lineMaterials.push(tubeMaterial)
    flowGroup.add(new THREE.Mesh(tubeGeometry, tubeMaterial))

    const beadGeometry = new THREE.SphereGeometry(0.075, 12, 12)
    const beadMaterial = new THREE.MeshBasicMaterial({
      color: index === 2 ? 0xc4b5fd : index === 3 ? 0xfcd34d : index % 2 === 0 ? 0x99f6e4 : 0x7dd3fc,
      transparent: true,
      opacity: 1,
      blending: THREE.AdditiveBlending
    })
    resources.push(beadGeometry, beadMaterial)

    for (let beadIndex = 0; beadIndex < 3; beadIndex += 1) {
      const bead = new THREE.Mesh(beadGeometry, beadMaterial)
      flowGroup.add(bead)
      flowParticles.push({
        mesh: bead,
        curve,
        offset: beadIndex / 3 + index * 0.17,
        speed: 0.095 + index * 0.012
      })
    }
  })

  const pointCount = window.innerWidth < 768 ? 42 : 78
  const pointPositions = new Float32Array(pointCount * 3)
  for (let index = 0; index < pointCount; index += 1) {
    const radius = 3.6 + Math.random() * 3.4
    const theta = Math.random() * Math.PI * 2
    pointPositions[index * 3] = Math.cos(theta) * radius
    pointPositions[index * 3 + 1] = (Math.random() - 0.5) * 5.6
    pointPositions[index * 3 + 2] = Math.sin(theta) * radius * 0.45 - 0.8
  }
  const pointGeometry = new THREE.BufferGeometry()
  pointGeometry.setAttribute('position', new THREE.BufferAttribute(pointPositions, 3))
  const pointMaterial = new THREE.PointsMaterial({
    color: isDarkTheme() ? 0x67e8f9 : 0x0d9488,
    size: window.innerWidth < 768 ? 0.035 : 0.045,
    transparent: true,
    opacity: isDarkTheme() ? 0.68 : 0.32,
    depthWrite: false,
    blending: THREE.AdditiveBlending
  })
  resources.push(pointGeometry, pointMaterial)
  const pointField = new THREE.Points(pointGeometry, pointMaterial)
  world.add(pointField)

  const updateTheme = (dark: boolean) => {
    renderer.toneMappingExposure = dark ? 1.24 : 1.08
    ambientLight.intensity = dark ? 1.5 : 1.8
    keyLight.intensity = dark ? 3.4 : 3.8
    adTexture.redraw(dark)
    sideMaterial.color.setHex(dark ? 0x062b38 : 0x0e7490)
    sideMaterial.emissive.setHex(dark ? 0x063f47 : 0x064e5b)
    sideMaterial.emissiveIntensity = dark ? 0.72 : 0.28
    faceMaterial.emissive.setHex(dark ? 0x073b49 : 0x063a45)
    faceMaterial.emissiveIntensity = dark ? 0.56 : 0.24
    edgeMaterial.color.setHex(dark ? 0x99f6e4 : 0xffffff)
    edgeMaterial.opacity = dark ? 0.92 : 0.66
    frameMaterial.color.setHex(dark ? 0x22d3ee : 0x0891b2)
    frameMaterial.opacity = dark ? 0.24 : 0.12
    platformMaterial.color.setHex(dark ? 0x082f49 : 0xcffafe)
    platformMaterial.emissive.setHex(dark ? 0x0f766e : 0x0d9488)
    platformMaterial.emissiveIntensity = dark ? 0.72 : 0.16
    platformMaterial.opacity = dark ? 0.64 : 0.42
    pointMaterial.color.setHex(dark ? 0x67e8f9 : 0x0d9488)
    pointMaterial.opacity = dark ? 0.68 : 0.32
    nodeMaterials.forEach((material) => {
      material.emissiveIntensity = dark ? 1.1 : 0.5
    })
    ringMaterials.forEach((material, index) => {
      const darkColors = [0x2dd4bf, 0x38bdf8, 0xa78bfa]
      const lightColors = [0x0f766e, 0x0284c7, 0x7c3aed]
      material.color.setHex((dark ? darkColors : lightColors)[index % 3])
      material.opacity = dark ? 0.43 - index * 0.065 : 0.23 - index * 0.035
    })
    lineMaterials.forEach((material) => {
      material.opacity = dark ? 0.38 : 0.2
    })
  }
  const stopThemeObserver = observeTheme(updateTheme)

  const resize = () => {
    const rect = host.getBoundingClientRect()
    const width = Math.max(rect.width, 1)
    const height = Math.max(rect.height, 1)
    camera.aspect = width / height
    camera.updateProjectionMatrix()
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 1.7))
    renderer.setSize(width, height, false)
  }
  const resizeObserver = new ResizeObserver(resize)
  resizeObserver.observe(host)
  resize()

  const handlePointerMove = (event: PointerEvent) => {
    const rect = visual.getBoundingClientRect()
    pointerTargetX = (event.clientX - rect.left) / Math.max(rect.width, 1) - 0.5
    pointerTargetY = (event.clientY - rect.top) / Math.max(rect.height, 1) - 0.5
  }

  const handlePointerLeave = () => {
    pointerTargetX = 0
    pointerTargetY = 0
  }

  const handlePointerDown = () => {
    pulseTarget = 1
  }

  const handleScroll = () => {
    const rect = panel.getBoundingClientRect()
    const centerDistance = rect.top + rect.height / 2 - window.innerHeight / 2
    scrollTarget = Math.max(-1, Math.min(1, centerDistance / Math.max(window.innerHeight, 1)))
  }

  visual.addEventListener('pointermove', handlePointerMove, { passive: true })
  visual.addEventListener('pointerleave', handlePointerLeave)
  visual.addEventListener('pointerdown', handlePointerDown)
  window.addEventListener('scroll', handleScroll, { passive: true })
  handleScroll()

  const intersectionObserver = new IntersectionObserver((entries) => {
    visible = entries.some((entry) => entry.isIntersecting)
    lastFrameTime = performance.now()
  }, { rootMargin: '180px' })
  intersectionObserver.observe(panel)

  panel.classList.add('three-ad-ready')

  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  const clock = new THREE.Clock()

  const animate = (time: number) => {
    if (stopped) return
    animationFrame = window.requestAnimationFrame(animate)

    if (!visible || document.hidden) {
      lastFrameTime = time
      return
    }

    const delta = Math.min((time - lastFrameTime) / 1000, 0.05)
    lastFrameTime = time
    const elapsed = clock.getElapsedTime()

    pointerX += (pointerTargetX - pointerX) * Math.min(1, delta * 4.5)
    pointerY += (pointerTargetY - pointerY) * Math.min(1, delta * 4.5)
    scrollValue += (scrollTarget - scrollValue) * Math.min(1, delta * 3.2)
    pulseValue += (pulseTarget - pulseValue) * Math.min(1, delta * 9)
    pulseTarget *= Math.max(0, 1 - delta * 4.2)

    if (!reducedMotion) {
      plate.rotation.x = -0.24 + Math.sin(elapsed * 0.72) * 0.035 + pointerY * 0.12
      plate.rotation.y = -0.48 + Math.sin(elapsed * 0.42) * 0.08 + pointerX * 0.34
      plate.rotation.z = 0.045 + Math.sin(elapsed * 0.5) * 0.025
      plate.position.y = Math.sin(elapsed * 1.05) * 0.12
      plate.scale.setScalar(1 + pulseValue * 0.035)

      plateEdges.rotation.copy(plate.rotation)
      plateEdges.position.copy(plate.position)
      plateEdges.scale.setScalar(1.008 + pulseValue * 0.04)

      hologramFrame.rotation.x = -0.17 - pointerY * 0.08
      hologramFrame.rotation.y = -0.35 - elapsed * 0.055 - pointerX * 0.14
      hologramFrame.rotation.z = -0.02 + Math.sin(elapsed * 0.7) * 0.04
      hologramFrame.scale.setScalar(1 + pulseValue * 0.07)

      ringGroup.children.forEach((child: any) => {
        if (typeof child.userData.speed === 'number') child.rotation.z += child.userData.speed * delta
      })

      orbitItems.forEach((item, index) => {
        const angle = item.phase + elapsed * item.speed
        item.mesh.position.set(
          Math.cos(angle) * item.radius,
          item.height + Math.sin(elapsed * 0.8 + index) * 0.2,
          Math.sin(angle) * item.radius * 0.42
        )
        item.mesh.rotation.x = angle * 0.8
        item.mesh.rotation.y = angle
        item.mesh.rotation.z = angle * 0.45
        item.mesh.scale.setScalar(0.8 + Math.sin(elapsed * 1.8 + index) * 0.18 + pulseValue * 0.35)
      })

      flowParticles.forEach((particle, index) => {
        const progress = (elapsed * particle.speed + particle.offset) % 1
        particle.mesh.position.copy(particle.curve.getPointAt(progress))
        particle.mesh.scale.setScalar(0.75 + Math.sin(elapsed * 4.5 + index) * 0.22 + pulseValue * 0.18)
      })

      pointField.rotation.y = elapsed * 0.025
      pointField.rotation.z = Math.sin(elapsed * 0.18) * 0.03
      platform.rotation.y = elapsed * 0.05
      platform.scale.setScalar(1 + Math.sin(elapsed * 1.3) * 0.015 + pulseValue * 0.04)
    }

    world.rotation.x = pointerY * -0.1 + scrollValue * 0.035
    world.rotation.y = pointerX * 0.13 - scrollValue * 0.12
    world.position.y = -scrollValue * 0.2
    camera.position.x = pointerX * 0.65
    camera.position.y = 0.35 - pointerY * 0.5
    camera.position.z = 11.4 - pulseValue * 0.22
    camera.lookAt(0, -0.08, 0)
    renderer.render(scene, camera)
  }

  if (reducedMotion) renderer.render(scene, camera)
  else animationFrame = window.requestAnimationFrame(animate)

  return () => {
    stopped = true
    window.cancelAnimationFrame(animationFrame)
    visual.removeEventListener('pointermove', handlePointerMove)
    visual.removeEventListener('pointerleave', handlePointerLeave)
    visual.removeEventListener('pointerdown', handlePointerDown)
    window.removeEventListener('scroll', handleScroll)
    intersectionObserver.disconnect()
    resizeObserver.disconnect()
    stopThemeObserver()
    resources.forEach((resource) => resource.dispose())
    renderer.dispose()
    host.remove()
    panel.classList.remove('three-ad-ready')
  }
}
