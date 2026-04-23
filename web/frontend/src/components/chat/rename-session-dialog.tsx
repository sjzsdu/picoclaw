import { useState, useEffect } from "react"
import { useTranslation } from "react-i18next"

import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"

interface RenameSessionDialogProps {
  open: boolean
  initialTitle: string
  onOpenChange: (open: boolean) => void
  onRename: (title: string) => Promise<void>
}

export function RenameSessionDialog({
  open,
  initialTitle,
  onOpenChange,
  onRename,
}: RenameSessionDialogProps) {
  const { t } = useTranslation()
  const [value, setValue] = useState(initialTitle)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (open) {
      setValue(initialTitle)
    }
  }, [open, initialTitle])

  const handleSubmit = async () => {
    const trimmed = value.trim()
    if (!trimmed) return

    setLoading(true)
    try {
      await onRename(trimmed)
      onOpenChange(false)
    } finally {
      setLoading(false)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !loading && value.trim()) {
      void handleSubmit()
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>{t("chat.renameSession")}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <Input
            value={value}
            onChange={(e) => setValue(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={t("chat.sessionTitlePlaceholder")}
            autoFocus
            disabled={loading}
          />
          <div className="flex justify-end gap-2">
            <Button
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={loading}
            >
              {t("common.cancel")}
            </Button>
            <Button
              onClick={() => void handleSubmit()}
              disabled={loading || !value.trim()}
            >
              {loading ? t("common.saving") : t("common.save")}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}