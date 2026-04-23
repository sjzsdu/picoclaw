import { IconEdit, IconHistory, IconTrash } from "@tabler/icons-react"
import dayjs from "dayjs"
import relativeTime from "dayjs/plugin/relativeTime"
import { useState } from "react"
import { useTranslation } from "react-i18next"

import type { SessionSummary } from "@/api/sessions"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"

dayjs.extend(relativeTime)

interface SessionHistoryDropdownProps {
  sessions: SessionSummary[]
  activeSessionId: string
  onSwitchSession: (session: SessionSummary) => void
  onRenameSession: (session: SessionSummary, title: string) => Promise<void>
  onDeleteSession: (session: SessionSummary) => void
}

export function SessionHistoryDropdown({
  sessions,
  activeSessionId,
  onSwitchSession,
  onRenameSession,
  onDeleteSession,
}: SessionHistoryDropdownProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)

  const activeSession = sessions.find(
    (s) => (s.session_id || s.id) === activeSessionId
  )

  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger asChild>
        <Button variant="secondary" size="sm" className="h-9 gap-2">
          <IconHistory className="size-4" />
          <span className="hidden sm:inline">
            {activeSession?.title || t("chat.history")}
          </span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="start"
        className="max-h-80 w-72 overflow-y-auto"
      >
        {sessions.length === 0 ? (
          <DropdownMenuItem className="text-muted-foreground" disabled>
            {t("chat.noHistory")}
          </DropdownMenuItem>
        ) : (
          <>
            {sessions.map((session) => (
              <DropdownMenuItem
                key={session.id}
                className="flex w-full cursor-pointer items-center justify-between gap-2 p-2"
                onClick={() => {
                  onSwitchSession(session)
                  setOpen(false)
                }}
              >
                <div className="flex min-w-0 flex-1 flex-col items-start gap-0.5">
                  <span className="line-clamp-1 text-sm font-medium">
                    {session.title || "(empty)"}
                  </span>
                  <span className="text-muted-foreground text-xs">
                    {session.message_count} · {dayjs(session.updated).fromNow()}
                  </span>
                </div>
                <div className="flex items-center gap-1">
                  <RenameDialog
                    session={session}
                    onRename={onRenameSession}
                  />
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-6 w-6 p-0 text-destructive hover:text-destructive"
                    onClick={(e) => {
                      e.stopPropagation()
                      if (
                        window.confirm(
                          t("chat.confirmDelete", {
                            defaultValue: "Delete this chat?",
                          })
                        )
                      ) {
                        onDeleteSession(session)
                      }
                    }}
                  >
                    <IconTrash className="size-3" />
                  </Button>
                </div>
              </DropdownMenuItem>
            ))}
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function RenameDialog({
  session,
  onRename,
}: {
  session: SessionSummary
  onRename: (session: SessionSummary, title: string) => Promise<void>
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [title, setTitle] = useState(session.title || "")
  const [loading, setLoading] = useState(false)

  const handleRename = async () => {
    if (!title.trim()) return
    setLoading(true)
    try {
      await onRename(session, title.trim())
      setOpen(false)
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button
          variant="ghost"
          size="sm"
          className="h-6 w-6 p-0"
          onClick={(e) => e.stopPropagation()}
        >
          <IconEdit className="size-3" />
        </Button>
      </DialogTrigger>
      <DialogContent className="w-80">
        <DialogHeader>
          <DialogTitle>{t("chat.renameSession")}</DialogTitle>
          <DialogDescription>
            {t("chat.renameSessionDesc") || "Enter a new name for this chat"}
          </DialogDescription>
        </DialogHeader>
        <Input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder={t("chat.sessionTitlePlaceholder") || "Chat title"}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              void handleRename()
            }
          }}
        />
        <div className="flex justify-end gap-2">
          <Button variant="outline" onClick={() => setOpen(false)}>
            {t("common.cancel") || "Cancel"}
          </Button>
          <Button onClick={handleRename} disabled={loading || !title.trim()}>
            {loading ? t("common.saving") || "Saving..." : t("common.save") || "Save"}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}