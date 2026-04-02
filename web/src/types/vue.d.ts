import type { App } from 'vue'
import type { RouteLocationNormalized } from 'vue-router'

declare module 'vue' {
  interface ComponentCustomProperties {
    $theme: 'light' | 'dark'
  }
}

declare global {
  interface Window {
    __VUE_DEVTOOLS_GLOBAL_HOOK__?: unknown
  }
}

export {}

export interface ThemeInstance {
  current: 'light' | 'dark'
  toggle: () => void
  setTheme: (theme: 'light' | 'dark') => void
}

export interface NavigationGuard {
  (to: RouteLocationNormalized, from: RouteLocationNormalized): boolean | string | void
}

export interface InstallablePlugin {
  install: (app: App) => void
}
