import { IconEdit, IconTrash } from "@tabler/icons-react"
import dayjs from "dayjs"
import { type RefObject } from "react"
import { useTranslation } from "react-i18next"

import type { SessionSummary } from "@/api/sessions"
import { Button } from "@/components/ui/button"

interface SessionHistoryMenuProps {
  sessions: SessionSummary[]
  activeSessionId: string
  hasMore: boolean
  loadError: boolean
  loadErrorMessage: string
  observerRef: RefObject<HTMLDivElement | null>
  resolveAgentLabel?: (agentId: string) => string
  onOpenChange: (open: boolean) => void
  onSwitchSession: (session: SessionSummary) => void
  onRenameSession: (session: SessionSummary, title: string) => Promise<void>
  onDeleteSession: (session: SessionSummary) => void
}

export function SessionHistoryMenu({
  sessions,
  activeSessionId,
  hasMore,
  loadError,
  loadErrorMessage,
  observerRef,
  resolveAgentLabel,
  onSwitchSession,
  onRenameSession,
  onDeleteSession,
}: SessionHistoryMenuProps) {
  const { t } = useTranslation()

  const getAgentLabel = (agentId: string) =>
    resolveAgentLabel?.(agentId) || agentId
  const resolveTitle = (session: SessionSummary) =>
    session.title || (session.agent_id ? getAgentLabel(session.agent_id) : "")

  if (loadError) {
    return <span className="text-destructive text-xs">{loadErrorMessage}</span>
  }

  return (
    <div className="flex items-center gap-1">
      {sessions.map((session) => (
        <div
          key={session.id}
          className={`group hover:bg-accent flex items-center gap-2 rounded-md px-2 py-1 ${
            (session.session_id || session.id) === activeSessionId
              ? "bg-accent"
              : ""
          }`}
        >
          <button
            className="flex min-w-0 flex-1 cursor-pointer flex-col items-start gap-0.5"
            onClick={() => onSwitchSession(session)}
          >
            <span className="line-clamp-1 text-xs font-medium">
              {resolveTitle(session)}
            </span>
            <div className="flex items-center gap-1">
              <span className="text-muted-foreground text-[10px]">
                {session.message_count} · {dayjs(session.updated).fromNow()}
              </span>
              {session.agent_id && resolveAgentLabel && (
                <span className="bg-muted text-muted-foreground rounded px-1 py-0 text-[10px]">
                  {resolveAgentLabel(session.agent_id)}
                </span>
              )}
            </div>
          </button>
          <div className="flex items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
            <Button
              variant="ghost"
              size="sm"
              className="h-5 w-5 p-0"
              onClick={(e) => {
                e.stopPropagation()
                const newTitle = window.prompt(
                  t("chat.renameSession"),
                  resolveTitle(session),
                )
                if (newTitle && newTitle.trim()) {
                  void onRenameSession(session, newTitle.trim())
                }
              }}
            >
              <IconEdit className="size-2.5" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              className="text-destructive hover:text-destructive h-5 w-5 p-0"
              onClick={(e) => {
                e.stopPropagation()
                onDeleteSession(session)
              }}
            >
              <IconTrash className="size-2.5" />
            </Button>
          </div>
        </div>
      ))}
      {hasMore && (
        <div ref={observerRef} className="px-1">
          <span className="text-muted-foreground animate-pulse text-[10px]">
            {t("chat.loadingMore")}
          </span>
        </div>
      )}
    </div>
  )
}
