import {
  IconArrowLeft,
  IconCalendarTime,
  IconCheck,
  IconClock,
  IconDatabase,
  IconFileText,
  IconFolder,
  IconHistory,
  IconInfoCircle,
  IconX,
} from "@tabler/icons-react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import type React from "react"
import ReactMarkdown from "react-markdown"
import remarkGfm from "remark-gfm"

import { type AgentDetailFile, getAgentDetail } from "@/api/agent-detail"
import { PageHeader } from "@/components/page-header"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"

interface AgentDetailPageProps {
  agentId: string
}

function formatBytes(size: number) {
  if (!Number.isFinite(size) || size <= 0) return "0 B"
  const units = ["B", "KB", "MB", "GB"]
  let value = size
  let index = 0
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024
    index += 1
  }
  return `${value.toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

function formatDate(value?: string) {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

function formatMs(value?: number) {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  return date.toLocaleString()
}

function scheduleLabel(schedule: {
  kind: string
  everyMs?: number
  expr?: string
}) {
  if (schedule.kind === "every" && schedule.everyMs) {
    return `every ${Math.round(schedule.everyMs / 1000)}s`
  }
  if (schedule.kind === "cron") {
    return schedule.expr || "cron"
  }
  return schedule.kind || "one-time"
}

export function AgentDetailPage({ agentId }: AgentDetailPageProps) {
  const { data, isLoading, error } = useQuery({
    queryKey: ["agent-detail", agentId],
    queryFn: () => getAgentDetail(agentId),
    staleTime: 30 * 1000,
  })

  if (isLoading) {
    return (
      <div className="flex h-full flex-col">
        <PageHeader title="Agent detail" />
        <div className="text-muted-foreground p-6 text-sm">
          Loading agent...
        </div>
      </div>
    )
  }

  if (error || !data) {
    return (
      <div className="flex h-full flex-col">
        <PageHeader title="Agent detail" />
        <div className="p-6">
          <Card>
            <CardHeader>
              <CardTitle>Unable to load agent</CardTitle>
              <CardDescription>
                {error instanceof Error ? error.message : "Unknown error"}
              </CardDescription>
            </CardHeader>
          </Card>
        </div>
      </div>
    )
  }

  const agent = data.agent
  const existingPromptFiles = data.prompt_files.filter((file) => file.exists)

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title={agent.name || agent.id}
        children={
          <Button variant="outline" size="sm" asChild>
            <Link to="/agent/manage">
              <IconArrowLeft className="size-4" />
              Back to agents
            </Link>
          </Button>
        }
      />

      <div className="min-h-0 flex-1 overflow-auto px-4 py-4 sm:px-6">
        <div className="mx-auto w-full max-w-7xl space-y-6 pb-10">
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="outline">{agent.id}</Badge>
            {agent.is_main && <Badge>Main</Badge>}
            {agent.is_default && <Badge variant="secondary">Default</Badge>}
            {agent.is_implicit && <Badge variant="outline">Implicit</Badge>}
          </div>

          <div className="grid gap-4 lg:grid-cols-[2fr_1fr]">
            <OverviewCard data={data} />
            <DirectoriesCard directories={data.directories} />
          </div>

          <SectionCard
            icon={<IconFileText className="size-4" />}
            title="Prompt files"
            description="Identity and instruction markdown files that can shape this agent."
          >
            {existingPromptFiles.length === 0 ? (
              <EmptyState>No prompt markdown files found.</EmptyState>
            ) : (
              <div className="grid gap-4 lg:grid-cols-2">
                {data.prompt_files.map((file) => (
                  <FilePreview key={file.path} file={file} />
                ))}
              </div>
            )}
          </SectionCard>

          <SectionCard
            icon={<IconDatabase className="size-4" />}
            title="Memory"
            description="Long-term memory and recent daily notes from the agent workspace."
          >
            <div className="grid gap-4 lg:grid-cols-2">
              <FilePreview
                file={data.memory.long_term}
                title="Long-term memory"
              />
              <Card className="border-dashed" size="sm">
                <CardHeader>
                  <CardTitle className="text-base">
                    Recent daily notes
                  </CardTitle>
                  <CardDescription>
                    {data.memory.recent_daily.length} recent memory files
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  {data.memory.recent_daily.length === 0 ? (
                    <EmptyState>No recent daily memory notes.</EmptyState>
                  ) : (
                    <div className="space-y-2">
                      {data.memory.recent_daily.map((file) => (
                        <FileRow key={file.path} file={file} />
                      ))}
                    </div>
                  )}
                </CardContent>
              </Card>
            </div>
          </SectionCard>

          <SectionCard
            icon={<IconCheck className="size-4" />}
            title="Skills"
            description="Configured skills and workspace skills visible to this agent."
          >
            {data.skills.length === 0 ? (
              <EmptyState>No configured or workspace skills found.</EmptyState>
            ) : (
              <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                {data.skills.map((skill) => (
                  <Card key={`${skill.name}-${skill.path}`} size="sm">
                    <CardHeader>
                      <CardTitle className="flex items-center gap-2 text-base">
                        {skill.name}
                        {skill.resolved ? (
                          <Badge variant="secondary">resolved</Badge>
                        ) : (
                          <Badge variant="destructive">missing</Badge>
                        )}
                      </CardTitle>
                      <CardDescription>
                        {skill.description || skill.path || "-"}
                      </CardDescription>
                    </CardHeader>
                    <CardContent className="flex flex-wrap gap-2 text-xs">
                      {skill.configured && (
                        <Badge variant="outline">configured</Badge>
                      )}
                      {skill.source && (
                        <Badge variant="outline">{skill.source}</Badge>
                      )}
                    </CardContent>
                  </Card>
                ))}
              </div>
            )}
          </SectionCard>

          <SectionCard
            icon={<IconHistory className="size-4" />}
            title="Recent sessions"
            description="Recent conversation summaries for this agent workspace."
          >
            {data.sessions.length === 0 ? (
              <EmptyState>No sessions found.</EmptyState>
            ) : (
              <div className="space-y-2">
                {data.sessions.map((session) => (
                  <Card key={session.id} size="sm">
                    <CardContent className="flex flex-col gap-2 p-4 md:flex-row md:items-center md:justify-between">
                      <div className="min-w-0">
                        <div className="truncate font-medium">
                          {session.title || session.id}
                        </div>
                        <div className="text-muted-foreground line-clamp-1 text-sm">
                          {session.preview || "No preview"}
                        </div>
                      </div>
                      <div className="text-muted-foreground flex shrink-0 items-center gap-4 text-xs">
                        <span>{session.message_count} messages</span>
                        <span>{formatDate(session.updated)}</span>
                      </div>
                    </CardContent>
                  </Card>
                ))}
              </div>
            )}
          </SectionCard>

          <div className="grid gap-4 lg:grid-cols-2">
            <SectionCard
              icon={<IconCalendarTime className="size-4" />}
              title="Cron"
              description="Scheduled jobs stored in this workspace."
            >
              {data.cron.count === 0 ? (
                <EmptyState>No cron jobs found.</EmptyState>
              ) : (
                <div className="space-y-2">
                  {(data.cron.jobs ?? []).map((job) => (
                    <Card key={job.id} size="sm">
                      <CardContent className="space-y-2 p-4">
                        <div className="flex items-center justify-between gap-2">
                          <div className="font-medium">
                            {job.name || job.id}
                          </div>
                          <Badge
                            variant={job.enabled ? "secondary" : "outline"}
                          >
                            {job.enabled ? "enabled" : "disabled"}
                          </Badge>
                        </div>
                        <div className="text-muted-foreground text-sm">
                          {scheduleLabel(job.schedule)}
                        </div>
                        <div className="text-muted-foreground flex flex-wrap gap-4 text-xs">
                          <span>Next: {formatMs(job.state?.nextRunAtMs)}</span>
                          <span>Last: {formatMs(job.state?.lastRunAtMs)}</span>
                        </div>
                      </CardContent>
                    </Card>
                  ))}
                </div>
              )}
            </SectionCard>

            <SectionCard
              icon={<IconFolder className="size-4" />}
              title="State files"
              description="Internal state file summaries. Contents are not expanded by default."
            >
              {data.state_files.length === 0 ? (
                <EmptyState>No state files found.</EmptyState>
              ) : (
                <div className="space-y-2">
                  {data.state_files.map((file) => (
                    <FileRow key={file.path} file={file} />
                  ))}
                </div>
              )}
            </SectionCard>
          </div>
        </div>
      </div>
    </div>
  )
}

function OverviewCard({
  data,
}: {
  data: Awaited<ReturnType<typeof getAgentDetail>>
}) {
  const agent = data.agent
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <IconInfoCircle className="size-5" />
          Overview
        </CardTitle>
        <CardDescription>
          Effective configuration for this agent.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-3 text-sm md:grid-cols-2">
          <Meta
            label="Effective model"
            value={agent.effective_model_name || "-"}
          />
          <Meta
            label="Configured model"
            value={agent.model_name || "inherits default"}
          />
          <Meta
            label="Fallback models"
            value={(agent.model_fallbacks ?? []).join(", ") || "-"}
          />
          <Meta label="Skills" value={`${agent.skills_count} configured`} />
          <Meta label="Workspace" value={data.workspace} wide />
          <Meta
            label="Configured workspace"
            value={agent.workspace || "inherits default"}
            wide
          />
        </div>
      </CardContent>
    </Card>
  )
}

function DirectoriesCard({
  directories,
}: {
  directories: Awaited<ReturnType<typeof getAgentDetail>>["directories"]
}) {
  const entries = [
    ["memory", directories.memory_exists],
    ["skills", directories.skills_exists],
    ["sessions", directories.sessions_exists],
    ["cron", directories.cron_exists],
    ["state", directories.state_exists],
  ] as const
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <IconFolder className="size-5" />
          Workspace directories
        </CardTitle>
        <CardDescription>
          Known data directories in the effective workspace.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-2">
        {entries.map(([name, exists]) => (
          <div key={name} className="flex items-center justify-between text-sm">
            <span>{name}/</span>
            {exists ? (
              <Badge variant="secondary">
                <IconCheck className="size-3" /> exists
              </Badge>
            ) : (
              <Badge variant="outline">
                <IconX className="size-3" /> missing
              </Badge>
            )}
          </div>
        ))}
      </CardContent>
    </Card>
  )
}

function SectionCard({
  icon,
  title,
  description,
  children,
}: {
  icon: React.ReactNode
  title: string
  description: string
  children: React.ReactNode
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          {icon}
          {title}
        </CardTitle>
        <CardDescription>{description}</CardDescription>
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  )
}

function Meta({
  label,
  value,
  wide,
}: {
  label: string
  value: string
  wide?: boolean
}) {
  return (
    <div className={wide ? "md:col-span-2" : undefined}>
      <div className="text-muted-foreground text-xs tracking-wide uppercase">
        {label}
      </div>
      <div className="font-medium break-all">{value}</div>
    </div>
  )
}

function FilePreview({
  file,
  title,
}: {
  file: AgentDetailFile
  title?: string
}) {
  return (
    <Card
      size="sm"
      className={!file.exists ? "border-dashed opacity-70" : undefined}
    >
      <CardHeader>
        <CardTitle className="flex items-center justify-between gap-2 text-base">
          <span>{title || file.name}</span>
          <Badge variant={file.exists ? "secondary" : "outline"}>
            {file.exists ? "exists" : "missing"}
          </Badge>
        </CardTitle>
        <CardDescription className="break-all">
          {file.path}{" "}
          {file.exists
            ? `• ${formatBytes(file.size)} • ${formatDate(file.modified_at)}`
            : ""}
        </CardDescription>
      </CardHeader>
      {file.exists && file.content ? (
        <CardContent>
          <div className="prose prose-sm dark:prose-invert max-h-80 max-w-none overflow-auto rounded-md border p-3">
            <ReactMarkdown remarkPlugins={[remarkGfm]}>
              {file.content}
            </ReactMarkdown>
          </div>
          {file.truncated && (
            <div className="text-muted-foreground mt-2 text-xs">
              Content truncated.
            </div>
          )}
        </CardContent>
      ) : null}
    </Card>
  )
}

function FileRow({ file }: { file: AgentDetailFile }) {
  return (
    <div className="flex items-center justify-between gap-3 rounded-md border px-3 py-2 text-sm">
      <div className="min-w-0">
        <div className="truncate font-medium">{file.path}</div>
        <div className="text-muted-foreground flex items-center gap-2 text-xs">
          <IconClock className="size-3" />
          {formatDate(file.modified_at)}
        </div>
      </div>
      <div className="text-muted-foreground shrink-0 text-xs">
        {formatBytes(file.size)}
      </div>
    </div>
  )
}

function EmptyState({ children }: { children: React.ReactNode }) {
  return (
    <div className="text-muted-foreground rounded-md border border-dashed p-4 text-sm">
      {children}
    </div>
  )
}
