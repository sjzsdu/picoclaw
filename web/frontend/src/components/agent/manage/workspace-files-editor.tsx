import {
  IconDeviceFloppy,
  IconFile,
  IconFilePencil,
  IconFolder,
  IconLoader2,
  IconSearch,
  IconX,
} from "@tabler/icons-react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { useEffect, useMemo, useState } from "react"
import MDEditor from "@uiw/react-md-editor"
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
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { useTheme } from "@/hooks/use-theme"
import { cn } from "@/lib/utils"

import "@uiw/react-md-editor/markdown-editor.css"

interface WorkspaceFilesEditorProps {
  agentId: string
  open: boolean
  onOpenChange: (open: boolean) => void
}

function isMarkdownFile(path: string) {
  const normalized = path.trim().toLowerCase()
  return normalized.endsWith(".md") || normalized.endsWith(".markdown")
}

export function WorkspaceFilesEditor({
  agentId,
  open,
  onOpenChange,
}: WorkspaceFilesEditorProps) {
  const { t } = useTranslation()
  const { theme } = useTheme()
  const queryClient = useQueryClient()
  const [selectedFile, setSelectedFile] = useState<string | null>(null)
  const [drafts, setDrafts] = useState<Record<string, string>>({})
  const [lastSavedContent, setLastSavedContent] = useState<Record<string, string>>(
    {},
  )
  const [saving, setSaving] = useState(false)
  const [searchQuery, setSearchQuery] = useState("")

  const { data: filesData, isLoading: loadingFiles } = useQuery({
    queryKey: ["workspace-files", agentId],
    queryFn: () => getWorkspaceFiles(agentId),
    enabled: open,
  })

  const files = useMemo(() => filesData?.files ?? [], [filesData?.files])

  useEffect(() => {
    if (!open) {
      setSelectedFile(null)
      setDrafts({})
      setLastSavedContent({})
      setSearchQuery("")
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
    if (selectedFile && fileContent) {
      setDrafts((prev) =>
        prev[selectedFile] === undefined
          ? { ...prev, [selectedFile]: fileContent.content }
          : prev,
      )
      setLastSavedContent((prev) =>
        prev[selectedFile] === fileContent.content
          ? prev
          : { ...prev, [selectedFile]: fileContent.content },
      )
    }
  }, [fileContent, selectedFile])

  const filteredFiles = useMemo(() => {
    const query = searchQuery.trim().toLowerCase()
    if (!query) {
      return files
    }
    return files.filter((file) => file.path.toLowerCase().includes(query))
  }, [files, searchQuery])

  const selectedFileInfo = useMemo(
    () => files.find((file) => file.path === selectedFile) ?? null,
    [files, selectedFile],
  )

  const content = selectedFile ? (drafts[selectedFile] ?? "") : ""
  const selectedFileSavedContent = selectedFile
    ? (lastSavedContent[selectedFile] ?? "")
    : ""
  const isSelectedFileEditable = selectedFileInfo?.can_edit ?? false
  const hasUnsavedChanges =
    Boolean(selectedFile) && content !== selectedFileSavedContent
  const isMarkdownDocument = Boolean(selectedFile && isMarkdownFile(selectedFile))

  const workspaceLabel = filesData?.workspace?.trim() ?? ""

  const handleContentChange = (value: string) => {
    if (!selectedFile) {
      return
    }
    setDrafts((prev) => ({ ...prev, [selectedFile]: value }))
  }

  const handleSave = async () => {
    if (!selectedFile || !isSelectedFileEditable) {
      return
    }
    try {
      setSaving(true)
      await updateWorkspaceFile(selectedFile, content, agentId)
      setLastSavedContent((prev) => ({ ...prev, [selectedFile]: content }))
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

  const fileCountLabel = t("pages.agent.manage.workspaceFiles.fileCount", {
    count: filteredFiles.length,
  })

  const selectedFileName = selectedFileInfo
    ? selectedFileInfo.path.split("/").pop() || selectedFileInfo.path
    : ""
  const selectedFileDir =
    selectedFileInfo && selectedFileInfo.path.includes("/")
      ? selectedFileInfo.path.slice(
          0,
          selectedFileInfo.path.length - selectedFileName.length - 1,
        )
      : ""

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[92vh] max-w-6xl gap-0 overflow-hidden p-0">
        <DialogHeader className="border-border/70 bg-muted/30 border-b px-6 py-5">
          <DialogTitle className="flex items-center gap-2 text-base">
            <IconFolder className="size-4" />
            {t("pages.agent.manage.workspaceFiles.title")}
          </DialogTitle>
          <DialogDescription className="space-y-1">
            <span className="block">{workspaceLabel}</span>
            <span className="text-xs">{fileCountLabel}</span>
          </DialogDescription>
        </DialogHeader>

        <div className="grid h-[74vh] min-h-[500px] overflow-hidden lg:grid-cols-[240px_minmax(0,1fr)]">
          <aside className="bg-muted/20 border-border/70 flex min-h-0 flex-col border-b lg:border-r lg:border-b-0">
            <div className="border-border/60 space-y-3 border-b px-4 py-4">
              <p className="text-foreground text-sm font-semibold">
                {t("pages.agent.manage.workspaceFiles.files")}
              </p>
              <div className="relative">
                <IconSearch className="text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2" />
                <Input
                  value={searchQuery}
                  onChange={(event) => setSearchQuery(event.target.value)}
                  placeholder={t("pages.agent.manage.workspaceFiles.searchPlaceholder")}
                  className="bg-background pl-9"
                />
              </div>
            </div>

            <div className="min-h-0 flex-1 overflow-y-auto px-3 py-3">
              {loadingFiles ? (
                <div className="text-muted-foreground px-2 py-3 text-sm">
                  {t("labels.loading")}
                </div>
              ) : filteredFiles.length === 0 ? (
                <div className="text-muted-foreground px-2 py-3 text-sm">
                  {files.length === 0
                    ? t("pages.agent.manage.workspaceFiles.empty")
                    : t("pages.agent.manage.workspaceFiles.noMatches")}
                </div>
              ) : (
                <div className="space-y-1.5">
                  {filteredFiles.map((file: WorkspaceFileInfo) => {
                    const fileName = file.path.split("/").pop() || file.path
                    const fileDir = file.path.includes("/")
                      ? file.path.slice(0, file.path.length - fileName.length - 1)
                      : ""
                    const isActive = selectedFile === file.path

                    return (
                      <button
                        key={file.path}
                        type="button"
                        className={cn(
                          "w-full rounded-xl border px-3 py-3 text-left transition",
                          isActive
                            ? "border-primary/35 bg-primary/8 shadow-sm"
                            : "border-transparent bg-background hover:border-border hover:bg-accent/40",
                        )}
                        onClick={() => setSelectedFile(file.path)}
                      >
                        <div className="flex items-start gap-3">
                          <div
                            className={cn(
                              "mt-0.5 rounded-lg p-1.5",
                              isActive ? "bg-primary/12 text-primary" : "bg-muted text-muted-foreground",
                            )}
                          >
                            {file.can_edit ? (
                              <IconFilePencil className="size-4" />
                            ) : (
                              <IconFile className="size-4" />
                            )}
                          </div>
                          <div className="min-w-0 flex-1">
                            <div className="flex items-center gap-2">
                              <p className="truncate text-sm font-medium">{fileName}</p>
                              <span
                                className={cn(
                                  "rounded-full px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide",
                                  file.can_edit
                                    ? "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300"
                                    : "bg-zinc-500/10 text-zinc-700 dark:text-zinc-300",
                                )}
                              >
                                {file.can_edit
                                  ? t("pages.agent.manage.workspaceFiles.editable")
                                  : t("pages.agent.manage.workspaceFiles.readOnly")}
                              </span>
                            </div>
                            <p className="text-muted-foreground mt-1 truncate text-xs">
                              {fileDir || file.path}
                            </p>
                          </div>
                        </div>
                      </button>
                    )
                  })}
                </div>
              )}
            </div>
          </aside>

          <section className="bg-background flex min-h-0 flex-col">
            {selectedFile ? (
              <>
                <div className="border-border/70 bg-background/90 border-b px-5 py-4 backdrop-blur">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <p className="truncate text-sm font-semibold">{selectedFileName}</p>
                        <span
                          className={cn(
                            "rounded-full px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide",
                            isSelectedFileEditable
                              ? "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300"
                              : "bg-zinc-500/10 text-zinc-700 dark:text-zinc-300",
                          )}
                        >
                          {isSelectedFileEditable
                            ? t("pages.agent.manage.workspaceFiles.editable")
                            : t("pages.agent.manage.workspaceFiles.readOnly")}
                        </span>
                        {hasUnsavedChanges && (
                          <span className="rounded-full bg-amber-500/12 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-amber-700 dark:text-amber-300">
                            {t("pages.agent.manage.workspaceFiles.unsaved")}
                          </span>
                        )}
                      </div>
                      <p className="text-muted-foreground mt-1 truncate text-xs">
                        {selectedFileDir || selectedFile}
                      </p>
                    </div>

                    <div className="text-muted-foreground flex items-center gap-3 text-xs">
                      <span>
                        {t("pages.agent.manage.workspaceFiles.characterCount", {
                          count: content.length,
                        })}
                      </span>
                      {loadingContent && <IconLoader2 className="size-4 animate-spin" />}
                    </div>
                  </div>

                  <p className="text-muted-foreground mt-3 text-xs">
                    {isSelectedFileEditable
                      ? t("pages.agent.manage.workspaceFiles.editHint")
                      : t("pages.agent.manage.workspaceFiles.readOnlyHint")}
                  </p>
                </div>

                <div className="min-h-0 flex-1 p-4 lg:p-5">
                  <div className="bg-muted/15 border-border/70 flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border shadow-inner">
                    {isMarkdownDocument ? (
                      <div
                        data-color-mode={theme}
                        className="workspace-md-editor h-full min-h-0 flex-1 overflow-hidden"
                      >
                        <MDEditor
                          value={content}
                          onChange={(value) => handleContentChange(value ?? "")}
                          preview={isSelectedFileEditable ? "edit" : "preview"}
                          hideToolbar={!isSelectedFileEditable}
                          visibleDragbar={false}
                          textareaProps={{
                            placeholder: t("pages.agent.manage.workspaceFiles.placeholder"),
                            disabled: loadingContent,
                            readOnly: !isSelectedFileEditable,
                          }}
                          height="100%"
                        />
                      </div>
                    ) : (
                      <Textarea
                        value={content}
                        onChange={(e) => handleContentChange(e.target.value)}
                        className="h-full min-h-0 flex-1 resize-none border-0 bg-transparent px-4 py-4 font-mono text-sm leading-6 shadow-none focus-visible:ring-0"
                        placeholder={t("pages.agent.manage.workspaceFiles.placeholder")}
                        disabled={loadingContent}
                        readOnly={!isSelectedFileEditable}
                      />
                    )}
                  </div>
                </div>
              </>
            ) : (
              <div className="text-muted-foreground flex h-full items-center justify-center px-6 text-sm">
                {t("pages.agent.manage.workspaceFiles.selectFile")}
              </div>
            )}
          </section>
        </div>

        <DialogFooter className="border-border/70 bg-muted/20 border-t px-6 py-4">
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            <IconX className="size-4" />
            {t("common.cancel")}
          </Button>
          <Button
            disabled={!selectedFile || !isSelectedFileEditable || !hasUnsavedChanges || saving}
            onClick={() => void handleSave()}
          >
            {saving ? (
              <IconLoader2 className="size-4 animate-spin" />
            ) : (
              <IconDeviceFloppy className="size-4" />
            )}
            {saving ? t("common.saving") : t("common.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
