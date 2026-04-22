import {
  IconFile,
  IconFilePencil,
  IconLoader2,
  IconX,
} from "@tabler/icons-react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import {
  type WorkspaceFileInfo,
  getWorkspaceFile,
  getWorkspaceFiles,
  updateWorkspaceFile,
} from "@/api/workspace-files"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Textarea } from "@/components/ui/textarea"

interface WorkspaceFilesEditorProps {
  agentId: string
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function WorkspaceFilesEditor({
  agentId,
  open,
  onOpenChange,
}: WorkspaceFilesEditorProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [selectedFile, setSelectedFile] = useState<string | null>(null)
  const [content, setContent] = useState("")
  const [saving, setSaving] = useState(false)

  const { data: filesData, isLoading: loadingFiles } = useQuery({
    queryKey: ["workspace-files", agentId],
    queryFn: () => getWorkspaceFiles(agentId),
    enabled: open,
  })

  const files = useMemo(() => filesData?.files ?? [], [filesData?.files])

  useEffect(() => {
    if (!open) {
      setSelectedFile(null)
      setContent("")
      return
    }
    if (!selectedFile && files.length > 0) {
      const firstEditable = files.find((file) => file.can_edit) ?? files[0]
      setSelectedFile(firstEditable.path)
    }
  }, [files, open, selectedFile])

  const { data: fileContent, isLoading: loadingContent } = useQuery({
    queryKey: ["workspace-file", agentId, selectedFile],
    queryFn: () => getWorkspaceFile(selectedFile!, agentId),
    enabled: open && Boolean(selectedFile),
  })

  useEffect(() => {
    if (fileContent) {
      setContent(fileContent.content)
    }
  }, [fileContent])

  const handleSave = async () => {
    if (!selectedFile) {
      return
    }
    try {
      setSaving(true)
      await updateWorkspaceFile(selectedFile, content, agentId)
      await queryClient.invalidateQueries({ queryKey: ["workspace-files", agentId] })
      await queryClient.invalidateQueries({ queryKey: ["workspace-file", agentId, selectedFile] })
      toast.success(t("pages.agent.manage.workspaceFiles.saveSuccess"))
    } catch (err) {
      toast.error(
        err instanceof Error
          ? err.message
          : t("pages.agent.manage.workspaceFiles.saveError"),
      )
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] max-w-5xl overflow-hidden">
        <DialogHeader>
          <DialogTitle>{t("pages.agent.manage.workspaceFiles.title")}</DialogTitle>
          <DialogDescription>{filesData?.workspace ?? ""}</DialogDescription>
        </DialogHeader>

        <div className="flex h-[60vh] min-h-[400px] gap-4 overflow-hidden">
          <div className="w-60 shrink-0 space-y-2 overflow-auto border-r pr-3">
            <p className="text-muted-foreground text-sm font-medium">
              {t("pages.agent.manage.workspaceFiles.files")}
            </p>
            {loadingFiles ? (
              <div className="text-muted-foreground text-sm">{t("labels.loading")}</div>
            ) : files.length === 0 ? (
              <div className="text-muted-foreground text-sm">
                {t("pages.agent.manage.workspaceFiles.empty")}
              </div>
            ) : (
              <div className="space-y-1">
                {files.map((file: WorkspaceFileInfo) => (
                  <Button
                    key={file.path}
                    variant={selectedFile === file.path ? "secondary" : "ghost"}
                    size="sm"
                    className="h-auto w-full justify-start gap-2 px-2 py-2 text-left"
                    onClick={() => setSelectedFile(file.path)}
                  >
                    {file.can_edit ? (
                      <IconFilePencil className="mt-0.5 size-4 shrink-0" />
                    ) : (
                      <IconFile className="mt-0.5 size-4 shrink-0" />
                    )}
                    <span className="min-w-0 truncate text-xs">{file.path}</span>
                  </Button>
                ))}
              </div>
            )}
          </div>

          <div className="flex min-w-0 flex-1 flex-col gap-2 overflow-hidden">
            {selectedFile ? (
              <>
                <div className="flex items-center justify-between gap-2">
                  <p className="truncate text-sm font-medium">{selectedFile}</p>
                  {loadingContent && <IconLoader2 className="size-4 animate-spin" />}
                </div>
                <Textarea
                  value={content}
                  onChange={(e) => setContent(e.target.value)}
                  className="min-h-0 flex-1 resize-none font-mono text-sm"
                  placeholder={t("pages.agent.manage.workspaceFiles.placeholder")}
                  disabled={loadingContent}
                />
              </>
            ) : (
              <div className="text-muted-foreground flex h-full items-center justify-center text-sm">
                {t("pages.agent.manage.workspaceFiles.selectFile")}
              </div>
            )}
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            <IconX className="size-4" />
            {t("common.cancel")}
          </Button>
          <Button disabled={!selectedFile || saving} onClick={() => void handleSave()}>
            {saving ? (
              <IconLoader2 className="size-4 animate-spin" />
            ) : (
              <IconFilePencil className="size-4" />
            )}
            {saving ? t("common.saving") : t("common.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
