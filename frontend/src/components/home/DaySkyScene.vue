<template>
  <canvas ref="canvas" class="day-sky-canvas" :class="{ 'is-ready': ready }" />
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import {
  Mesh, MeshBasicMaterial, PerspectiveCamera, PlaneGeometry, Scene,
  SRGBColorSpace, TextureLoader, WebGLRenderer,
} from 'three'
import cloudImage from '@/assets/home-cumulus.webp'

const canvas = ref<HTMLCanvasElement | null>(null)
const ready = ref(false)
let dispose: (() => void) | undefined

onMounted(() => {
  const element = canvas.value
  const host = element?.parentElement
  const hero = host?.parentElement
  if (!element || !host || !hero) return

  const motion = window.matchMedia('(prefers-reduced-motion: reduce)')
  if (motion.matches) return
  let renderer: WebGLRenderer
  try {
    renderer = new WebGLRenderer({ canvas: element, alpha: true, powerPreference: 'low-power' })
  } catch {
    return
  }

  const scene = new Scene()
  const camera = new PerspectiveCamera(38, 1, 0.5, 150)
  const geometry = new PlaneGeometry(1, 1)
  const banks = [
    { depth: 72, wide: [0.67, 0.22, 0.49], narrow: [0.65, 0.13, 0.86], opacity: 0.44, speed: 0.12, phase: 1.1, turn: -0.07 },
    { depth: 55, wide: [0.52, 0.48, 0.37], narrow: [0.84, 0.45, 0.87], opacity: 0.36, speed: 0.18, phase: 2.6, turn: 0.13 },
    { depth: 41, wide: [0.87, 0.58, 0.82], narrow: [0.84, 0.53, 1.55], opacity: 0.9, speed: 0.22, phase: 0.6, turn: -0.04 },
    { depth: 30, wide: [0.91, 0.89, 0.72], narrow: [0.81, 0.82, 1.56], opacity: 0.35, speed: 0.29, phase: 3.2, turn: 0.14 },
  ].map(bank => ({
    ...bank,
    mesh: new Mesh(geometry, new MeshBasicMaterial({ transparent: true, opacity: bank.opacity, depthWrite: false })),
  }))
  banks.forEach(({ mesh, turn }) => {
    mesh.rotation.z = turn
    scene.add(mesh)
  })

  let disposed = false
  let textureReady = false
  let inView = true
  let width = 0
  let height = 0
  let frame = 0
  let lastFrame = 0
  let seconds = 0
  let pointerX = 0
  let pointerY = 0

  function resize() {
    if (!host) return
    width = host.clientWidth
    height = host.clientHeight
    if (!width || !height) return
    camera.aspect = width / height
    camera.updateProjectionMatrix()
    renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, width <= 640 ? 1 : 1.35))
    renderer.setSize(width, height, false)
  }

  function updatePointer(event: PointerEvent) {
    if (event.pointerType !== 'mouse' || !host) return
    const bounds = host.getBoundingClientRect()
    pointerX = ((event.clientX - bounds.left) / bounds.width - 0.5) * 2
    pointerY = ((event.clientY - bounds.top) / bounds.height - 0.5) * 2
  }

  function resetPointer() {
    pointerX = 0
    pointerY = 0
  }

  function paint(now: number) {
    frame = 0
    if (disposed || !inView || document.hidden || motion.matches || !textureReady || !width || !height) return
    frame = requestAnimationFrame(paint)
    if (lastFrame && now - lastFrame < 32) return
    if (lastFrame) seconds += Math.min((now - lastFrame) / 1000, 0.1)
    lastFrame = now

    camera.position.x += (pointerX * 0.95 - camera.position.x) * 0.07
    camera.position.y += (-pointerY * 0.62 - camera.position.y) * 0.07
    camera.position.z = Math.sin(seconds * 0.12) * 0.3

    for (const bank of banks) {
      const viewHeight = 2 * Math.tan(camera.fov * Math.PI / 360) * bank.depth
      const visibleWidth = viewHeight * camera.aspect
      const [x, y, size] = width <= 640 ? bank.narrow : bank.wide
      const drift = seconds * bank.speed + bank.phase
      const cloudWidth = visibleWidth * size
      bank.mesh.scale.set(cloudWidth, cloudWidth * 2 / 3, 1)
      bank.mesh.position.set(
        (x - 0.5 + Math.sin(drift * 0.45) * 0.052) * visibleWidth,
        (0.5 - y + Math.cos(drift * 0.38) * 0.018) * viewHeight,
        -bank.depth + Math.sin(drift * 0.32) * 1.15,
      )
    }
    renderer.render(scene, camera)
    ready.value = true
  }

  function syncAnimation() {
    if (frame) cancelAnimationFrame(frame)
    frame = 0
    lastFrame = 0
    if (motion.matches) ready.value = false
    if (!disposed && textureReady && inView && !motion.matches && !document.hidden && width && height) {
      frame = requestAnimationFrame(paint)
    }
  }

  function handleResize() {
    resize()
    syncAnimation()
  }

  const sizeObserver = new ResizeObserver(handleResize)
  const visibilityObserver = new IntersectionObserver(([entry]) => {
    inView = entry.isIntersecting
    syncAnimation()
  })
  const texture = new TextureLoader().load(cloudImage, loaded => {
    if (disposed) return
    loaded.colorSpace = SRGBColorSpace
    for (const { mesh } of banks) {
      mesh.material.map = loaded
      mesh.material.needsUpdate = true
    }
    textureReady = true
    syncAnimation()
  }, undefined, () => dispose?.())

  const handleContextLost = () => {
    ready.value = false
    dispose?.()
  }
  element.addEventListener('webglcontextlost', handleContextLost)
  sizeObserver.observe(host)
  visibilityObserver.observe(host)
  hero.addEventListener('pointermove', updatePointer)
  hero.addEventListener('pointerleave', resetPointer)
  document.addEventListener('visibilitychange', syncAnimation)
  motion.addEventListener('change', syncAnimation)
  resize()

  dispose = () => {
    if (disposed) return
    disposed = true
    syncAnimation()
    ready.value = false
    sizeObserver.disconnect()
    visibilityObserver.disconnect()
    element.removeEventListener('webglcontextlost', handleContextLost)
    hero.removeEventListener('pointermove', updatePointer)
    hero.removeEventListener('pointerleave', resetPointer)
    document.removeEventListener('visibilitychange', syncAnimation)
    motion.removeEventListener('change', syncAnimation)
    texture.dispose()
    banks.forEach(({ mesh }) => mesh.material.dispose())
    geometry.dispose()
    renderer.dispose()
    renderer.forceContextLoss()
  }
})

onBeforeUnmount(() => dispose?.())
</script>

<style scoped>
.day-sky-canvas {
  position: absolute;
  inset: 0;
  display: block;
  width: 100%;
  height: 100%;
  opacity: 0;
}
.day-sky-canvas.is-ready { opacity: 1; }
</style>
