import { inject, ref, type InjectionKey, type Ref } from 'vue'

// Appearance follows the current page, including components teleported to body.
export const adminAppearanceKey: InjectionKey<Readonly<Ref<boolean>>> = Symbol('adminAppearance')

export function useAdminAppearance() {
  return inject(adminAppearanceKey, ref(false))
}
