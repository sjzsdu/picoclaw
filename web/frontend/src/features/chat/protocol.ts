import { toast } from "sonner"

import {
  parseAssistantMessageCreateState,
  parseAssistantMessageUpdateState,
} from "@/features/chat/assistant-message-state"
import { normalizeUnixTimestamp } from "@/features/chat/state"
import {
  type AssistantMessageKind,
  type ChatAttachment,
  type ChatMessage,
  type ContextUsage,
  updateChatStore,
} from "@/store/chat"

export const CHAT_SESSION_ACTIVITY_EVENT = "picoclaw:chat-session-activity"
export const CHAT_SESSION_USER_MESSAGE_EVENT =
  "picoclaw:chat-session-user-message"
export const CHAT_SESSION_TITLE_EVENT = "picoclaw:chat-session-title"
const CHAT_CROSS_TAB_CHANNEL = "picoclaw:chat-cross-tab"
const CHAT_TAB_ID = `${Date.now()}-${Math.random().toString(36).slice(2)}`

type CrossTabMessage =
  | {
      source: string
      type: typeof CHAT_SESSION_ACTIVITY_EVENT
      detail: ChatSessionActivityDetail
    }
  | {
      source: string
      type: typeof CHAT_SESSION_USER_MESSAGE_EVENT
      detail: ChatSessionUserMessageDetail
    }
  | {
      source: string
      type: typeof CHAT_SESSION_TITLE_EVENT
      detail: ChatSessionTitleDetail
    }

export interface ChatSessionActivityDetail {
  sessionId: string
  preview?: string
  timestamp?: string
}

export interface ChatSessionUserMessageDetail {
  sessionId: string
  message: ChatMessage
}

export interface ChatSessionTitleDetail {
  sessionId: string
  title: string
}

const crossTabChannel =
  typeof globalThis.BroadcastChannel === "function"
    ? new BroadcastChannel(CHAT_CROSS_TAB_CHANNEL)
    : null

function dispatchChatEvent<T>(type: string, detail: T) {
  globalThis.window?.dispatchEvent(new CustomEvent(type, { detail }))
}

function broadcastChatEvent(message: Omit<CrossTabMessage, "source">) {
  crossTabChannel?.postMessage({ ...message, source: CHAT_TAB_ID })
}

crossTabChannel?.addEventListener(
  "message",
  (event: MessageEvent<CrossTabMessage>) => {
    const data = event.data
    if (!data || data.source === CHAT_TAB_ID) {
      return
    }
    dispatchChatEvent(data.type, data.detail)
  },
)

export function notifySessionActivity(detail: ChatSessionActivityDetail) {
  if (!detail.sessionId) {
    return
  }
  dispatchChatEvent(CHAT_SESSION_ACTIVITY_EVENT, detail)
  broadcastChatEvent({ type: CHAT_SESSION_ACTIVITY_EVENT, detail })
}

export function notifySessionUserMessage(detail: ChatSessionUserMessageDetail) {
  if (!detail.sessionId || detail.message.role !== "user") {
    return
  }
  dispatchChatEvent(CHAT_SESSION_USER_MESSAGE_EVENT, detail)
  broadcastChatEvent({ type: CHAT_SESSION_USER_MESSAGE_EVENT, detail })
}

export function notifySessionTitle(detail: ChatSessionTitleDetail) {
  if (!detail.sessionId) {
    return
  }
  dispatchChatEvent(CHAT_SESSION_TITLE_EVENT, detail)
  broadcastChatEvent({ type: CHAT_SESSION_TITLE_EVENT, detail })
}

export interface PicoMessage {
  type: string
  id?: string
  session_id?: string
  timestamp?: number | string
  payload?: Record<string, unknown>
}

function parseAttachments(
  payload: Record<string, unknown>,
): ChatAttachment[] | undefined {
  const raw = payload.attachments
  if (!Array.isArray(raw)) {
    return undefined
  }

  const attachments: ChatAttachment[] = []
  for (const item of raw) {
    if (!item || typeof item !== "object") {
      continue
    }

    const attachment = item as Record<string, unknown>
    const url = typeof attachment.url === "string" ? attachment.url : ""
    if (!url) {
      continue
    }

    const type =
      attachment.type === "audio" ||
      attachment.type === "video" ||
      attachment.type === "file" ||
      attachment.type === "image"
        ? attachment.type
        : "file"

    const filename =
      typeof attachment.filename === "string" ? attachment.filename : undefined
    const contentType =
      typeof attachment.content_type === "string"
        ? attachment.content_type
        : undefined

    attachments.push({
      type,
      url,
      ...(filename ? { filename } : {}),
      ...(contentType ? { contentType } : {}),
    })
  }

  return attachments.length > 0 ? attachments : undefined
}

