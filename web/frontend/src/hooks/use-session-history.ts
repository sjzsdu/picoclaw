import { useCallback, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import {
  type SessionSummary,
  deleteSession,
  getSessions,
  updateSessionTitle,
} from "@/api/sessions"
import {
  CHAT_SESSION_ACTIVITY_EVENT,
  type ChatSessionActivityDetail,
} from "@/features/chat/protocol"

const LIMIT = 20
const OPTIMISTIC_SESSION_TTL_MS = 30_000

type OptimisticSession = SessionSummary & { optimisticUntil: number }

interface UseSessionHistoryOptions {
  activeSessionId: string
  onDeletedActiveSession: () => void
}

export function useSessionHistory({
  activeSessionId,
  onDeletedActiveSession,
}: UseSessionHistoryOptions) {
  const { t } = useTranslation()
  const observerRef = useRef<HTMLDivElement>(null)
  const [sessions, setSessions] = useState<SessionSummary[]>([])
  const [offset, setOffset] = useState(0)
  const [hasMore, setHasMore] = useState(true)
  const [isLoadingMore, setIsLoadingMore] = useState(false)
  const [loadError, setLoadError] = useState(false)
  const optimisticSessionsRef = useRef<Map<string, OptimisticSession>>(
    new Map(),
  )

  const mergeOptimisticSessions = useCallback((data: SessionSummary[]) => {
    const now = Date.now()
    const dataIds = new Set(data.map((session) => session.id))
    const pending: SessionSummary[] = []
    for (const [id, session] of optimisticSessionsRef.current) {
      if (session.optimisticUntil <= now || dataIds.has(id)) {
        optimisticSessionsRef.current.delete(id)
        continue
      }
      const summary: SessionSummary = {
        id: session.id,
        title: session.title,
        preview: session.preview,
        message_count: session.message_count,
        created: session.created,
        updated: session.updated,
        session_id: session.session_id,
        agent_id: session.agent_id,
      }
      pending.push(summary)
    }
    return [...pending, ...data]
  }, [])

  const applyOptimisticSessionActivity = useCallback(
    (detail: ChatSessionActivityDetail) => {
      const sessionId = detail.sessionId.trim()
      if (!sessionId) {
        return
      }
      const updated = detail.timestamp ?? new Date().toISOString()
      const preview = (detail.preview?.trim() || "(new chat)").slice(0, 60)

      setSessions((prev) => {
        const existing = prev.find((session) => session.id === sessionId)
        const rest = prev.filter((session) => session.id !== sessionId)
        const next: SessionSummary = existing
          ? {
              ...existing,
              preview: existing.preview || preview,
              title:
                existing.title && existing.title !== "(empty)"
                  ? existing.title
                  : preview,
              updated,
            }
          : {
              id: sessionId,
              title: preview,
              preview,
              message_count: 1,
              created: updated,
              updated,
            }
        optimisticSessionsRef.current.set(sessionId, {
          ...next,
          optimisticUntil: Date.now() + OPTIMISTIC_SESSION_TTL_MS,
        })
        return [next, ...rest]
      })
    },
    [],
  )

  const loadSessions = useCallback(
    async (reset = true) => {
      try {
        const currentOffset = reset ? 0 : offset
        if (reset) {
          setLoadError(false)
          setHasMore(true)
          setOffset(0)
        }

        const data = await getSessions(currentOffset, LIMIT)
        setLoadError(false)

        if (data.length < LIMIT) {
          setHasMore(false)
        }

        if (reset) {
          setSessions(mergeOptimisticSessions(data))
        } else {
          setSessions((prev) => {
            const existingIds = new Set(prev.map((s) => s.id))
            const newItems = data.filter((s) => !existingIds.has(s.id))
            return [...prev, ...newItems]
          })
        }

        setOffset(currentOffset + data.length)
      } catch (err) {
        console.error("Failed to fetch session history:", err)
        setLoadError(true)
        if (!reset) {
          setHasMore(false)
        }
      } finally {
        setIsLoadingMore(false)
      }
    },
    [mergeOptimisticSessions, offset],
  )

  useEffect(() => {
    void loadSessions(true)
  }, [])

  useEffect(() => {
    if (!observerRef.current || !hasMore || isLoadingMore || loadError) return

    const observer = new IntersectionObserver(
      (entries) => {
        if (
          entries[0].isIntersecting &&
          hasMore &&
          !isLoadingMore &&
          !loadError
        ) {
          setIsLoadingMore(true)
          void loadSessions(false)
        }
      },
      { threshold: 0.1 },
    )

    observer.observe(observerRef.current)
    return () => observer.disconnect()
  }, [hasMore, isLoadingMore, loadError, loadSessions])

  useEffect(() => {
    let refreshTimer: number | null = null

    const handleSessionActivity = (event: Event) => {
      const detail = (event as CustomEvent<ChatSessionActivityDetail>).detail
      if (detail?.sessionId) {
        applyOptimisticSessionActivity(detail)
      }
      if (refreshTimer !== null) {
        window.clearTimeout(refreshTimer)
      }
      refreshTimer = window.setTimeout(() => {
        void loadSessions(true)
      }, 300)
    }

    window.addEventListener(
      CHAT_SESSION_ACTIVITY_EVENT,
      handleSessionActivity as EventListener,
    )

    return () => {
      if (refreshTimer !== null) {
        window.clearTimeout(refreshTimer)
      }
      window.removeEventListener(
        CHAT_SESSION_ACTIVITY_EVENT,
        handleSessionActivity as EventListener,
      )
    }
  }, [applyOptimisticSessionActivity, loadSessions])

  const handleDeleteSession = useCallback(
    async (session: SessionSummary) => {
      const id = session.id
      try {
        const deletedLoadedSession = sessions.some((s) => s.id === id)
        await deleteSession(id)
        setSessions((prev) => prev.filter((s) => s.id !== id))
        if (deletedLoadedSession) {
          setOffset((prev) => Math.max(prev - 1, 0))
        }
        if (id === activeSessionId) {
          onDeletedActiveSession()
        }
      } catch (err) {
        console.error("Failed to delete session:", err)
      }
    },
    [activeSessionId, onDeletedActiveSession, sessions],
  )

  const handleRenameSession = useCallback(
    async (session: SessionSummary, title: string) => {
      const normalized = title.trim()
      const controller = new AbortController()
      const timeout = setTimeout(() => controller.abort(), 8000)
      try {
        const result = await updateSessionTitle(
          session.id,
          normalized,
          controller.signal,
        )
        clearTimeout(timeout)
        setSessions((prev) =>
          prev.map((item) =>
            item.id === session.id ? { ...item, title: result.title } : item,
          ),
        )
      } catch (err) {
        clearTimeout(timeout)
        // Don't show toast for abort (timeout) - the dialog will handle it via loading reset
        if (err instanceof Error && err.name !== "AbortError") {
          console.error("Failed to rename session:", err)
          toast.error(
            t("chat.renameFailed", {
              defaultValue: "Failed to rename session",
            }),
          )
        }
      }
    },
    [t],
  )

  return {
    sessions,
    hasMore,
    loadError,
    loadErrorMessage: t("chat.historyLoadFailed"),
    observerRef,
    loadSessions,
    handleDeleteSession,
    handleRenameSession,
  }
}
