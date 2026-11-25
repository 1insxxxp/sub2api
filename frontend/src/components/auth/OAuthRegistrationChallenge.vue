<template>
  <CaptchaChallenge
    v-if="captchaEnabled"
    ref="widget"
    :turnstile-enabled="turnstileEnabled"
    :turnstile-site-key="turnstileSiteKey"
    :tencent-enabled="tencentCaptchaEnabled"
    :tencent-app-id="tencentCaptchaAppId"
    :tencent-region="tencentCaptchaRegion"
    :aliyun-enabled="aliyunCaptchaEnabled"
    :aliyun-scene-id="aliyunCaptchaSceneId"
    :aliyun-prefix="aliyunCaptchaPrefix"
    :aliyun-region="aliyunCaptchaRegion"
    @verify="token = $event"
    @expire="token = ''"
    @error="token = ''"
  />
</template>

<script setup lang="ts">
import { ref } from 'vue'
import CaptchaChallenge from '@/components/CaptchaChallenge.vue'
import { useRegistrationCaptcha } from '@/composables/useRegistrationCaptcha'
import type { ActionCaptchaRequestProof } from '@/types'

const {
  turnstileEnabled, turnstileSiteKey, tencentCaptchaEnabled, tencentCaptchaAppId,
  tencentCaptchaRegion, aliyunCaptchaEnabled, aliyunCaptchaSceneId, aliyunCaptchaPrefix,
  aliyunCaptchaRegion, captchaEnabled, actionCaptchaEnabled, refreshChallenge
} = useRegistrationCaptcha()
const token = ref('')
const widget = ref<InstanceType<typeof CaptchaChallenge> | null>(null)

// The first submit can reuse a proof issued when OAuth started. Render a
// challenge only if the server asks for another proof.
async function proof(): Promise<ActionCaptchaRequestProof | null> {
  if (!captchaEnabled.value) return {}
  if (actionCaptchaEnabled.value) {
    const answer = await widget.value?.verifyAction()
    if (!answer) return null
    return tencentCaptchaEnabled.value
      ? { tencent_captcha_ticket: answer.token, tencent_captcha_randstr: answer.randstr }
      : { turnstile_token: answer.token }
  }
  return token.value ? { turnstile_token: token.value } : null
}

async function failed(error: unknown) {
  token.value = ''
  widget.value?.reset()
  await refreshChallenge(error)
}

defineExpose({ proof, failed })
</script>
