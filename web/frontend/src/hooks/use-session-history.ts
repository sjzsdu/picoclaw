import { useCallback, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"

import { type SessionSummary, deleteSession, getSessions } from "@/api/sessions"
import {
  CHAT_SESSION_ACTIVITY_EVENT,
  type ChatSessionActivityDetail,
} from "@/features/chat/protocol"

const LIMIT = 20

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
          setSessions(data)
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
    [offset],
  )

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
        const deletedLoadedSession = sessions.some(
          (s) => s.id === id,
        )
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

  return {
    sessions,
    hasMore,
    loadError,
    loadErrorMessage: t("chat.historyLoadFailed"),
    observerRef,
    loadSessions,
    handleDeleteSession,
  }
}
