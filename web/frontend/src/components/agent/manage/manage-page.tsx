import {
  IconDeviceFloppy,
  IconEdit,
  IconFolderOpen,
  IconInfoCircle,
  IconLoader2,
  IconPlus,
  IconTrash,
} from "@tabler/icons-react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import {
  type AgentConfigInfo,
  type AgentConfigInput,
  createAgentConfig,
  deleteAgentConfig,
  getAgentConfigs,
  updateAgentConfig,
} from "@/api/agent-configs"
import { DeleteAgentDialog } from "@/components/agent/manage/delete-agent-dialog"
import { WorkspaceFilesEditor } from "@/components/agent/manage/workspace-files-editor"
import { PageHeader } from "@/components/page-header"
import { Field } from "@/components/shared-form"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"

interface AgentFormState {
  id: string
  name: string
  workspace: string
  modelName: string
  modelFallbacksText: string
  skillsText: string
}

const EMPTY_FORM: AgentFormState = {
  id: "",
  name: "",
  workspace: "",
  modelName: "",
  modelFallbacksText: "",
  skillsText: "",
}

function listToText(values: string[]) {
  return values.join("\n")
}

function parseList(text: string) {
  return text
    .split(/\n|,/g)
    .map((item) => item.trim())
    .filter(Boolean)
}

function buildForm(agent: AgentConfigInfo): AgentFormState {
  return {
    id: agent.id,
    name: agent.name,
    workspace: agent.workspace,
    modelName: agent.model_name,
    modelFallbacksText: listToText(agent.model_fallbacks ?? []),
    skillsText: listToText(agent.skills ?? []),
  }
}

function isEditableAgent(agent: AgentConfigInfo) {
  return !agent.is_implicit || agent.is_main
}

function buildPayload(form: AgentFormState): AgentConfigInput {
  return {
    id: form.id.trim(),
    name: form.name.trim(),
    workspace: form.workspace.trim(),
    model_name: form.modelName.trim(),
    model_fallbacks: parseList(form.modelFallbacksText),
    skills: parseList(form.skillsText),
    is_default: false,
  }
}

