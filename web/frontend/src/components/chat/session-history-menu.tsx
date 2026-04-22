import { IconEdit, IconHistory, IconTrash } from "@tabler/icons-react"
import dayjs from "dayjs"
import { type RefObject, useState } from "react"
import { useTranslation } from "react-i18next"

import type { SessionSummary } from "@/api/sessions"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { ScrollArea } from "@/components/ui/scroll-area"

const SESSION_TITLE_KEY_PREFIX = "picoclaw:session-title:"

function sessionTitleKey(sessionId: string) {
  return `${SESSION_TITLE_KEY_PREFIX}${sessionId}`
}

function getCustomSessionTitle(sessionId: string) {
  return (
    globalThis.localStorage?.getItem(sessionTitleKey(sessionId))?.trim() || ""
  )
}

function setCustomSessionTitle(sessionId: string, title: string) {
  const normalized = title.trim()
  if (!normalized) {
    globalThis.localStorage?.removeItem(sessionTitleKey(sessionId))
    return
  }
  globalThis.localStorage?.setItem(sessionTitleKey(sessionId), normalized)
}

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
  onOpenChange,
  onSwitchSession,
  onDeleteSession,
}: SessionHistoryMenuProps) {
  const { t } = useTranslation()
  const [, setRefreshTick] = useState(0)

  const getAgentLabel = (agentId: string) => resolveAgentLabel?.(agentId) || agentId
  const resolveTitle = (session: SessionSummary) =>
    getCustomSessionTitle(session.id) ||
    session.title ||
    (session.agent_id ? getAgentLabel(session.agent_id) : "")

  const handleRenameSession = (session: SessionSummary) => {
    const next = globalThis.window?.prompt(
      "Rename this chat",
      resolveTitle(session),
    )
    if (next === undefined || next === null) {
      return
    }
    setCustomSessionTitle(session.id, next)
    setRefreshTick((value) => value + 1)
  }

  return (
    <DropdownMenu onOpenChange={onOpenChange}>
      <DropdownMenuTrigger asChild>
        <Button variant="secondary" size="sm" className="h-9 gap-2">
          <IconHistory className="size-4" />
          <span className="hidden sm:inline">{t("chat.history")}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-72">
        <ScrollArea className="max-h-[300px]">
          {loadError && (
            <DropdownMenuItem disabled>
              <span className="text-destructive text-xs">
                {loadErrorMessage}
              </span>
            </DropdownMenuItem>
          )}
          {sessions.length === 0 && !loadError ? (
            <DropdownMenuItem disabled>
              <span className="text-muted-foreground text-xs">
                {t("chat.noHistory")}
              </span>
            </DropdownMenuItem>
          ) : (
            sessions.map((session) => (
              <DropdownMenuItem
                key={session.id}
                className={`group relative my-0.5 flex flex-col items-start gap-0.5 pr-8 ${
                  (session.session_id || session.id) === activeSessionId
                    ? "bg-accent"
                    : ""
                }`}
                onClick={() => onSwitchSession(session)}
              >
                <span className="line-clamp-1 text-sm font-medium">
                  {resolveTitle(session)}
                </span>
                <div className="flex w-full items-center gap-2">
                  <span className="text-muted-foreground text-xs">
                    {t("chat.messagesCount", {
                      count: session.message_count,
                    })}{" "}
                    · {dayjs(session.updated).fromNow()}
                  </span>
                  {session.agent_id && resolveAgentLabel && (
                    <span className="bg-muted text-muted-foreground rounded px-1.5 py-0.5 text-xs">
                      {resolveAgentLabel(session.agent_id)}
                    </span>
                  )}
                </div>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label="Rename chat"
                  className="text-muted-foreground absolute top-1/2 right-9 h-6 w-6 -translate-y-1/2 opacity-0 transition-opacity group-hover:opacity-100"
                  onClick={(e) => {
                    e.preventDefault()
                    e.stopPropagation()
                    handleRenameSession(session)
                  }}
                >
                  <IconEdit className="h-4 w-4" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label={t("chat.deleteSession")}
                  className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive absolute top-1/2 right-2 h-6 w-6 -translate-y-1/2 opacity-0 transition-opacity group-hover:opacity-100"
                  onClick={(e) => {
                    e.preventDefault()
                    e.stopPropagation()
                    onDeleteSession(session)
                  }}
                >
                  <IconTrash className="h-4 w-4" />
                </Button>
              </DropdownMenuItem>
            ))
          )}
          {hasMore && sessions.length > 0 && (
            <div ref={observerRef} className="py-2 text-center">
              <span className="text-muted-foreground animate-pulse text-xs">
                {t("chat.loadingMore")}
              </span>
            </div>
          )}
        </ScrollArea>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
