import * as THREE from 'three'
import { RoomEnvironment } from 'three/examples/jsm/environments/RoomEnvironment.js'
import {
  BloomEffect,
  ChromaticAberrationEffect,
  EffectComposer,
  EffectPass,
  NoiseEffect,
  RenderPass,
  ToneMappingEffect,
  ToneMappingMode,
  VignetteEffect,
  BlendFunction,
} from 'postprocessing'
import {
  beamFragment,
  coreFragment,
  coreVertex,
  dustFragment,
  dustVertex,
  haloFragment,
  uvVertex,
} from './relayShaders'

interface RelayPalette {
  primary: THREE.Color
  primaryDeep: THREE.Color
  cyan: THREE.Color
  cyanSoft: THREE.Color
  glass: THREE.Color
  attenuation: THREE.Color
  node: THREE.Color
  ambient: THREE.Color
  dustOpacity: number
  haloIntensity: number
  edgeOpacity: number
}

// The daytime scene is warm and atmospheric; the nighttime scene is deep-space blue.
const SOLAR_PALETTE: RelayPalette = {
  primary: new THREE.Color(0xf59e0b),
  primaryDeep: new THREE.Color(0xea580c),
  cyan: new THREE.Color(0x0ea5e9),
  cyanSoft: new THREE.Color(0x7dd3fc),
  glass: new THREE.Color(0xfef3c7),
  attenuation: new THREE.Color(0xf59e0b),
  node: new THREE.Color(0x7c2d12),
  ambient: new THREE.Color(0xfff7ed),
  dustOpacity: 0.52,
  haloIntensity: 0.38,
  edgeOpacity: 0.28,
}

const COSMIC_PALETTE: RelayPalette = {
  primary: new THREE.Color(0x3b82f6),
  primaryDeep: new THREE.Color(0x1d4ed8),
  cyan: new THREE.Color(0x06b6d4),
  cyanSoft: new THREE.Color(0x67e8f9),
  glass: new THREE.Color(0x93c5fd),
  attenuation: new THREE.Color(0x1d4ed8),
  node: new THREE.Color(0x0b1f3a),
  ambient: new THREE.Color(0x9ec5ff),
  dustOpacity: 0.85,
  haloIntensity: 0.7,
  edgeOpacity: 0.45,
}

const NODE_COUNT = 8
const NODE_RADIUS = 3.4
const DUST_COUNT = 260
const MAX_PIXEL_RATIO = 1.75
const WIDE_LAYOUT_MIN_WIDTH = 1024
const WIDE_LAYOUT_SHIFT_RATIO = 0.24
const INTRO_SECONDS = 2.6
const CAMERA_START_Z = 15
const CAMERA_REST_Z = 9.2
const PARALLAX_DAMPING = 0.045

export interface RelaySceneHandle {
  setDark(dark: boolean): void
  setPaused(paused: boolean): void
  dispose(): void
}

type Uniforms = Record<string, THREE.IUniform>

interface Bag {
  materials: THREE.Material[]
  geometries: THREE.BufferGeometry[]
  timed: Uniforms[]
}

function keep<T extends THREE.Material | THREE.BufferGeometry>(bag: Bag, item: T): T {
  if (item instanceof THREE.Material) bag.materials.push(item)
  else bag.geometries.push(item as THREE.BufferGeometry)
  return item
}

function shader(bag: Bag, vertexShader: string, fragmentShader: string, uniforms: Uniforms, extra: Partial<THREE.ShaderMaterialParameters> = {}) {
  const all = { uTime: { value: 0 }, ...uniforms }
  bag.timed.push(all)
  return keep(bag, new THREE.ShaderMaterial({ vertexShader, fragmentShader, uniforms: all, ...extra }))
}

const easeOutExpo = (t: number) => (t >= 1 ? 1 : 1 - Math.pow(2, -10 * t))

