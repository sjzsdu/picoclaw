import { useCallback, useEffect, useMemo, useState } from "react"

import { type AgentInfo, getAgents } from "@/api/agents"

const AGENT_STORAGE_KEY_PREFIX = "picoclaw:session-agent:"
const MAIN_AGENT_ID = "main"

function readStoredAgentId(sessionId: string): string {
  if (!sessionId) {
    return ""
  }
  return (
    globalThis.localStorage?.getItem(`${AGENT_STORAGE_KEY_PREFIX}${sessionId}`)
      ?.trim() || ""
  )
}

function writeStoredAgentId(sessionId: string, agentId: string) {
  if (!sessionId) {
    return
  }
  const storageKey = `${AGENT_STORAGE_KEY_PREFIX}${sessionId}`
  if (agentId) {
    globalThis.localStorage?.setItem(storageKey, agentId)
    return
  }
  globalThis.localStorage?.removeItem(storageKey)
}

function resolveSelectedAgentId(storedAgentId: string) {
  if (storedAgentId) {
    return storedAgentId
  }
  return MAIN_AGENT_ID
}

function resolveValidSelectedAgentId(
  agentIds: Set<string>,
  requestedAgentId: string,
) {
  const normalizedRequestedAgentId = requestedAgentId.trim()
  if (normalizedRequestedAgentId && agentIds.has(normalizedRequestedAgentId)) {
    return normalizedRequestedAgentId
  }
  if (agentIds.has(MAIN_AGENT_ID)) {
    return MAIN_AGENT_ID
  }
  return Array.from(agentIds)[0] ?? MAIN_AGENT_ID
}

export function useChatAgents(activeSessionId: string) {
  const [agents, setAgents] = useState<AgentInfo[]>([])
  const [sessionSelections, setSessionSelections] = useState<
    Record<string, string>
  >({})

  const loadAgents = useCallback(async () => {
    const storedAgentId = readStoredAgentId(activeSessionId)
    try {
      const data = await getAgents()
      setAgents(data.agents)

      const agentIDs = new Set(data.agents.map((agent) => agent.id))
      const nextSelectedAgentId = resolveValidSelectedAgentId(
        agentIDs,
        resolveSelectedAgentId(storedAgentId),
      )
      setSessionSelections((prev) => ({
        ...prev,
        [activeSessionId]: nextSelectedAgentId,
      }))
      writeStoredAgentId(activeSessionId, nextSelectedAgentId)
    } catch {
      setAgents([])
      setSessionSelections((prev) => ({
        ...prev,
        [activeSessionId]: resolveSelectedAgentId(storedAgentId),
      }))
    }
  }, [activeSessionId])

  useEffect(() => {
    const timerId = setTimeout(() => {
      void loadAgents()
    }, 0)

    return () => clearTimeout(timerId)
  }, [loadAgents])

  const handleSelectAgent = useCallback(
    (agentId: string) => {
      const agentIDs = new Set(agents.map((agent) => agent.id))
      const nextSelectedAgentId = resolveValidSelectedAgentId(agentIDs, agentId)
      setSessionSelections((prev) => ({
        ...prev,
        [activeSessionId]: nextSelectedAgentId,
      }))
      writeStoredAgentId(activeSessionId, nextSelectedAgentId)
    },
    [activeSessionId, agents],
  )

  const selectedAgentId = useMemo(() => {
    const agentIDs = new Set(agents.map((agent) => agent.id))
    const requestedAgentId =
      sessionSelections[activeSessionId] ??
      resolveSelectedAgentId(readStoredAgentId(activeSessionId))
    return resolveValidSelectedAgentId(agentIDs, requestedAgentId)
  }, [activeSessionId, agents, sessionSelections])

  const hasSelectableAgents = useMemo(() => agents.length > 1, [agents])

  return {
    agents,
    selectedAgentId,
    hasSelectableAgents,
    handleSelectAgent,
  }
}