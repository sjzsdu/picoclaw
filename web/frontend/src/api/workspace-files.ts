import { launcherFetch } from "@/api/http"

export interface WorkspaceFileInfo {
  name: string
  path: string
  exists: boolean
  size: number
  modified_at: string
  can_edit: boolean
}

export interface WorkspaceFileContent {
  name: string
  path: string
  content: string
  exists: boolean
}

export interface WorkspaceFileListResponse {
  agent_id: string
  workspace: string
  files: WorkspaceFileInfo[]
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
  return res.json() as Promise<T>
}

export async function getWorkspaceFiles(
  agentId?: string,
): Promise<WorkspaceFileListResponse> {
  const params = agentId ? `?agent_id=${encodeURIComponent(agentId)}` : ""
  return request<WorkspaceFileListResponse>(
    `/api/agent-workspace-files${params}`,
  )
}

export async function getWorkspaceFile(
  filename: string,
  agentId?: string,
): Promise<WorkspaceFileContent> {
  const params = agentId ? `?agent_id=${encodeURIComponent(agentId)}` : ""
  return request<WorkspaceFileContent>(
    `/api/agent-workspace-files/${encodeURIComponent(filename)}${params}`,
  )
}

export async function updateWorkspaceFile(
  filename: string,
  content: string,
  agentId?: string,
): Promise<{ status: string; file: WorkspaceFileContent }> {
  const params = agentId ? `?agent_id=${encodeURIComponent(agentId)}` : ""
  return request<{ status: string; file: WorkspaceFileContent }>(
    `/api/agent-workspace-files/${encodeURIComponent(filename)}${params}`,
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ content }),
    },
  )
}