function nodePosition(index: number) {
  const angle = (index / NODE_COUNT) * Math.PI * 2 + 0.3
  const lift = Math.sin(angle * 2) * 0.9
  return new THREE.Vector3(Math.cos(angle) * NODE_RADIUS, lift, Math.sin(angle) * NODE_RADIUS * 0.6)
}
// Glass shell around a plasma core, plus a soft halo sprite behind it.
function buildCore(bag: Bag, palette: RelayPalette) {
  const group = new THREE.Group()
  const plasma = new THREE.Mesh(
    keep(bag, new THREE.IcosahedronGeometry(0.95, 24)),
    shader(bag, coreVertex, coreFragment, {
      uColorA: { value: palette.primaryDeep.clone() },
      uColorB: { value: palette.cyanSoft.clone() },
    }),
  )
  const glass = new THREE.Mesh(
    keep(bag, new THREE.IcosahedronGeometry(1.45, 2)),
    keep(bag, new THREE.MeshPhysicalMaterial({
      color: palette.glass.clone(),
      metalness: 0,
      roughness: 0.12,
      transmission: 1,
      thickness: 0.6,
      ior: 1.35,
      attenuationColor: palette.attenuation.clone(),
      attenuationDistance: 2.5,
      iridescence: 0.6,
      iridescenceIOR: 1.3,
      clearcoat: 0.6,
      envMapIntensity: 0.45,
      flatShading: true,
    })),
  )
  const edges = new THREE.LineSegments(
    keep(bag, new THREE.EdgesGeometry(new THREE.IcosahedronGeometry(1.47, 2))),
    keep(bag, new THREE.LineBasicMaterial({ color: palette.cyanSoft.clone(), transparent: true, opacity: 0.35 })),
  )
  const halo = new THREE.Mesh(
    keep(bag, new THREE.PlaneGeometry(9, 9)),
    shader(bag, uvVertex, haloFragment, { uColor: { value: palette.primary.clone() }, uIntensity: { value: palette.haloIntensity } }, {
      transparent: true,
      depthWrite: false,
      blending: THREE.AdditiveBlending,
    }),
  )
  halo.position.z = -1.5
  group.add(halo, plasma, glass, edges)
  return { group, plasma, glass, edges, halo }
}

function buildRings(bag: Bag, palette: RelayPalette) {
  const group = new THREE.Group()
  const materials: THREE.MeshBasicMaterial[] = []
  const specs = [
    { r: 2.2, tube: 0.008, tilt: 1.25, opacity: 0.7 },
    { r: 2.6, tube: 0.004, tilt: 1.1, opacity: 0.35 },
    { r: NODE_RADIUS, tube: 0.005, tilt: 1.42, opacity: 0.22 },
  ]
  for (const s of specs) {
    const ring = new THREE.Mesh(
      keep(bag, new THREE.TorusGeometry(s.r, s.tube, 6, 220)),
      keep(bag, new THREE.MeshBasicMaterial({ color: palette.cyanSoft.clone(), transparent: true, opacity: s.opacity, blending: THREE.AdditiveBlending, depthWrite: false })),
    )
    materials.push(ring.material as THREE.MeshBasicMaterial)
    ring.rotation.x = s.tilt
    group.add(ring)
  }
  return { group, materials }
}

function buildNodes(bag: Bag, palette: RelayPalette) {
  const group = new THREE.Group()
  const geometry = keep(bag, new THREE.OctahedronGeometry(0.16, 0))
  const material = keep(bag, new THREE.MeshStandardMaterial({
    color: palette.node.clone(),
    emissive: palette.cyan.clone(),
    emissiveIntensity: 2.4,
    metalness: 0.6,
    roughness: 0.2,
  }))
  for (let i = 0; i < NODE_COUNT; i += 1) {
    const node = new THREE.Mesh(geometry, material)
    node.position.copy(nodePosition(i))
    group.add(node)
  }
  return { group, material }
}

