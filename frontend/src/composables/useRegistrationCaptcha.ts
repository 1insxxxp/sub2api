import { computed, ref } from 'vue'
import { getPublicSettings } from '@/api/auth'
import { extractApiErrorCode } from '@/utils/apiError'
import type { PublicSettings } from '@/types'

export function requiresRegistrationCaptcha(error: unknown) {
  return [
    'REGISTRATION_CAPTCHA_REQUIRED',
    'TURNSTILE_VERIFICATION_FAILED',
    'TENCENT_CAPTCHA_VERIFICATION_FAILED',
    'ALIYUN_CAPTCHA_VERIFICATION_FAILED'
  ].includes(extractApiErrorCode(error) || '')
}

export function useRegistrationCaptcha() {
  const turnstileEnabled = ref(false)
  const turnstileSiteKey = ref('')
  const tencentCaptchaEnabled = ref(false)
  const tencentCaptchaAppId = ref('')
  const tencentCaptchaRegion = ref('cn')
  const aliyunCaptchaEnabled = ref(false)
  const aliyunCaptchaSceneId = ref('')
  const aliyunCaptchaPrefix = ref('')
  const aliyunCaptchaRegion = ref('cn')
  const actionCaptchaEnabled = computed(() =>
    (tencentCaptchaEnabled.value && Boolean(tencentCaptchaAppId.value)) ||
    (aliyunCaptchaEnabled.value && Boolean(aliyunCaptchaSceneId.value && aliyunCaptchaPrefix.value))
  )
  const captchaEnabled = computed(() =>
    actionCaptchaEnabled.value || (turnstileEnabled.value && Boolean(turnstileSiteKey.value))
  )

  function applyCaptchaSettings(settings: PublicSettings) {
    turnstileEnabled.value = settings.turnstile_enabled === true
    turnstileSiteKey.value = settings.turnstile_site_key || ''
    tencentCaptchaEnabled.value = settings.tencent_captcha_enabled === true
    tencentCaptchaAppId.value = settings.tencent_captcha_app_id || ''
    tencentCaptchaRegion.value = settings.tencent_captcha_region || 'cn'
    aliyunCaptchaEnabled.value = settings.aliyun_captcha_enabled === true
    aliyunCaptchaSceneId.value = settings.aliyun_captcha_scene_id || ''
    aliyunCaptchaPrefix.value = settings.aliyun_captcha_prefix || ''
    aliyunCaptchaRegion.value = settings.aliyun_captcha_region || 'cn'
  }

  async function refreshChallenge(error: unknown) {
    if (!requiresRegistrationCaptcha(error)) return false
    try {
      applyCaptchaSettings(await getPublicSettings('registration'))
      return captchaEnabled.value
    } catch {
      // Keep the server's submission error and the current form state.
      return false
    }
  }

  return {
    turnstileEnabled, turnstileSiteKey, tencentCaptchaEnabled, tencentCaptchaAppId,
    tencentCaptchaRegion, aliyunCaptchaEnabled, aliyunCaptchaSceneId, aliyunCaptchaPrefix,
    aliyunCaptchaRegion, actionCaptchaEnabled, captchaEnabled, applyCaptchaSettings, refreshChallenge
  }
}
