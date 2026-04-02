import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { configApi } from '@/api'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/setup',
      name: 'Setup',
      component: () => import('@/views/SetupWizard.vue'),
      meta: { public: true }
    },
    {
      path: '/login',
      name: 'Login',
      component: () => import('@/views/Login.vue'),
      meta: { public: true }
    },
    {
      path: '/',
      name: 'Chat',
      component: () => import('@/views/Chat/Index.vue'),
      meta: { requiresAuth: false }
    },
    {
      path: '/admin',
      name: 'Admin',
      component: () => import('@/views/Admin/Layout.vue'),
      meta: { requiresAuth: true },
      children: [
        {
          path: '',
          name: 'AdminDashboard',
          component: () => import('@/views/Admin/Dashboard.vue')
        },
        {
          path: 'config',
          name: 'AdminConfig',
          component: () => import('@/views/Admin/Config.vue')
        }
      ]
    },
    {
      path: '/:pathMatch(.*)*',
      redirect: '/'
    }
  ]
})

router.beforeEach(async (to, _from, next) => {
  const authStore = useAuthStore()

  if (to.meta.public) {
    if (to.name === 'Login') {
      try {
        const status = await configApi.getStatus()
        if (status.data && !status.data.has_password) {
          return next({ name: 'Setup' })
        }
        if (status.data?.has_password && authStore.isAuthenticated) {
          return next({ name: 'AdminDashboard' })
        }
      } catch {
        void 0
      }
    }
    return next()
  }

  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    return next({ name: 'Login' })
  }

  next()
})

export default router
