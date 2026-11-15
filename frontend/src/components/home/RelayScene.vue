<template>
  <div ref="host" class="relay-scene" aria-hidden="true">
    <canvas ref="canvas" class="relay-scene-canvas"></canvas>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { createRelayScene, type RelaySceneHandle } from './relayScene'

const props = defineProps<{ dark: boolean; paused?: boolean }>()

const host = ref<HTMLDivElement | null>(null)
const canvas = ref<HTMLCanvasElement | null>(null)
let handle: RelaySceneHandle | null = null

onMounted(() => {
  if (!host.value || !canvas.value) return
  try {
    handle = createRelayScene(host.value, canvas.value, { dark: props.dark })
  } catch (error) {
    // WebGL unavailable (old device, disabled GPU): keep the CSS fallback visible.
    console.warn('Relay scene disabled', error)
    host.value.classList.add('relay-scene-unavailable')
  }
})

watch(
  () => props.dark,
  (dark) => handle?.setDark(dark),
)

watch(
  () => props.paused,
  (paused) => handle?.setPaused(paused ?? false),
)

onBeforeUnmount(() => {
  handle?.dispose()
  handle = null
})
</script>

<style scoped>
.relay-scene {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.relay-scene-canvas {
  display: block;
  width: 100%;
  height: 100%;
}
</style>
