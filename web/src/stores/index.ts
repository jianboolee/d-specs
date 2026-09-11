import { createPinia } from 'pinia'

/**
 * 显式导出 pinia 实例。
 *
 * 路由守卫里要用 store，而守卫可能在此之前触发，
 * 显式传入实例可以避免「no active Pinia」这类时序问题。
 */
export const pinia = createPinia()
