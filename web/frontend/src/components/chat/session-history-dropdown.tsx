import { IconEdit, IconHistory, IconTrash } from "@tabler/icons-react"
import dayjs from "dayjs"
import relativeTime from "dayjs/plugin/relativeTime"
import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"

import type { SessionSummary } from "@/api/sessions"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
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
  const [renameSession, setRenameSession] = useState<SessionSummary | null>(
    null,
  )

  const activeSession = sessions.find(
    (s) => (s.session_id || s.id) === activeSessionId,
  )

  return (
    <>
      <DropdownMenu open={open} onOpenChange={setOpen}>
        <DropdownMenuTrigger asChild>
          <Button variant="secondary" size="sm" className="h-9 min-w-0 gap-2">
            <IconHistory className="size-4 shrink-0" />
            <span
              className="hidden max-w-40 truncate sm:inline md:max-w-56 lg:max-w-72"
              title={activeSession?.title || t("chat.history")}
            >
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
                      {session.message_count} ·{" "}
                      {dayjs(session.updated).fromNow()}
                    </span>
                  </div>
                  <div className="flex items-center gap-1">
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-6 w-6 p-0"
                      onClick={(e) => {
                        e.stopPropagation()
                        setRenameSession(session)
                        setOpen(false)
                      }}
                    >
                      <IconEdit className="size-3" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="text-destructive hover:text-destructive h-6 w-6 p-0"
                      onClick={(e) => {
                        e.stopPropagation()
                        onDeleteSession(session)
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
      <RenameDialog
        session={renameSession}
        open={renameSession !== null}
        onOpenChange={(nextOpen) => {
          if (!nextOpen) {
            setRenameSession(null)
          }
        }}
        onRename={onRenameSession}
      />
    </>
  )
}

function RenameDialog({
  session,
  open,
  onOpenChange,
  onRename,
}: {
  session: SessionSummary | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onRename: (session: SessionSummary, title: string) => Promise<void>
}) {
  const { t } = useTranslation()
  const [title, setTitle] = useState("")
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (open) {
      setTitle(session?.title || "")
    }
  }, [open, session])

  const handleRename = async () => {
    if (!session || !title.trim()) return
    setLoading(true)
    try {
      await onRename(session, title.trim())
      onOpenChange(false)
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
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
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("common.cancel") || "Cancel"}
          </Button>
          <Button onClick={handleRename} disabled={loading || !title.trim()}>
            {loading
              ? t("common.saving") || "Saving..."
              : t("common.save") || "Save"}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
