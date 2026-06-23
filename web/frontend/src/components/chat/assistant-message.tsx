import {
  IconBrain,
  IconCheck,
  IconChevronDown,
  IconCopy,
  IconDownload,
  IconFileText,
  IconTool,
} from "@tabler/icons-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import ReactMarkdown from "react-markdown"
import rehypeHighlight from "rehype-highlight"
import rehypeRaw from "rehype-raw"
import rehypeSanitize from "rehype-sanitize"
import remarkGfm from "remark-gfm"

import {
  MessageCodeBlock,
  MarkdownCodeBlock,
} from "@/components/chat/message-code-block"
import { Button } from "@/components/ui/button"
import { formatMessageTime } from "@/hooks/use-pico-chat"
import { useCopyToClipboard } from "@/hooks/use-copy-to-clipboard"
import { cn } from "@/lib/utils"
import {
  type AssistantMessageKind,
  type ChatAttachment,
  type ChatToolCall,
} from "@/store/chat"

const FOLLOW_UP_PREFIXES = [
  "如果你愿意",
  "如果需要",
  "如果你需要",
  "你要的话",
  "需要的话",
]

function stripWrappingQuote(text: string) {
  return text
    .trim()
    .replace(/^[“”"'「『]+/, "")
    .replace(/[“”"'」』。，,.\s]+$/, "")
}

function extractFollowUpPrompt(content: string): string | null {
  const normalized = content.trim()
  if (!normalized) {
    return null
  }

  const lines = normalized
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
  const tail = lines.slice(-4).join("\n")
  if (!FOLLOW_UP_PREFIXES.some((prefix) => tail.includes(prefix))) {
    return null
  }

  const quoted = tail.match(/[“"「『]([^”"」』]{6,120})[”"」』]/)
  if (quoted?.[1]) {
    return `请直接给我这份：${stripWrappingQuote(quoted[1])}`
  }

  const colonIndex = Math.max(tail.lastIndexOf("："), tail.lastIndexOf(":"))
  if (colonIndex >= 0) {
    const afterColon = stripWrappingQuote(tail.slice(colonIndex + 1))
    if (afterColon.length >= 6 && afterColon.length <= 120) {
      return `请直接给我这份：${afterColon}`
    }
  }

  if (/下一条|直接给你|我可以/.test(tail)) {
    return "请继续，直接给我你刚才提到的内容"
  }

  return null
}

interface AssistantMessageProps {
  content: string
  attachments?: ChatAttachment[]
  kind?: AssistantMessageKind
  modelName?: string
  toolCalls?: ChatToolCall[]
  timestamp?: string | number
  agentId?: string
  agentName?: string
  onQuickPrompt?: (prompt: string) => void
}

export function AssistantMessage({
  content,
  attachments = [],
  kind = "normal",
  modelName,
  toolCalls = [],
  timestamp = "",
  agentId,
  agentName,
  onQuickPrompt,
}: AssistantMessageProps) {
  const { t } = useTranslation()
  const { copy, isCopied } = useCopyToClipboard()
  const [isThoughtExpanded, setIsThoughtExpanded] = useState(false)
  const isThought = kind === "thought"
  const isToolCalls = kind === "tool_calls"
  const isCollapsedBlock = isThought || isToolCalls
  const hasText = content.trim().length > 0
  const hasToolCalls = toolCalls.length > 0
  const formattedTimestamp =
    timestamp !== "" ? formatMessageTime(timestamp) : ""
  const senderLabel = [agentName || agentId, modelName]
    .filter(Boolean)
    .join(" · ")
  const imageAttachments = attachments.filter(
    (attachment) => attachment.type === "image",
  )
  const fileAttachments = attachments.filter(
    (attachment) => attachment.type !== "image",
  )
  const followUpPrompt = !isThought ? extractFollowUpPrompt(content) : null
  const collapsedLabel = isThought
    ? t("chat.reasoningLabel")
    : t("chat.toolCallsLabel")
  const copyMessageLabel = isCopied
    ? t("chat.copiedLabel")
    : t("chat.copyMessage")

  return (
    <div className="group flex w-full flex-col gap-1.5">
      {!isCollapsedBlock && (
        <div className="text-muted-foreground flex items-center justify-between gap-2 px-1 text-xs opacity-70">
          <div className="flex items-center gap-2">
            <span>{senderLabel || "PicoClaw"}</span>
            {formattedTimestamp && (
              <>
                <span className="opacity-50">•</span>
                <span>{formattedTimestamp}</span>
              </>
            )}
          </div>
        </div>
      )}

      {(hasText || isCollapsedBlock || hasToolCalls) && (
        <div
          className={cn(
            "relative overflow-hidden rounded-xl border",
            isThought
              ? "border-amber-200/90 bg-amber-50/70 text-amber-950 dark:border-amber-500/35 dark:bg-amber-500/10 dark:text-amber-100"
              : isCollapsedBlock
                ? "border-border/30 bg-muted/20 text-muted-foreground dark:border-border/20 dark:bg-muted/10"
                : "border-border/60 bg-card text-card-foreground",
          )}
        >
          {isCollapsedBlock && (
            <button
              type="button"
              className={cn(
                "flex w-full items-center justify-between px-3 py-2 text-left text-[12px] font-medium transition-colors select-none",
                isThought
                  ? "text-muted-foreground hover:text-foreground border-b border-amber-200/70 bg-amber-100/40 dark:border-amber-500/20 dark:bg-amber-500/5"
                  : "text-muted-foreground/60 hover:text-muted-foreground/80",
              )}
              onClick={() => setIsThoughtExpanded((value) => !value)}
            >
              <div className="flex min-w-0 items-center gap-1.5">
                {isThought ? (
                  <IconBrain className="size-3.5 shrink-0" />
                ) : (
                  <IconTool className="size-3.5 shrink-0" />
                )}
                <div className="min-w-0">
                  <div
                    className={cn(
                      isThought && "font-medium text-amber-900 dark:text-amber-100",
                    )}
                  >
                    {collapsedLabel}
                  </div>
                  {isThought && (
                    <div className="truncate text-[10px] opacity-70">
                      {isThoughtExpanded
                        ? "Internal reasoning details"
                        : "Preview hidden by default — expand to inspect"}
                    </div>
                  )}
                </div>
              </div>
              <span className="flex items-center gap-1 text-[10px] font-medium opacity-80">
                <span>{isThoughtExpanded ? "Hide" : "Expand"}</span>
                <IconChevronDown
                  className={cn(
                    "size-3 transition-transform",
                    isThoughtExpanded ? "rotate-180" : "rotate-0",
                  )}
                />
              </span>
            </button>
          )}

          {isToolCalls && hasToolCalls && (!isCollapsedBlock || isThoughtExpanded) && (
            <div className="space-y-3 px-3 pt-0 pb-3">
              {toolCalls.map((toolCall, index) => {
                const explanation =
                  toolCall.extraContent?.toolFeedbackExplanation?.trim() ?? ""
                const toolName = toolCall.function?.name?.trim() ?? ""
                const toolArguments = toolCall.function?.arguments?.trim() ?? ""
                const hasFunctionSummary = toolName || toolArguments

                if (!explanation && !hasFunctionSummary) {
                  return null
                }

                return (
                  <div
                    key={toolCall.id ?? `${toolName}-${index}`}
                    className={cn(
                      "space-y-3",
                      index > 0 && "border-border/20 border-t pt-3",
                    )}
                  >
                    {explanation && (
                      <div className="space-y-1.5">
                        <div className="text-muted-foreground/55 text-[11px] font-medium tracking-wide uppercase">
                          {t("chat.toolCallExplanationLabel")}
                        </div>
                        <div className="prose dark:prose-invert prose-p:my-1.5 prose-p:whitespace-pre-wrap max-w-none text-[13px] leading-relaxed [overflow-wrap:anywhere] break-words opacity-75">
                          <ReactMarkdown
                            remarkPlugins={[remarkGfm]}
                            rehypePlugins={[
                              rehypeRaw,
                              rehypeSanitize,
                              rehypeHighlight,
                            ]}
                            components={{
                              pre: MarkdownCodeBlock,
                            }}
                          >
                            {explanation}
                          </ReactMarkdown>
                        </div>
                      </div>
                    )}

                    {hasFunctionSummary && (
                      <div
                        className={cn(
                          "space-y-1.5",
                          explanation && "border-border/20 border-t pt-3",
                        )}
                      >
                        <div className="text-muted-foreground/55 text-[11px] font-medium tracking-wide uppercase">
                          {t("chat.toolCallFunctionLabel")}
                        </div>
                        <div className="bg-background/55 border-border/25 space-y-2 rounded-lg border px-3 py-2.5">
                          {toolName && !toolArguments && (
                            <div className="text-foreground/75 font-mono text-[12px] font-semibold">
                              {toolName}
                            </div>
                          )}
                          {toolArguments && (
                            <MessageCodeBlock
                              code={toolArguments}
                              language="json"
                              label={toolName || t("chat.toolCallArgumentsLabel")}
                              className="my-0 shadow-none"
                              bodyClassName="px-3 py-2 text-[12px] leading-relaxed"
                            />
                          )}
                        </div>
                      </div>
                    )}
                  </div>
                )
              })}
            </div>
          )}

          {(!isCollapsedBlock || isThoughtExpanded) && !isToolCalls && hasText && (
            <div
              className={cn(
                "prose prose-sm dark:prose-invert prose-headings:my-2 prose-p:leading-normal prose-ul:my-1.5 prose-ol:my-1.5 prose-li:my-0.5 prose-blockquote:my-2 prose-hr:my-3 prose-table:my-2 prose-pre:my-2 prose-pre:overflow-x-auto prose-pre:rounded-lg prose-pre:border prose-pre:bg-zinc-100 prose-pre:p-0 prose-pre:text-zinc-900 dark:prose-pre:bg-zinc-950 dark:prose-pre:text-zinc-100 max-w-none [overflow-wrap:anywhere] break-words",
                isThought
                  ? "prose-p:my-1 prose-p:whitespace-pre-wrap p-2.5 text-[12px] leading-normal text-amber-950/90 dark:text-amber-50/90"
                  : "prose-p:my-1.5 prose-p:whitespace-pre-wrap p-3 text-[14px] leading-normal",
              )}
            >
              <ReactMarkdown
                remarkPlugins={[remarkGfm]}
                rehypePlugins={[rehypeRaw, rehypeSanitize, rehypeHighlight]}
                components={{
                  pre: MarkdownCodeBlock,
                }}
              >
                {content}
              </ReactMarkdown>
            </div>
          )}

          {isThought && !isThoughtExpanded && (
            <div className="pointer-events-none absolute inset-x-0 bottom-0 h-20 bg-gradient-to-t from-amber-50 via-amber-50/95 to-transparent dark:from-amber-500/10 dark:via-amber-500/5" />
          )}

          {!isCollapsedBlock && hasText && (
            <Button
              variant="ghost"
              size="icon"
              className={cn(
                "absolute top-2 right-2 h-7 w-7 opacity-0 transition-opacity group-hover:opacity-100",
                isThought
                  ? "bg-amber-100/70 hover:bg-amber-200/80 dark:bg-amber-500/20 dark:hover:bg-amber-400/30"
                  : "bg-background/50 hover:bg-background/80",
              )}
              onClick={() => void copy(content)}
              aria-label={copyMessageLabel}
              title={copyMessageLabel}
            >
              {isCopied ? (
                <IconCheck className="h-4 w-4 text-green-500" />
              ) : (
                <IconCopy className="text-muted-foreground h-4 w-4" />
              )}
            </Button>
          )}
        </div>
      )}

      {followUpPrompt && onQuickPrompt && (
        <div className="flex px-1">
          <Button
            type="button"
            variant="secondary"
            size="sm"
            className="border-border/60 bg-background/80 h-7 max-w-full gap-1.5 rounded-full border px-2.5 text-xs font-normal shadow-sm"
            onClick={() => onQuickPrompt(followUpPrompt)}
            title={followUpPrompt}
          >
            <span className="shrink-0">继续生成</span>
            <span className="text-muted-foreground max-w-80 truncate">
              {followUpPrompt.replace(/^请直接给我这份：/, "")}
            </span>
          </Button>
        </div>
      )}

      {imageAttachments.length > 0 && (
        <div className="mt-1 flex flex-wrap gap-2">
          {imageAttachments.map((attachment, index) => (
            <a
              key={`${attachment.url}-${index}`}
              href={attachment.url}
              target="_blank"
              rel="noreferrer"
              className="group/img border-border/50 bg-muted/30 hover:border-border/80 relative overflow-hidden rounded-xl border shadow-sm transition-colors"
            >
              <img
                src={attachment.url}
                alt={attachment.filename || "Attached image"}
                className="max-h-80 max-w-[280px] object-contain transition-transform duration-300 group-hover/img:scale-[1.02]"
              />
              <div className="absolute inset-0 bg-black/0 transition-colors group-hover/img:bg-black/10 dark:group-hover/img:bg-black/20" />
            </a>
          ))}
        </div>
      )}

      {fileAttachments.length > 0 && (
        <div className="mt-1 flex flex-wrap gap-3">
          {fileAttachments.map((attachment, index) => (
            <a
              key={`${attachment.url}-${index}`}
              href={attachment.url}
              download={attachment.filename}
              className="group/file border-border/60 bg-card flex w-fit max-w-sm min-w-[220px] items-center gap-3.5 rounded-xl border px-4 py-3 transition-all duration-300 hover:-translate-y-0.5 hover:border-violet-500/30 hover:shadow-sm dark:hover:border-violet-500/40"
            >
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-violet-400 ring-1 ring-violet-500/10 dark:bg-violet-500/10 dark:text-violet-400 dark:ring-violet-500/30">
                <IconFileText className="h-5 w-5" />
              </div>
              <div className="flex min-w-0 flex-1 flex-col pr-1">
                <span className="text-foreground/90 truncate text-[14px] leading-tight font-medium transition-colors group-hover/file:text-violet-600 dark:group-hover/file:text-violet-400">
                  {attachment.filename || "Download file"}
                </span>
                <span className="text-muted-foreground/70 mt-1 text-[12px] font-medium">
                  {attachment.filename?.split(".").pop()?.toUpperCase() ||
                    "FILE"}
                </span>
              </div>
              <div className="bg-muted/60 text-muted-foreground/50 dark:bg-muted/20 flex h-8 w-8 shrink-0 items-center justify-center rounded-full transition-all duration-300 group-hover/file:bg-violet-400 group-hover/file:text-white group-hover/file:shadow-sm dark:group-hover/file:bg-violet-400">
                <IconDownload className="h-4 w-4 transition-transform duration-300 group-hover/file:-translate-y-[1px]" />
              </div>
            </a>
          ))}
        </div>
      )}
    </div>
  )
}
