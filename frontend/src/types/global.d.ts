import type { InjectedPublicSettings } from '@/types'

declare global {
  interface Window {
    __APP_CONFIG__?: InjectedPublicSettings
  }
}

export {}
