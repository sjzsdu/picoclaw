import { launcherFetch } from "@/api/http"
import { refreshGatewayState } from "@/store/gateway"

// API client for model list management.

export interface ModelInfo {
  index: number
  model_name: string
  provider?: string
  model: string
  api_base?: string
  api_key: string
  proxy?: string
  auth_method?: string
  // Advanced fields
  connect_mode?: string
  workspace?: string
  rpm?: number
  max_tokens_field?: string
  request_timeout?: number
  thinking_level?: string
  tool_schema_transform?: string
  disable_tools?: boolean
  extra_body?: Record<string, unknown>
  custom_headers?: Record<string, string>
  // Meta
  available: boolean
  status: "available" | "unconfigured" | "unreachable"
  status_reason?: string
  last_test_status?: string
  last_test_reason?: string
  last_test_message?: string
  last_tested_at_unix?: number
  is_default: boolean
  is_virtual: boolean
  default_model_allowed?: boolean
}

export interface ModelProviderOption {
  id: string
  default_api_base: string
  empty_api_key_allowed: boolean
  create_allowed: boolean
  default_model_allowed: boolean
  default_auth_method?: string
  auth_method_locked?: boolean
}

interface ModelsListResponse {
  models: ModelInfo[]
  total: number
  default_model: string
  provider_options: ModelProviderOption[]
}

interface ModelActionResponse {
  status: string
  index?: number
  default_model?: string
}

export interface ModelTestResponse {
  status: string
  message?: string
  available?: boolean
  reason?: string
}

export interface ModelBatchTestResult {
  index: number
  model_name: string
  available: boolean
  status: "available" | "unreachable" | "unconfigured"
  reason?: string
  last_test_status?: string
  last_test_reason?: string
  last_test_message?: string
  last_tested_at_unix?: number
}

export interface ModelBatchTestResponse {
  status: string
  results: ModelBatchTestResult[]
}

interface TestModelPayload {
  model_name: string
  model: string
  api_base?: string
  proxy?: string
  auth_method?: string
  connect_mode?: string
  workspace?: string
  rpm?: number
  max_tokens_field?: string
  request_timeout?: number
  thinking_level?: string
  disable_tools?: boolean
  extra_body?: Record<string, unknown>
  custom_headers?: Record<string, string>
  index?: number
  include_tools?: boolean
}

const BASE_URL = ""

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await launcherFetch(`${BASE_URL}${path}`, options)
  if (!res.ok) {
    throw new Error(`API error: ${res.status} ${res.statusText}`)
  }
  return res.json() as Promise<T>
}

export async function getModels(): Promise<ModelsListResponse> {
  return request<ModelsListResponse>("/api/models")
}

export async function addModel(
  model: Partial<ModelInfo>,
): Promise<ModelActionResponse> {
  return request<ModelActionResponse>("/api/models", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(model),
  })
}

export async function updateModel(
  index: number,
  model: Partial<ModelInfo>,
): Promise<ModelActionResponse> {
  return request<ModelActionResponse>(`/api/models/${index}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(model),
  })
}

export async function deleteModel(index: number): Promise<ModelActionResponse> {
  return request<ModelActionResponse>(`/api/models/${index}`, {
    method: "DELETE",
  })
}

export async function setDefaultModel(
  modelName: string,
): Promise<ModelActionResponse> {
  const response = await request<ModelActionResponse>("/api/models/default", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ model_name: modelName }),
  })

  await refreshGatewayState()
  return response
}

export async function testModel(
  model: TestModelPayload,
): Promise<ModelTestResponse> {
  return request<ModelTestResponse>("/api/models/test", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(model),
  })
}

export async function testAllModels(
  providerKey?: string,
): Promise<ModelBatchTestResponse> {
  return request<ModelBatchTestResponse>("/api/models/test-all", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(providerKey ? { provider_key: providerKey } : {}),
  })
}

export type { ModelsListResponse, ModelActionResponse }
