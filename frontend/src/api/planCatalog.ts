import { apiClient } from './client'
import type { PlanCatalogItem } from '@/types/planCatalog'

export const planCatalogAPI = {
  list() {
    return apiClient.get<PlanCatalogItem[]>('/plan-catalog')
  },
}

