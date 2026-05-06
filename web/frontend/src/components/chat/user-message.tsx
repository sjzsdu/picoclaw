import { cn } from "@/lib/utils"
import type { ChatAttachment } from "@/store/chat"

interface UserMessageProps {
  content: string
  attachments?: ChatAttachment[]
}

export function UserMessage({ content, attachments = [] }: UserMessageProps) {
  const hasText = content.trim().length > 0
  const isCommand = content.trim().startsWith("/")
  const imageAttachments = attachments.filter(
    (attachment) => attachment.type === "image",
  )

  return (
    <div className="flex w-full flex-col items-end gap-1.5">
      {imageAttachments.length > 0 && (
        <div className="flex max-w-[70%] flex-wrap justify-end gap-2">
          {imageAttachments.map((attachment, index) => (
            <img
              key={`${attachment.url}-${index}`}
              src={attachment.url}
              alt={attachment.filename || "Uploaded image"}
              className="max-h-72 max-w-full object-cover"
            />
          ))}
        </div>
      )}

      {hasText && (
        <div
          className={cn(
            "max-w-[70%] wrap-break-word whitespace-pre-wrap",
            isCommand
              ? "rounded-xl border border-zinc-200 bg-transparent px-3 py-2.5 font-mono text-[13px] leading-normal text-zinc-800 dark:border-zinc-800/60 dark:bg-[#121212] dark:text-zinc-200 dark:shadow-sm"
              : "rounded-2xl rounded-tr-sm bg-violet-500 px-4 py-2.5 text-[14px] leading-normal text-white shadow-sm",
          )}
        >
          {isCommand ? (
            <div className="flex items-start gap-2.5">
              <span className="font-bold text-emerald-600 select-none dark:text-emerald-400">
                ❯
              </span>
              <span className="mt-[1px]">{content}</span>
            </div>
          ) : (
            content
          )}
        </div>
      )}
    </div>
  )
}
