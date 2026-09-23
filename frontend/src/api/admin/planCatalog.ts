import { apiClient } from '../client'
import type { PlanCatalogInput, PlanCatalogItem } from '@/types/planCatalog'

export const adminPlanCatalogAPI = {
  list() { return apiClient.get<PlanCatalogItem[]>('/admin/plan-catalog') },
  create(data: PlanCatalogInput) { return apiClient.post<PlanCatalogItem>('/admin/plan-catalog', data) },
  update(id: number, data: PlanCatalogInput) { return apiClient.put<PlanCatalogItem>(`/admin/plan-catalog/${id}`, data) },
  delete(id: number) { return apiClient.delete(`/admin/plan-catalog/${id}`) },
}
