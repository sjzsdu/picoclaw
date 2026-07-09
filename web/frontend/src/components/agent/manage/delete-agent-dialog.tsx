import { IconLoader2, IconTrash } from "@tabler/icons-react"
import { useTranslation } from "react-i18next"

import type { AgentConfigInfo } from "@/api/agent-configs"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"

interface DeleteAgentDialogProps {
  open: boolean
  agentPendingDelete: AgentConfigInfo | null
  isDeletePending: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: () => void
}

export function DeleteAgentDialog({
  open,
  agentPendingDelete,
  isDeletePending,
  onOpenChange,
  onConfirm,
}: DeleteAgentDialogProps) {
  const { t } = useTranslation()

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent size="sm">
        <AlertDialogHeader>
          <AlertDialogTitle>
            {t("pages.agent.manage.delete.title")}
          </AlertDialogTitle>
          <AlertDialogDescription>
            {agentPendingDelete?.delete_block_reason
              ? t("pages.agent.manage.delete.blocked")
              : t("pages.agent.manage.delete.description", {
                  name: agentPendingDelete?.name,
                })}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={isDeletePending}>
            {t("common.cancel")}
          </AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            disabled={
              isDeletePending ||
              !agentPendingDelete ||
              !agentPendingDelete.can_delete
            }
            onClick={onConfirm}
          >
            {isDeletePending ? (
              <IconLoader2 className="size-4 animate-spin" />
            ) : (
              <IconTrash className="size-4" />
            )}
            {t("pages.agent.manage.delete.confirm")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