export function ManagePage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [editorOpen, setEditorOpen] = useState(false)
  const [editingAgent, setEditingAgent] = useState<AgentConfigInfo | null>(null)
  const [isCreating, setIsCreating] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [agentPendingDelete, setAgentPendingDelete] =
    useState<AgentConfigInfo | null>(null)
  const [isDeletePending, setIsDeletePending] = useState(false)
  const [workspaceEditorAgentId, setWorkspaceEditorAgentId] = useState<
    string | null
  >(null)
  const [form, setForm] = useState<AgentFormState>(EMPTY_FORM)
  const [saving, setSaving] = useState(false)

  const { data, isLoading, error } = useQuery({
    queryKey: ["agent-configs"],
    queryFn: getAgentConfigs,
    staleTime: 30 * 1000,
  })

  const agents = useMemo(() => data?.agents ?? [], [data?.agents])

  const invalidateAgents = async () => {
    await queryClient.invalidateQueries({ queryKey: ["agent-configs"] })
  }

  const handleFieldChange = <K extends keyof AgentFormState>(
    key: K,
    value: AgentFormState[K],
  ) => {
    setForm((prev) => ({ ...prev, [key]: value }))
  }

  const openEditDialog = (agent: AgentConfigInfo) => {
    if (!isEditableAgent(agent)) {
      return
    }
    setEditingAgent(agent)
    setIsCreating(false)
    setForm(buildForm(agent))
    setEditorOpen(true)
  }

  const openCreateDialog = () => {
    setEditingAgent(null)
    setIsCreating(true)
    setForm(EMPTY_FORM)
    setEditorOpen(true)
  }

  const handleSave = async () => {
    try {
      const payload = buildPayload(form)
      setSaving(true)
      if (isCreating) {
        await createAgentConfig(payload)
      } else {
        if (!editingAgent) {
          throw new Error(t("pages.agent.manage.saveError"))
        }
        await updateAgentConfig(editingAgent.id, payload)
      }

      await invalidateAgents()
      setEditorOpen(false)
      toast.success(t("pages.agent.manage.saveSuccess"))
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t("pages.agent.manage.saveError"),
      )
    } finally {
      setSaving(false)
    }
  }

  const openDeleteDialog = (agent: AgentConfigInfo) => {
    setAgentPendingDelete(agent)
    setDeleteOpen(true)
  }

  const handleDelete = async () => {
    if (!agentPendingDelete) {
      return
    }
    try {
      setIsDeletePending(true)
      await deleteAgentConfig(agentPendingDelete.id)
      await invalidateAgents()
      setDeleteOpen(false)
      setAgentPendingDelete(null)
      toast.success(t("pages.agent.manage.delete.success"))
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t("pages.agent.manage.saveError"),
      )
    } finally {
      setIsDeletePending(false)
    }
  }

  const dialogTitle = isCreating
    ? t("pages.agent.manage.add.title")
    : (editingAgent?.name ?? "")

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title={t("navigation.agents")}
        children={
          <Button onClick={openCreateDialog}>
            <IconPlus className="size-4" />
            {t("pages.agent.manage.add.button")}
          </Button>
        }
      />

      <div className="min-h-0 flex-1 overflow-auto px-4 py-4 sm:px-6">
        <div className="mx-auto w-full max-w-6xl space-y-4 pb-8">
          <p className="text-muted-foreground text-sm">
            {t("pages.agent.manage.pageDescription")}
          </p>

          {isLoading ? (
            <div className="text-muted-foreground text-sm">
              {t("labels.loading")}
            </div>
          ) : error ? (
            <div className="text-destructive text-sm">
              {t("pages.agent.manage.loadError")}
            </div>
          ) : (
            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
              {agents.map((agent) => (
                <Card key={agent.id} size="sm" className="h-full">
                  <CardHeader className="border-border border-b">
                    <CardTitle className="flex items-center gap-2">
                      <span className="truncate">{agent.name}</span>
                      {agent.is_main && (
                        <span className="bg-muted text-muted-foreground rounded px-1.5 py-0.5 text-[10px] tracking-wide uppercase">
                          {t("pages.agent.manage.badge.main")}
                        </span>
                      )}
                      {agent.is_default && (
                        <span className="bg-primary/10 text-primary rounded px-1.5 py-0.5 text-[10px] tracking-wide uppercase">
                          {t("pages.agent.manage.badge.default")}
                        </span>
                      )}
                    </CardTitle>
                    <CardDescription className="break-all">
                      {agent.id}
                    </CardDescription>
                    <CardAction />
                  </CardHeader>
                  <CardContent className="space-y-4 pt-4">
                    <div className="space-y-2 text-sm">
                      <div>
                        <span className="text-muted-foreground">
                          {t("pages.agent.manage.meta.effectiveModel")}
                          {":"}
                        </span>{" "}
                        <span>
                          {agent.effective_model_name ||
                            t("pages.agent.manage.inherit")}
                        </span>
                      </div>
                      <div>
                        <span className="text-muted-foreground">
                          {t("pages.agent.manage.meta.skillCount")}
                          {":"}
                        </span>{" "}
                        <span>{agent.skills_count}</span>
                      </div>
                      <div>
                        <span className="text-muted-foreground">
                          {t("pages.agent.manage.meta.effectiveWorkspace")}
                          {":"}
                        </span>{" "}
                        <span className="break-all">
                          {agent.effective_workspace}
                        </span>
                      </div>
                    </div>

                    <div className="flex flex-wrap gap-2">
                      <Button variant="outline" size="sm" asChild>
                        <Link
                          to="/agent/$agentId"
                          params={{ agentId: agent.id }}
                        >
                          <IconInfoCircle className="size-4" />
                          {t("pages.agent.manage.actions.details")}
                        </Link>
                      </Button>
                      {isEditableAgent(agent) && (
                        <>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => setWorkspaceEditorAgentId(agent.id)}
                          >
                            <IconFolderOpen className="size-4" />
                            {t("pages.agent.manage.workspaceFiles.button")}
                          </Button>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => openEditDialog(agent)}
                          >
                            <IconEdit className="size-4" />
                            {t("pages.agent.manage.actions.edit")}
                          </Button>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => openDeleteDialog(agent)}
                            disabled={!agent.can_delete}
                          >
                            <IconTrash className="size-4" />
                            {t("pages.agent.manage.actions.delete")}
                          </Button>
                        </>
                      )}
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
        </div>
      </div>

      <Dialog open={editorOpen} onOpenChange={setEditorOpen}>
        <DialogContent className="overflow-auto sm:max-w-2xl">
          <DialogHeader className="pb-4">
            <DialogTitle className="text-lg">
              {isCreating ? t("pages.agent.manage.add.title") : dialogTitle}
            </DialogTitle>
            <DialogDescription>
              {t("pages.agent.manage.editDescription")}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-5 py-2">
            <div className="bg-card rounded-lg border p-4">
              <h4 className="text-foreground mb-3 text-sm font-medium">
                {t("pages.agent.manage.form.basicInfo")}
              </h4>
              <div className="space-y-4">
                <Field
                  label={t("pages.agent.manage.form.id")}
                  hint={
                    isCreating
                      ? t("pages.agent.manage.form.idHint")
                      : t("pages.agent.manage.form.idLocked")
                  }
                  layout="setting-row"
                >
                  <Input
                    value={form.id}
                    onChange={(e) => handleFieldChange("id", e.target.value)}
                    disabled={!isCreating}
                    className="font-mono text-sm"
                  />
                </Field>
                <Field
                  label={t("pages.agent.manage.form.name")}
                  hint={t("pages.agent.manage.form.nameHint")}
                  layout="setting-row"
                >
                  <Input
                    value={form.name}
                    onChange={(e) => handleFieldChange("name", e.target.value)}
                    placeholder={t("pages.agent.manage.form.namePlaceholder")}
                  />
                </Field>
              </div>
            </div>

            <div className="bg-card rounded-lg border p-4">
              <h4 className="text-foreground mb-3 text-sm font-medium">
                {t("pages.agent.manage.form.advancedSettings")}
              </h4>
              <div className="space-y-4">
                <Field
                  label={t("pages.agent.manage.form.model")}
                  hint={t("pages.agent.manage.form.modelHint")}
                  layout="setting-row"
                >
                  <Input
                    value={form.modelName}
                    onChange={(e) =>
                      handleFieldChange("modelName", e.target.value)
                    }
                    placeholder={t("pages.agent.manage.inherit")}
                  />
                </Field>
                <Field
                  label={t("pages.agent.manage.form.workspace")}
                  hint={t("pages.agent.manage.form.workspaceHint")}
                  layout="setting-row"
                >
                  <Input
                    value={form.workspace}
                    onChange={(e) =>
                      handleFieldChange("workspace", e.target.value)
                    }
                    placeholder={t("pages.agent.manage.inherit")}
                    className="font-mono text-sm"
                  />
                </Field>
                <Field
                  label={t("pages.agent.manage.form.skills")}
                  hint={t("pages.agent.manage.form.skillsHint")}
                  layout="setting-row"
                >
                  <Input
                    value={form.skillsText}
                    onChange={(e) =>
                      handleFieldChange("skillsText", e.target.value)
                    }
                    placeholder="gh-cli, jira"
                  />
                </Field>
              </div>
            </div>
          </div>

          <DialogFooter className="gap-2 border-t pt-4">
            <Button variant="outline" onClick={() => setEditorOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button disabled={saving} onClick={() => void handleSave()}>
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

      <DeleteAgentDialog
        open={deleteOpen}
        agentPendingDelete={agentPendingDelete}
        isDeletePending={isDeletePending}
        onOpenChange={setDeleteOpen}
        onConfirm={() => void handleDelete()}
      />

      {workspaceEditorAgentId && (
        <WorkspaceFilesEditor
          agentId={workspaceEditorAgentId}
          open={workspaceEditorAgentId !== null}
          onOpenChange={(open) => {
            if (!open) {
              setWorkspaceEditorAgentId(null)
            }
          }}
        />
      )}
    </div>
  )
}