function parseContextUsage(
  payload: Record<string, unknown>,
): ContextUsage | undefined {
  const raw = payload.context_usage
  if (!raw || typeof raw !== "object") return undefined
  const obj = raw as Record<string, unknown>
  const used = Number(obj.used_tokens)
  const total = Number(obj.total_tokens)
  if (!Number.isFinite(used) || !Number.isFinite(total) || total <= 0)
    return undefined
  return {
    used_tokens: used,
    total_tokens: total,
    history_tokens: obj.history_tokens != null ? Number(obj.history_tokens) : undefined,
    compress_at_tokens: Number(obj.compress_at_tokens) || 0,
    summarize_at_tokens: obj.summarize_at_tokens != null ? Number(obj.summarize_at_tokens) : undefined,
    used_percent: Number(obj.used_percent) || 0,
  }
}

function parseModelName(payload: Record<string, unknown>): string | undefined {
  if (typeof payload.model_name !== "string") {
    return undefined
  }
  const modelName = payload.model_name.trim()
  return modelName || undefined
}

function isToolFeedbackMessage(message: ChatMessage): boolean {
  if (message.role !== "assistant") {
    return false
  }

  const firstLine = message.content.split("\n", 1)[0]?.trim() ?? ""
  return /^🔧\s+`[^`]+`/.test(firstLine)
}

function findToolFeedbackMessageIndex(messages: ChatMessage[]): number {
  let lastUserIndex = -1
  for (let i = messages.length - 1; i >= 0; i -= 1) {
    if (messages[i].role === "user") {
      lastUserIndex = i
      break
    }
  }

  for (let i = messages.length - 1; i >= 0; i -= 1) {
    if (i <= lastUserIndex) {
      break
    }
    if (isToolFeedbackMessage(messages[i])) {
      return i
    }
  }
  return -1
}

function assistantMessageMatchesIncoming(
  message: {
    role: "user" | "assistant"
    content: string
    kind?: AssistantMessageKind
    agentId?: string
    modelName?: string
    toolCalls?: ChatMessage["toolCalls"]
  },
  incoming: {
    content: string
    kind: AssistantMessageKind
    agentId?: string
    modelName?: string
    toolCalls?: ChatMessage["toolCalls"]
  },
) {
  return (
    message.role === "assistant" &&
    message.content === incoming.content &&
    (message.kind ?? "normal") === incoming.kind &&
    (message.agentId ?? "") === (incoming.agentId ?? "") &&
    (message.modelName ?? "") === (incoming.modelName ?? "") &&
    JSON.stringify(message.toolCalls ?? []) ===
      JSON.stringify(incoming.toolCalls ?? [])
  )
}
export function handlePicoMessage(
  message: PicoMessage,
  expectedSessionId: string,
) {
  if (message.session_id && message.session_id !== expectedSessionId) {
    return
  }

  const payload = message.payload || {}

  switch (message.type) {
    case "message.create":
    case "media.create": {
      const messageId = (payload.message_id as string) || `pico-${Date.now()}`
      const { content, kind, toolCalls } =
        parseAssistantMessageCreateState(payload)
      const attachments = parseAttachments(payload)
      const contextUsage = parseContextUsage(payload)
      const isPlaceholder = payload.placeholder === true
      const agentId =
        typeof payload.agent_id === "string" ? payload.agent_id : undefined
      const modelName = parseModelName(payload)
      const timestamp =
        message.timestamp !== undefined &&
        Number.isFinite(Number(message.timestamp))
          ? normalizeUnixTimestamp(Number(message.timestamp))
          : Date.now()

      updateChatStore((prev) => ({
        messages: (() => {
          const existingMessageIndex = prev.messages.findIndex(
            (msg) => msg.id === messageId,
          )
          if (existingMessageIndex >= 0) {
            return prev.messages.map((msg, index) =>
              index === existingMessageIndex
                ? {
                    ...msg,
                    id: messageId,
                    role: "assistant" as const,
                    content,
                    kind,
                    ...(toolCalls ? { toolCalls } : {}),
                    ...(attachments ? { attachments } : {}),
                    ...(agentId ? { agentId } : {}),
                    ...(modelName ? { modelName } : {}),
                  }
                : msg,
            )
          }

          const alreadyExists = prev.messages.some((msg) =>
            assistantMessageMatchesIncoming(msg, {
              content,
              kind,
              agentId,
              modelName,
              toolCalls,
            }),
          )
          if (alreadyExists) {
            return prev.messages
          }

          return [
            ...prev.messages,
            {
              id: messageId,
              role: "assistant",
              content,
              kind,
              ...(toolCalls ? { toolCalls } : {}),
              attachments,
              timestamp,
              ...(agentId ? { agentId } : {}),
              ...(modelName ? { modelName } : {}),
            },
          ]
        })(),
        isTyping:
          !isPlaceholder &&
          (kind === "normal" || message.type === "media.create")
            ? false
            : prev.isTyping,
        ...(contextUsage ? { contextUsage } : {}),
      }))
      notifySessionActivity({ sessionId: expectedSessionId, preview: content })
      break
    }

    case "message.update": {
      const messageId = payload.message_id as string
      const previewContent =
        typeof payload.content === "string" ? payload.content : ""
      const attachments = parseAttachments(payload)
      const contextUsage = parseContextUsage(payload)
      const modelName = parseModelName(payload)
      const timestamp =
        message.timestamp !== undefined &&
        Number.isFinite(Number(message.timestamp))
          ? normalizeUnixTimestamp(Number(message.timestamp))
          : Date.now()
      if (!messageId) {
        break
      }

      updateChatStore((prev) => ({
        messages: (() => {
          let found = false
          const messages = prev.messages.map((msg) => {
            if (msg.id !== messageId) {
              return msg
            }
            found = true
            const { content, kind, toolCalls } =
              parseAssistantMessageUpdateState(payload, msg)
            const hasKind = kind !== (msg.kind ?? "normal") ||
              payload.kind !== undefined ||
              payload.tool_calls !== undefined
            return {
              ...msg,
              id: messageId,
              content,
              ...(hasKind ? { kind } : {}),
              ...(toolCalls !== undefined ? { toolCalls } : {}),
              agentId:
                typeof payload.agent_id === "string"
                  ? payload.agent_id
                  : msg.agentId,
              modelName:
                parseModelName(payload) ?? msg.modelName,
              ...(attachments ? { attachments } : {}),
            }
          })
          if (found) {
            return messages
          }

          const { content, kind, toolCalls } =
            parseAssistantMessageUpdateState(payload)
          const hasKind = payload.kind !== undefined || payload.tool_calls !== undefined

          const fallbackIndex = findToolFeedbackMessageIndex(messages)
          if (fallbackIndex >= 0) {
            return messages.map((msg, index) =>
              index === fallbackIndex
                ? {
                    ...msg,
                    id: messageId,
                    content,
                    ...(hasKind ? { kind } : {}),
                    ...(toolCalls !== undefined ? { toolCalls } : {}),
                    agentId:
                      typeof payload.agent_id === "string"
                        ? payload.agent_id
                        : msg.agentId,
                    modelName:
                      typeof payload.model_name === "string"
                        ? payload.model_name
                        : msg.modelName,
                    ...(attachments ? { attachments } : {}),
                  }
                : msg,
            )
          }

          return [
            ...messages,
            {
              id: messageId,
              role: "assistant" as const,
              content,
              ...(hasKind ? { kind } : {}),
              ...(toolCalls ? { toolCalls } : {}),
              agentId:
                typeof payload.agent_id === "string"
                  ? payload.agent_id
                  : undefined,
              ...(modelName ? { modelName } : {}),
              ...(attachments ? { attachments } : {}),
              timestamp,
            },
          ]
        })(),
        ...(contextUsage ? { contextUsage } : {}),
      }))
      notifySessionActivity({
        sessionId: expectedSessionId,
        preview: previewContent,
      })
      break
    }

    case "message.delete": {
      const messageId = payload.message_id as string
      if (!messageId) {
        break
      }

      updateChatStore((prev) => ({
        messages: (() => {
          const exactMessages = prev.messages.filter((msg) => msg.id !== messageId)
          if (exactMessages.length !== prev.messages.length) {
            return exactMessages
          }

          const fallbackIndex = findToolFeedbackMessageIndex(prev.messages)
          if (fallbackIndex < 0) {
            return prev.messages
          }

          return prev.messages.filter((_, index) => index !== fallbackIndex)
        })(),
      }))
      notifySessionActivity({ sessionId: expectedSessionId })
      break
    }

    case "typing.start":
      updateChatStore({ isTyping: true })
      notifySessionActivity({ sessionId: expectedSessionId })
      break

    case "typing.stop":
      updateChatStore({ isTyping: false })
      break

    case "error": {
      const requestId =
        typeof payload.request_id === "string" ? payload.request_id : ""
      const errorMessage =
        typeof payload.message === "string" ? payload.message : ""

      console.error("Pico error:", payload)
      if (errorMessage) {
        toast.error(errorMessage)
      }
      updateChatStore((prev) => ({
        messages: requestId
          ? prev.messages.filter((msg) => msg.id !== requestId)
          : prev.messages,
        isTyping: false,
      }))
      break
    }

    case "pong":
      break

    default:
      console.log("Unknown pico message type:", message.type)
  }
}
