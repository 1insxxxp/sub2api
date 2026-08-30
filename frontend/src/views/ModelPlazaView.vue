<template>
  <!-- 后台内嵌形态:?embedded=1 且已登录,套完整后台布局 -->
  <AppLayout v-if="isEmbedded">
    <section
      v-if="useExternalModelSquare"
      class="model-plaza-frame-page model-plaza-frame-page--embedded"
    >
      <div v-if="!iframeLoaded" class="model-plaza-frame-loading">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-600/25 border-t-primary-600 dark:border-primary-400/25 dark:border-t-primary-400"></div>
      </div>
      <iframe
        class="model-plaza-frame"
        :src="modelSquareUrl"
        :title="t('modelPlaza.title')"
        referrerpolicy="strict-origin-when-cross-origin"
        allow="clipboard-read; clipboard-write"
        @load="handleIframeLoad"
        @error="handleIframeError"
      ></iframe>
      <div v-if="iframeFailed" class="model-plaza-frame-error">
        <span>{{ t('modelPlaza.loadFailed') }}</span>
        <a :href="modelSquareUrl" target="_blank" rel="noopener noreferrer">
          {{ t('customPage.openInNewTab') }}
        </a>
        <RouterLink :to="internalModelPlazaRoute">{{ t('modelPlaza.title') }}</RouterLink>
      </div>
    </section>
    <ModelPlazaContent
      v-else
      :response="data"
      :loading="loading"
      :error="loadFailed"
      embedded
    />
  </AppLayout>

  <!-- 独立形态:自带导航条(logo/站名 + 登录/回后台) -->
  <div v-else class="min-h-screen min-w-0 overflow-x-clip bg-gray-50 dark:bg-dark-950">
    <PlazaNavBar />
    <main
      v-if="useExternalModelSquare"
      class="model-plaza-frame-page model-plaza-frame-page--standalone"
    >
      <div v-if="!iframeLoaded" class="model-plaza-frame-loading">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-600/25 border-t-primary-600 dark:border-primary-400/25 dark:border-t-primary-400"></div>
      </div>
      <iframe
        class="model-plaza-frame"
        :src="modelSquareUrl"
        :title="t('modelPlaza.title')"
        referrerpolicy="strict-origin-when-cross-origin"
        allow="clipboard-read; clipboard-write"
        @load="handleIframeLoad"
        @error="handleIframeError"
      ></iframe>
      <div v-if="iframeFailed" class="model-plaza-frame-error">
        <span>{{ t('modelPlaza.loadFailed') }}</span>
        <a :href="modelSquareUrl" target="_blank" rel="noopener noreferrer">
          {{ t('customPage.openInNewTab') }}
        </a>
        <RouterLink :to="internalModelPlazaRoute">{{ t('modelPlaza.title') }}</RouterLink>
      </div>
    </main>
    <main v-else class="mx-auto min-w-0 max-w-7xl px-4 py-6 sm:px-6 lg:px-8 lg:py-8">
      <ModelPlazaContent :response="data" :loading="loading" :error="loadFailed" />
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import ModelPlazaContent from '@/components/modelPlaza/ModelPlazaContent.vue'
import PlazaNavBar from '@/components/modelPlaza/PlazaNavBar.vue'
import { getModelPlaza, type ModelPlazaResponse } from '@/api/modelPlaza'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()
const { t } = useI18n()

// embedded=1 但未登录(如转发的链接)自动降级为独立形态。
const isEmbedded = computed(() => route.query.embedded === '1' && authStore.isAuthenticated)
const useExternalModelSquare = computed(() => route.query.source !== 'internal')
const internalModelPlazaRoute = computed(() => ({
  path: route.path,
  query: { ...route.query, source: 'internal' },
}))

const modelSquareUrl = 'https://new.passionapi.com/pricing'
const iframeLoaded = ref(false)
const iframeFailed = ref(false)
const data = ref<ModelPlazaResponse | null>(null)
const loading = ref(true)
const loadFailed = ref(false)

function handleIframeLoad() {
  iframeLoaded.value = true
  iframeFailed.value = false
}

function handleIframeError() {
  iframeLoaded.value = true
  iframeFailed.value = true
}

onMounted(async () => {
  // 独立形态导航条需要站点名/Logo;有 __APP_CONFIG__ 注入时同步命中缓存。
  void appStore.fetchPublicSettings()
  try {
    data.value = await getModelPlaza()
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.model-plaza-frame-page {
  position: relative;
  min-height: 720px;
}

.model-plaza-frame-page--embedded {
  height: calc(100dvh - 5rem);
  padding: 0.75rem;
}

.model-plaza-frame-page--standalone {
  height: calc(100dvh - 4rem);
  padding: 0.75rem;
}

.model-plaza-frame {
  height: 100%;
  width: 100%;
  border: 0;
  border-radius: 0.75rem;
  background: #fff;
}

.model-plaza-frame-loading {
  position: absolute;
  inset: 0.75rem;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 0.75rem;
  background: rgb(249 250 251 / 0.92);
}

.model-plaza-frame-error {
  position: absolute;
  left: 50%;
  top: 50%;
  z-index: 2;
  display: flex;
  max-width: calc(100% - 2rem);
  transform: translate(-50%, -50%);
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
  border-radius: 0.75rem;
  border: 1px solid rgb(254 202 202);
  background: rgb(254 242 242);
  padding: 1rem 1.25rem;
  text-align: center;
  font-size: 0.875rem;
  color: rgb(220 38 38);
}

.model-plaza-frame-error a {
  font-weight: 700;
  color: rgb(37 99 235);
  text-decoration: underline;
  text-underline-offset: 3px;
}

:global(.dark) .model-plaza-frame {
  background: #020617;
}

:global(.dark) .model-plaza-frame-loading {
  background: rgb(2 6 23 / 0.9);
}

:global(.dark) .model-plaza-frame-error {
  border-color: rgb(127 29 29 / 0.6);
  background: rgb(127 29 29 / 0.28);
  color: rgb(252 165 165);
}

:global(.dark) .model-plaza-frame-error a {
  color: rgb(147 197 253);
}
</style>
