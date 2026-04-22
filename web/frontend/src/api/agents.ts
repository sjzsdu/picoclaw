import { launcherFetch } from "./http"

export interface AgentInfo {
  id: string
  name: string
  description?: string
}

interface GetAgentsResponse {
  agents: AgentInfo[]
}

export async function getAgents(): Promise<GetAgentsResponse> {
  const res = await launcherFetch("/api/agents")
  if (!res.ok) {
    throw new Error(`Failed to fetch agents: ${res.status}`)
  }
  return res.json()
}