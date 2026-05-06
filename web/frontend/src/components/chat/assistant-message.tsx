import {
  IconBrain,
  IconCheck,
  IconChevronDown,
  IconCopy,
  IconDownload,
  IconFileText,
} from "@tabler/icons-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import ReactMarkdown from "react-markdown"
import rehypeHighlight from "rehype-highlight"
import rehypeRaw from "rehype-raw"
import rehypeSanitize from "rehype-sanitize"
import remarkGfm from "remark-gfm"

import { Button } from "@/components/ui/button"
import { formatMessageTime } from "@/hooks/use-pico-chat"
import { cn } from "@/lib/utils"
import { type ChatAttachment } from "@/store/chat"

interface AssistantMessageProps {
  content: string
  attachments?: ChatAttachment[]
  isThought?: boolean
  timestamp?: string | number
  agentId?: string
  agentName?: string
  modelName?: string
}

export function AssistantMessage({
  content,
  attachments = [],
  isThought = false,
  timestamp = "",
  agentId,
  agentName,
  modelName,
}: AssistantMessageProps) {
  const { t } = useTranslation()
  const [isCopied, setIsCopied] = useState(false)
  const [isThoughtExpanded, setIsThoughtExpanded] = useState(false)
  const hasText = content.trim().length > 0
  const formattedTimestamp =
    timestamp !== "" ? formatMessageTime(timestamp) : ""
  const senderLabel = [agentName || agentId, modelName].filter(Boolean).join(" · ")
  const imageAttachments = attachments.filter(
    (attachment) => attachment.type === "image",
  )
  const fileAttachments = attachments.filter(
    (attachment) => attachment.type !== "image",
  )

  const handleCopy = () => {
    navigator.clipboard.writeText(content).then(() => {
      setIsCopied(true)
      setTimeout(() => setIsCopied(false), 2000)
    })
  }

  return (
    <div className="group flex w-full flex-col gap-1.5">
      <div className="text-muted-foreground flex items-center justify-between gap-2 px-1 text-xs opacity-70">
        <div className="flex items-center gap-2">
          <span>{senderLabel || "PicoClaw"}</span>
          {isThought && (
            <span className="inline-flex items-center gap-1 rounded-full border border-amber-300/80 bg-amber-100/80 px-2 py-0.5 text-[11px] font-medium text-amber-800 dark:border-amber-500/40 dark:bg-amber-500/15 dark:text-amber-200">
              <IconBrain className="size-3" />
              <span>{t("chat.reasoningLabel")}</span>
            </span>
          )}
          {formattedTimestamp && (
            <>
              <span className="opacity-50">•</span>
              <span>{formattedTimestamp}</span>
            </>
          )}
        </div>
      </div>

      {(hasText || isThought) && (
        <div
          className={cn(
            "relative overflow-hidden rounded-xl border",
            isThought
              ? "border-amber-200/90 bg-amber-50/70 text-amber-950 dark:border-amber-500/35 dark:bg-amber-500/10 dark:text-amber-100"
              : "border-border/60 bg-card text-card-foreground",
          )}
        >
          {isThought && (
            <button
              type="button"
              className="text-muted-foreground hover:text-foreground flex w-full items-center justify-between border-b border-amber-200/70 bg-amber-100/40 px-3 py-2.5 text-left text-xs dark:border-amber-500/20 dark:bg-amber-500/5"
              onClick={() => setIsThoughtExpanded((value) => !value)}
            >
              <div className="flex min-w-0 items-center gap-2">
                <IconBrain className="size-3.5 shrink-0" />
                <div className="min-w-0">
                  <div className="font-medium text-amber-900 dark:text-amber-100">
                    {t("chat.reasoningLabel")}
                  </div>
                  <div className="truncate text-[10px] opacity-70">
                    {isThoughtExpanded
                      ? "Internal reasoning details"
                      : "Preview hidden by default — expand to inspect"}
                  </div>
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

          {(!isThought || isThoughtExpanded) && hasText && (
            <div
              className={cn(
                "prose prose-sm dark:prose-invert prose-headings:my-2 prose-p:leading-normal prose-ul:my-1.5 prose-ol:my-1.5 prose-li:my-0.5 prose-blockquote:my-2 prose-hr:my-3 prose-table:my-2 prose-pre:my-2 prose-pre:overflow-x-auto prose-pre:rounded-lg prose-pre:border prose-pre:bg-zinc-100 prose-pre:p-0 prose-pre:text-zinc-900 max-w-none [overflow-wrap:anywhere] break-words dark:prose-pre:bg-zinc-950 dark:prose-pre:text-zinc-100",
                isThought
                  ? "prose-p:my-1 prose-p:whitespace-pre-wrap p-2.5 text-[12px] leading-normal text-amber-950/90 dark:text-amber-50/90"
                  : "prose-p:my-1.5 prose-p:whitespace-pre-wrap p-3 text-[14px] leading-normal",
              )}
            >
              <ReactMarkdown
                remarkPlugins={[remarkGfm]}
                rehypePlugins={[rehypeRaw, rehypeSanitize, rehypeHighlight]}
              >
                {content}
              </ReactMarkdown>
            </div>
          )}

          {isThought && !isThoughtExpanded && (
            <div className="pointer-events-none absolute inset-x-0 bottom-0 h-20 bg-gradient-to-t from-amber-50 via-amber-50/95 to-transparent dark:from-amber-500/10 dark:via-amber-500/5" />
          )}

          <Button
            variant="ghost"
            size="icon"
            className={cn(
              "absolute top-2 right-2 h-7 w-7 opacity-0 transition-opacity group-hover:opacity-100",
              isThought
                ? "bg-amber-100/70 hover:bg-amber-200/80 dark:bg-amber-500/20 dark:hover:bg-amber-400/30"
                : "bg-background/50 hover:bg-background/80",
            )}
            onClick={handleCopy}
          >
            {isCopied ? (
              <IconCheck className="h-4 w-4 text-green-500" />
            ) : (
              <IconCopy className="text-muted-foreground h-4 w-4" />
            )}
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
              className="group/img relative overflow-hidden rounded-xl border border-border/50 bg-muted/30 shadow-sm transition-colors hover:border-border/80"
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
              className="group/file flex w-fit min-w-[220px] max-w-sm items-center gap-3.5 rounded-xl border border-border/60 bg-card px-4 py-3 transition-all duration-300 hover:-translate-y-0.5 hover:border-violet-500/30 hover:shadow-sm dark:hover:border-violet-500/40"
            >
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-violet-400 ring-1 ring-violet-500/10 dark:bg-violet-500/10 dark:text-violet-400 dark:ring-violet-500/30">
                <IconFileText className="h-5 w-5" />
              </div>
              <div className="flex min-w-0 flex-1 flex-col pr-1">
                <span className="truncate text-[14px] font-medium leading-tight text-foreground/90 transition-colors group-hover/file:text-violet-600 dark:group-hover/file:text-violet-400">
                  {attachment.filename || "Download file"}
                </span>
                <span className="mt-1 text-[12px] font-medium text-muted-foreground/70">
                  {attachment.filename?.split(".").pop()?.toUpperCase() || "FILE"}
                </span>
              </div>
              <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-muted/60 text-muted-foreground/50 transition-all duration-300 group-hover/file:bg-violet-400 group-hover/file:text-white group-hover/file:shadow-sm dark:bg-muted/20 dark:group-hover/file:bg-violet-400">
                <IconDownload className="h-4 w-4 transition-transform duration-300 group-hover/file:-translate-y-[1px]" />
              </div>
            </a>
          ))}
        </div>
      )}
    </div>
  )
}