// Each route is a thin tube with a shader-driven light pulse running core <-> node.
function buildBeams(bag: Bag, palette: RelayPalette) {
  const group = new THREE.Group()
  const materials: THREE.ShaderMaterial[] = []
  for (let i = 0; i < NODE_COUNT; i += 1) {
    const end = nodePosition(i)
    const control = end.clone().multiplyScalar(0.45).add(new THREE.Vector3(0, 1.3 + (i % 3) * 0.25, 0))
    const curve = new THREE.QuadraticBezierCurve3(new THREE.Vector3(0, 0, 0), control, end)
    const beam = new THREE.Mesh(
      keep(bag, new THREE.TubeGeometry(curve, 96, 0.014, 6, false)),
      shader(bag, uvVertex, beamFragment, {
        uSpeed: { value: 0.22 + (i % 4) * 0.05 },
        uOffset: { value: i / NODE_COUNT },
        uDirection: { value: i % 2 === 0 ? 1 : -1 },
        uColor: { value: (i % 3 === 0 ? palette.primary : palette.cyanSoft).clone() },
      }, { transparent: true, depthWrite: false, blending: THREE.AdditiveBlending }),
    )
    materials.push(beam.material as THREE.ShaderMaterial)
    group.add(beam)
  }
  return { group, materials }
}
function buildDust(bag: Bag, palette: RelayPalette) {
  const positions = new Float32Array(DUST_COUNT * 3)
  const scales = new Float32Array(DUST_COUNT)
  const phases = new Float32Array(DUST_COUNT)
  for (let i = 0; i < DUST_COUNT; i += 1) {
    const radius = 4.5 + Math.random() * 10
    const theta = Math.random() * Math.PI * 2
    const phi = Math.acos(2 * Math.random() - 1)
    positions[i * 3] = radius * Math.sin(phi) * Math.cos(theta)
    positions[i * 3 + 1] = radius * Math.cos(phi) * 0.55
    positions[i * 3 + 2] = radius * Math.sin(phi) * Math.sin(theta) - 3
    scales[i] = 0.4 + Math.random() * 1.6
    phases[i] = Math.random() * Math.PI * 2
  }
  const geometry = keep(bag, new THREE.BufferGeometry())
  geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3))
  geometry.setAttribute('aScale', new THREE.BufferAttribute(scales, 1))
  geometry.setAttribute('aPhase', new THREE.BufferAttribute(phases, 1))
  const material = shader(bag, dustVertex, dustFragment, {
    uPixelRatio: { value: 1 },
    uSize: { value: 4 },
    uColor: { value: palette.cyanSoft.clone() },
    uOpacity: { value: palette.dustOpacity * 0.45 },
  }, { transparent: true, depthWrite: false, blending: THREE.AdditiveBlending })
  return { points: new THREE.Points(geometry, material), material }
}

function addLights(scene: THREE.Scene, palette: RelayPalette) {
  const ambient = new THREE.AmbientLight(palette.ambient.clone(), 0.35)
  const key = new THREE.DirectionalLight(0xffffff, 1.2)
  key.position.set(4, 6, 6)
  const rim = new THREE.PointLight(palette.cyanSoft.clone(), 40, 18)
  rim.position.set(-3, 1, 2)
  const fill = new THREE.PointLight(palette.primary.clone(), 22, 16)
  fill.position.set(2, -3, -2)
  scene.add(ambient, key, rim, fill)
  return { ambient, key, rim, fill }
}

function setupEnvironment(renderer: THREE.WebGLRenderer, scene: THREE.Scene) {
  const pmrem = new THREE.PMREMGenerator(renderer)
  const room = new RoomEnvironment()
  scene.environment = pmrem.fromScene(room, 0.04).texture
  room.dispose()
  pmrem.dispose()
}
function buildComposer(renderer: THREE.WebGLRenderer, scene: THREE.Scene, camera: THREE.Camera) {
  const composer = new EffectComposer(renderer, { multisampling: 4 })
  composer.addPass(new RenderPass(scene, camera))
  const bloom = new BloomEffect({ intensity: 1.1, luminanceThreshold: 0.42, luminanceSmoothing: 0.25, mipmapBlur: true, radius: 0.75 })
  const aberration = new ChromaticAberrationEffect({ offset: new THREE.Vector2(0.0006, 0.0006), radialModulation: true, modulationOffset: 0.35 })
  const vignette = new VignetteEffect({ darkness: 0.55, offset: 0.35 })
  const noise = new NoiseEffect({ blendFunction: BlendFunction.SOFT_LIGHT, premultiply: true })
  noise.blendMode.opacity.value = 0
  const tone = new ToneMappingEffect({ mode: ToneMappingMode.ACES_FILMIC })
  composer.addPass(new EffectPass(camera, bloom, aberration, vignette, noise, tone))
  return composer
}

