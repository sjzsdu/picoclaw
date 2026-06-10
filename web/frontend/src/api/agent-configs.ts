import { launcherFetch } from "@/api/http"

export interface AgentConfigInfo {
  id: string
  name: string
  is_main: boolean
  is_implicit: boolean
  is_default: boolean
  model_name: string
  model_fallbacks: string[]
  effective_model_name: string
  workspace: string
  effective_workspace: string
  skills: string[]
  skills_count: number
  can_delete: boolean
  delete_block_reason?: string
  index: number
}

export interface AgentConfigListResponse {
  agents: AgentConfigInfo[]
  default_agent: string
  total: number
}

interface AgentConfigActionResponse {
  status: string
  agent?: AgentConfigInfo
}

export interface AgentConfigInput {
  id?: string
  name: string
  workspace: string
  model_name: string
  model_fallbacks: string[]
  skills: string[]
  is_default: boolean
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await launcherFetch(path, options)
  if (!res.ok) {
    let message = `API error: ${res.status} ${res.statusText}`
    try {
      const body = (await res.json()) as {
        error?: string
        errors?: string[]
      }
      if (Array.isArray(body.errors) && body.errors.length > 0) {
        message = body.errors.join("; ")
      } else if (typeof body.error === "string" && body.error.trim() !== "") {
        message = body.error
      }
    } catch {
      const text = await res.text().catch(() => "")
      if (text.trim()) {
        message = text.trim()
      }
    }
    throw new Error(message)
  }
  if (res.status === 204) {
    return undefined as T
  }
  return res.json() as Promise<T>
}

export async function getAgentConfigs(): Promise<AgentConfigListResponse> {
  return request<AgentConfigListResponse>("/api/agent-configs")
}

export async function createAgentConfig(
  input: AgentConfigInput,
): Promise<AgentConfigActionResponse> {
  return request<AgentConfigActionResponse>("/api/agent-configs", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  })
}

export async function updateAgentConfig(
  id: string,
  input: AgentConfigInput,
): Promise<AgentConfigActionResponse> {
  return request<AgentConfigActionResponse>(
    `/api/agent-configs/${encodeURIComponent(id)}`,
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    },
  )
}

export async function deleteAgentConfig(id: string): Promise<void> {
  await request<void>(`/api/agent-configs/${encodeURIComponent(id)}`, {
    method: "DELETE",
  })
}
