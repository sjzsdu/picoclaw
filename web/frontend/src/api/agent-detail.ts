import type { AgentConfigInfo } from "@/api/agent-configs"
import { launcherFetch } from "@/api/http"
import type { SessionSummary } from "@/api/sessions"

export interface AgentDetailFile {
  name: string
  path: string
  exists: boolean
  size: number
  modified_at?: string
  content?: string
  truncated?: boolean
  can_read: boolean
}

export interface AgentDetailMemory {
  long_term: AgentDetailFile
  recent_daily: AgentDetailFile[]
}

export interface AgentDetailSkill {
  name: string
  configured: boolean
  resolved: boolean
  source?: string
  path?: string
  description?: string
}

export interface AgentDetailCronJob {
  id: string
  name: string
  enabled: boolean
  schedule: {
    kind: string
    atMs?: number
    everyMs?: number
    expr?: string
    tz?: string
  }
  payload: {
    kind: string
    message: string
    command?: string
    channel?: string
    to?: string
  }
  state?: {
    nextRunAtMs?: number
    lastRunAtMs?: number
    lastStatus?: string
    lastError?: string
  }
  createdAtMs: number
  updatedAtMs: number
  deleteAfterRun?: boolean
}

export interface AgentDetailCron {
  file: AgentDetailFile
  jobs?: AgentDetailCronJob[]
  count: number
}

export interface AgentDetailDirectories {
  memory_exists: boolean
  skills_exists: boolean
  sessions_exists: boolean
  cron_exists: boolean
  state_exists: boolean
}

export interface AgentDetailResponse {
  agent: AgentConfigInfo
  workspace: string
  prompt_files: AgentDetailFile[]
  memory: AgentDetailMemory
  skills: AgentDetailSkill[]
  sessions: SessionSummary[]
  cron: AgentDetailCron
  state_files: AgentDetailFile[]
  directories: AgentDetailDirectories
}

export async function getAgentDetail(
  agentId: string,
): Promise<AgentDetailResponse> {
  const res = await launcherFetch(
    `/api/agents/${encodeURIComponent(agentId)}/detail`,
  )
  if (!res.ok) {
    throw new Error(`Failed to fetch agent detail: ${res.status}`)
  }
  return res.json()
}