export function createRelayScene(
  host: HTMLElement,
  canvas: HTMLCanvasElement,
  options: { dark: boolean },
): RelaySceneHandle {
  const renderer = new THREE.WebGLRenderer({ canvas, antialias: false, alpha: true, powerPreference: 'high-performance' })
  const pixelRatio = Math.min(window.devicePixelRatio || 1, MAX_PIXEL_RATIO)
  renderer.setPixelRatio(pixelRatio)
  renderer.setClearColor(0x000000, 0)
  renderer.toneMapping = THREE.NoToneMapping

  const scene = new THREE.Scene()
  const camera = new THREE.PerspectiveCamera(38, 1, 0.1, 100)
  camera.position.set(0, 0.6, CAMERA_START_Z)

  setupEnvironment(renderer, scene)
  const initialPalette = options.dark ? COSMIC_PALETTE : SOLAR_PALETTE
  const lights = addLights(scene, initialPalette)

  const bag: Bag = { materials: [], geometries: [], timed: [] }
  const core = buildCore(bag, initialPalette)
  const rings = buildRings(bag, initialPalette)
  const nodes = buildNodes(bag, initialPalette)
  const beams = buildBeams(bag, initialPalette)
  const dust = buildDust(bag, initialPalette)
  scene.add(core.group, rings.group, nodes.group, beams.group, dust.points)
  ;(dust.material.uniforms.uPixelRatio as THREE.IUniform).value = pixelRatio

  const composer = buildComposer(renderer, scene, camera)
  const pointer = new THREE.Vector2(0, 0)
  const reduceMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false
  const clock = new THREE.Clock()
  const startedAt = performance.now()
  let frame = 0
  let visible = true
  let paused = false
  let dark = options.dark
  let themeMix = options.dark ? 1 : 0
  let themeTarget = themeMix
  const mixedPrimary = new THREE.Color()
  const mixedPrimaryDeep = new THREE.Color()
  const mixedCyan = new THREE.Color()
  const mixedCyanSoft = new THREE.Color()
  const mixedGlass = new THREE.Color()
  const mixedAttenuation = new THREE.Color()
  const mixedNode = new THREE.Color()
  const mixedAmbient = new THREE.Color()

  function updateTheme(delta: number) {
    themeMix = reduceMotion
      ? themeTarget
      : THREE.MathUtils.damp(themeMix, themeTarget, 5.8, delta)
    const mixColor = (target: THREE.Color, solar: THREE.Color, cosmic: THREE.Color) =>
      target.copy(solar).lerp(cosmic, themeMix)
    mixColor(mixedPrimary, SOLAR_PALETTE.primary, COSMIC_PALETTE.primary)
    mixColor(mixedPrimaryDeep, SOLAR_PALETTE.primaryDeep, COSMIC_PALETTE.primaryDeep)
    mixColor(mixedCyan, SOLAR_PALETTE.cyan, COSMIC_PALETTE.cyan)
    mixColor(mixedCyanSoft, SOLAR_PALETTE.cyanSoft, COSMIC_PALETTE.cyanSoft)
    mixColor(mixedGlass, SOLAR_PALETTE.glass, COSMIC_PALETTE.glass)
    mixColor(mixedAttenuation, SOLAR_PALETTE.attenuation, COSMIC_PALETTE.attenuation)
    mixColor(mixedNode, SOLAR_PALETTE.node, COSMIC_PALETTE.node)
    mixColor(mixedAmbient, SOLAR_PALETTE.ambient, COSMIC_PALETTE.ambient)

    const plasmaUniforms = (core.plasma.material as THREE.ShaderMaterial).uniforms
    plasmaUniforms.uColorA.value.copy(mixedPrimaryDeep)
    plasmaUniforms.uColorB.value.copy(mixedCyanSoft)
    const haloUniforms = (core.halo.material as THREE.ShaderMaterial).uniforms
    haloUniforms.uColor.value.copy(mixedPrimary)
    haloUniforms.uIntensity.value = THREE.MathUtils.lerp(SOLAR_PALETTE.haloIntensity, COSMIC_PALETTE.haloIntensity, themeMix)
    ;(core.glass.material as THREE.MeshPhysicalMaterial).color.copy(mixedGlass)
    ;(core.glass.material as THREE.MeshPhysicalMaterial).attenuationColor.copy(mixedAttenuation)
    ;(core.edges.material as THREE.LineBasicMaterial).color.copy(mixedCyanSoft)
    ;(core.edges.material as THREE.LineBasicMaterial).opacity = THREE.MathUtils.lerp(SOLAR_PALETTE.edgeOpacity, COSMIC_PALETTE.edgeOpacity, themeMix)
    rings.materials.forEach((material) => material.color.copy(mixedCyanSoft))
    nodes.material.color.copy(mixedNode)
    nodes.material.emissive.copy(mixedCyan)
    beams.materials.forEach((material, index) => {
      material.uniforms.uColor.value.copy(index % 3 === 0 ? mixedPrimary : mixedCyanSoft)
    })
    ;(dust.material.uniforms.uColor as THREE.IUniform).value.copy(mixedCyanSoft)
    ;(dust.material.uniforms.uOpacity as THREE.IUniform).value = THREE.MathUtils.lerp(SOLAR_PALETTE.dustOpacity, COSMIC_PALETTE.dustOpacity, themeMix)
    lights.ambient.color.copy(mixedAmbient)
    lights.rim.color.copy(mixedCyanSoft)
    lights.fill.color.copy(mixedPrimary)
  }

  function resize() {
    const { clientWidth, clientHeight } = host
    if (!clientWidth || !clientHeight) return
    renderer.setSize(clientWidth, clientHeight, false)
    composer.setSize(clientWidth, clientHeight, false)
    camera.aspect = clientWidth / clientHeight
    // Wide layouts push the relay core into the right column, clear of the headline.
    if (clientWidth >= WIDE_LAYOUT_MIN_WIDTH) {
      camera.setViewOffset(clientWidth, clientHeight, -clientWidth * WIDE_LAYOUT_SHIFT_RATIO, -clientHeight * 0.03, clientWidth, clientHeight)
    } else {
      camera.clearViewOffset()
    }
    camera.updateProjectionMatrix()
  }

  function setDark(next: boolean) {
    dark = next
    themeTarget = next ? 1 : 0
    if (reduceMotion) updateTheme(0)
  }

  function renderFrame() {
    const delta = Math.min(clock.getDelta(), 0.1)
    const elapsed = clock.elapsedTime
    for (const uniforms of bag.timed) uniforms.uTime.value = elapsed
    updateTheme(delta)
    const intro = easeOutExpo(Math.min((performance.now() - startedAt) / 1000 / INTRO_SECONDS, 1))
    const restZ = CAMERA_REST_Z + (dark ? 0 : 0.4)
    camera.position.z = CAMERA_START_Z + (restZ - CAMERA_START_Z) * intro
    camera.position.x += (pointer.x * 1.1 - camera.position.x) * PARALLAX_DAMPING
    camera.position.y += (0.6 + pointer.y * 0.5 - camera.position.y) * PARALLAX_DAMPING
    camera.lookAt(0, 0, 0)
    core.group.rotation.y = elapsed * 0.18
    rings.group.rotation.y = elapsed * 0.05
    rings.group.rotation.z = Math.sin(elapsed * 0.3) * 0.04
    nodes.group.children.forEach((node, i) => {
      node.position.y = nodePosition(i).y + Math.sin(elapsed * 1.1 + i) * 0.1
      node.rotation.y = elapsed * 0.8
    })
    dust.points.rotation.y = elapsed * 0.015
    composer.render()
  }

  function loop() {
    frame = requestAnimationFrame(loop)
    if (visible && !paused) renderFrame()
  }

  function onPointerMove(event: PointerEvent) {
    pointer.set((event.clientX / window.innerWidth) * 2 - 1, -((event.clientY / window.innerHeight) * 2 - 1))
  }

  const resizeObserver = new ResizeObserver(resize)
  resizeObserver.observe(host)
  // Pause rendering while the hero is scrolled off screen to save GPU.
  const visibilityObserver = new IntersectionObserver(([entry]) => {
    visible = entry?.isIntersecting ?? true
  })
  visibilityObserver.observe(host)

  resize()
  setDark(options.dark)
  if (reduceMotion) {
    renderFrame()
  } else {
    window.addEventListener('pointermove', onPointerMove, { passive: true })
    loop()
  }

  return {
    setDark,
    setPaused(next: boolean) {
      paused = next
      if (!paused && !reduceMotion && visible) {
        clock.start()
      }
    },
    dispose() {
      cancelAnimationFrame(frame)
      window.removeEventListener('pointermove', onPointerMove)
      resizeObserver.disconnect()
      visibilityObserver.disconnect()
      bag.geometries.forEach((geometry) => geometry.dispose())
      bag.materials.forEach((material) => material.dispose())
      scene.environment?.dispose()
      composer.dispose()
      renderer.dispose()
    },
  }
}
