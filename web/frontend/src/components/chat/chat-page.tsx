import { IconArrowDown, IconPlus } from "@tabler/icons-react"
import { useAtom } from "jotai"
import { useNavigate } from "@tanstack/react-router"
import { type ChangeEvent, useEffect, useMemo, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { AgentSelector } from "@/components/chat/agent-selector"
import { AssistantMessage } from "@/components/chat/assistant-message"
import {
  ChatComposer,
  type ChatInputDisabledReason,
} from "@/components/chat/chat-composer"
import { ChatEmptyState } from "@/components/chat/chat-empty-state"
import { ModelSelector } from "@/components/chat/model-selector"
import { SessionHistoryDropdown } from "@/components/chat/session-history-dropdown"
import { TypingIndicator } from "@/components/chat/typing-indicator"
import { UserMessage } from "@/components/chat/user-message"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { Switch } from "@/components/ui/switch"
import { useChatAgents } from "@/hooks/use-chat-agents"
import { useChatModels } from "@/hooks/use-chat-models"
import { useGateway } from "@/hooks/use-gateway"
import { usePicoChat } from "@/hooks/use-pico-chat"
import { useSessionHistory } from "@/hooks/use-session-history"
import type { ChatAttachment } from "@/store/chat"
import { showThoughtsAtom } from "@/store/chat"

const MAX_IMAGE_SIZE_BYTES = 7 * 1024 * 1024
const MAX_IMAGE_SIZE_LABEL = "7 MB"
const MINIMAP_VISIBLE_THRESHOLD = 12
const ALLOWED_IMAGE_TYPES = new Set([
  "image/jpeg",
  "image/png",
  "image/gif",
  "image/webp",
  "image/bmp",
])

interface MessageMinimapItem {
  id: string
  top: number
  height: number
  label: string
}

function readFileAsDataUrl(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => {
      if (typeof reader.result === "string") {
        resolve(reader.result)
        return
      }
      reject(new Error("Failed to read file"))
    }
    reader.onerror = () =>
      reject(reader.error || new Error("Failed to read file"))
    reader.readAsDataURL(file)
  })
}

function resolveChatInputDisabledReason({
  hasDefaultModel,
  connectionState,
  gatewayState,
}: {
  hasDefaultModel: boolean
  connectionState: "disconnected" | "connecting" | "connected" | "error"
  gatewayState:
    | "unknown"
    | "starting"
    | "running"
    | "restarting"
    | "stopping"
    | "stopped"
    | "error"
}): ChatInputDisabledReason | null {
  if (gatewayState === "unknown") return "gatewayUnknown"
  if (gatewayState === "starting") return "gatewayStarting"
  if (gatewayState === "restarting") return "gatewayRestarting"
  if (gatewayState === "stopping") return "gatewayStopping"
  if (gatewayState === "stopped") return "gatewayStopped"
  if (gatewayState === "error") return "gatewayError"
  if (connectionState === "connecting") return "websocketConnecting"
  if (connectionState === "error") return "websocketError"
  if (connectionState === "disconnected") return "websocketDisconnected"
  if (!hasDefaultModel) return "noDefaultModel"
  return null
}

export function ChatPage() {
  const { t } = useTranslation()
  const navigate = useNavigate({ from: "/" })
  const scrollRef = useRef<HTMLDivElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const messageRefs = useRef<Record<string, HTMLDivElement | null>>({})
  const [isAtBottom, setIsAtBottom] = useState(true)
  const [hasScrolled, setHasScrolled] = useState(false)
  const [input, setInput] = useState("")
  const [attachments, setAttachments] = useState<ChatAttachment[]>([])
  const [showThoughts, setShowThoughts] = useAtom(showThoughtsAtom)
  const [activeViewportMessageId, setActiveViewportMessageId] = useState("")
  const [minimapItems, setMinimapItems] = useState<MessageMinimapItem[]>([])

  const {
    messages,
    connectionState,
    isTyping,
    activeSessionId,
    contextUsage,
    sendMessage,
    switchSession,
    newChat,
  } = usePicoChat()

  const { state: gwState } = useGateway()
  const isGatewayRunning = gwState === "running"
  const isChatConnected = connectionState === "connected"
  const { agents, selectedAgentId, activeAgentId, hasSelectableAgents, handleSelectAgent } =
    useChatAgents(activeSessionId)
  const agentNameById = useMemo(
    () =>
      new Map(
        agents.map((agent) => [agent.id, agent.name?.trim() || agent.id]),
      ),
    [agents],
  )

  const {
    defaultModelName,
    hasAvailableModels,
    apiKeyModels,
    oauthModels,
    localModels,
    handleSetDefault,
    loadError,
  } = useChatModels({
    isConnected: isGatewayRunning,
    activeSessionId,
  })
  const inputDisabledReason = resolveChatInputDisabledReason({
    hasDefaultModel: Boolean(defaultModelName),
    connectionState,
    gatewayState: gwState,  })
  const canSend = isChatConnected && Boolean(defaultModelName) && !isTyping

  const {
    sessions,
    handleDeleteSession,
    handleRenameSession,
  } = useSessionHistory({
    activeSessionId,
    onDeletedActiveSession: newChat,
  })

  const syncScrollState = (element: HTMLDivElement) => {
    const { clientHeight, scrollHeight, scrollTop } = element
    setHasScrolled(scrollTop > 0)
    setIsAtBottom(scrollHeight - scrollTop <= clientHeight + 10)

    const viewportMiddle = scrollTop + clientHeight / 2
    let activeId = ""
    let nearestDistance = Number.POSITIVE_INFINITY
    for (const message of messages) {
      const node = messageRefs.current[message.id]
      if (!node) {
        continue
      }
      const messageMiddle = node.offsetTop + node.offsetHeight / 2
      const distance = Math.abs(messageMiddle - viewportMiddle)
      if (distance < nearestDistance) {
        nearestDistance = distance
        activeId = message.id
      }
    }
    setActiveViewportMessageId(activeId)
  }

  const handleScroll = (e: React.UIEvent<HTMLDivElement>) => {
    syncScrollState(e.currentTarget)
  }

  const scrollToBottom = () => {
    scrollRef.current?.scrollTo({
      top: scrollRef.current.scrollHeight,
      behavior: "smooth",
    })
  }

  useEffect(() => {
    if (scrollRef.current) {
      if (isAtBottom) {
        scrollRef.current.scrollTop = scrollRef.current.scrollHeight
      }
      syncScrollState(scrollRef.current)
    }
  }, [messages, isTyping, isAtBottom])

  useEffect(() => {
    const container = scrollRef.current
    if (!container || messages.length < MINIMAP_VISIBLE_THRESHOLD) {
      setMinimapItems([])
      return
    }

    const updateMinimap = () => {
      const scrollHeight = Math.max(container.scrollHeight, 1)
      setMinimapItems(
        messages
          .map((message) => {
            const node = messageRefs.current[message.id]
            if (!node) {
              return null
            }
            const rawLabel = message.content.trim() || message.role
            return {
              id: message.id,
              top: node.offsetTop / scrollHeight,
              height: Math.max(node.offsetHeight / scrollHeight, 0.025),
              label:
                rawLabel.length > 72
                  ? `${rawLabel.slice(0, 72).trimEnd()}...`
                  : rawLabel,
            }
          })
          .filter((item): item is MessageMinimapItem => item !== null),
      )
      syncScrollState(container)
    }

    updateMinimap()
    const resizeObserver = new ResizeObserver(() => {
      updateMinimap()
    })
    resizeObserver.observe(container)
    for (const message of messages) {
      const node = messageRefs.current[message.id]
      if (node) {
        resizeObserver.observe(node)
      }
    }

    return () => {
      resizeObserver.disconnect()
    }
  }, [messages])

  const handleSend = () => {
    if ((!input.trim() && attachments.length === 0) || !canSend) return
    if (
      sendMessage({
        content: input,
        attachments,
        agentId: activeAgentId || undefined,
      })
    ) {
      setInput("")
      setAttachments([])
    }
  }

  const handleAddImages = () => {
    if (!canSend) return
    fileInputRef.current?.click()
  }

  const handleRemoveAttachment = (index: number) => {
    setAttachments((prev) => prev.filter((_, itemIndex) => itemIndex !== index))
  }

  const handleImageSelection = async (event: ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files ?? [])
    event.target.value = ""

    if (files.length === 0) {
      return
    }

    const nextAttachments: ChatAttachment[] = []
    for (const file of files) {
      if (!ALLOWED_IMAGE_TYPES.has(file.type)) {
        toast.error(
          t("chat.invalidImage", {
            name: file.name,
          }),
        )
        continue
      }

      if (file.size > MAX_IMAGE_SIZE_BYTES) {
        toast.error(
          t("chat.imageTooLarge", {
            name: file.name,
            size: MAX_IMAGE_SIZE_LABEL,
          }),
        )
        continue
      }

      try {
        nextAttachments.push({
          type: "image",
          filename: file.name,
          url: await readFileAsDataUrl(file),
        })
      } catch {
        toast.error(
          t("chat.imageReadFailed", {
            name: file.name,
          }),
        )
      }
    }

    if (nextAttachments.length > 0) {
      setAttachments(nextAttachments.slice(0, 1))
    }
  }

  const hasStreamingAssistant = useMemo(
    () => messages.some((msg) => msg.role === "assistant" && msg.isStreaming),
    [messages],
  )

  const canSubmit = canSend && (Boolean(input.trim()) || attachments.length > 0)

  return (
    <div className="bg-background/95 flex h-full flex-col">
      <PageHeader
        title={t("navigation.chat")}
        className={`transition-shadow ${
          hasScrolled ? "shadow-xs" : "shadow-none"
        }`}
        titleExtra={
          <div className="flex items-center gap-2">
            {hasSelectableAgents && hasAvailableModels && (
              <AgentSelector
                selectedAgentId={selectedAgentId}
                agents={agents}
                onValueChange={handleSelectAgent}
              />
            )}
            {hasAvailableModels && (
              <ModelSelector
                defaultModelName={defaultModelName}
                apiKeyModels={apiKeyModels}
                oauthModels={oauthModels}
                localModels={localModels}
                onValueChange={handleSetDefault}
              />
            )}
          </div>
        }
      >
        <div className="hidden items-center gap-2 rounded-lg border border-border/60 px-3 py-1.5 sm:flex">
          <span className="text-muted-foreground text-sm">
            {t("chat.showThoughts")}
          </span>
          <Switch
            checked={showThoughts}
            onCheckedChange={setShowThoughts}
            aria-label={t("chat.showThoughts")}
            size="sm"
          />
        </div>

        <Button
          variant="secondary"
          size="sm"
          onClick={() => {
            void navigate({
              to: "/",
              search: (prev) => ({ ...prev, history: undefined }),
              replace: true,
            })
            void newChat()
          }}
          className="h-9 gap-2"
        >
          <IconPlus className="size-4" />
          <span className="hidden sm:inline">{t("chat.newChat")}</span>
        </Button>
        <SessionHistoryDropdown
          sessions={sessions}
          activeSessionId={activeSessionId}
          onSwitchSession={(session) => {
            void navigate({
              to: "/",
              search: (prev) => ({ ...prev, history: session.id }),
            })
            void switchSession(session.id, session.session_id, session.agent_id)
          }}
          onRenameSession={handleRenameSession}
          onDeleteSession={(session) => {
            void handleDeleteSession(session)
          }}
        />
      </PageHeader>

      <div className="relative min-h-0 flex-1">
        <div
          ref={scrollRef}
          onScroll={handleScroll}
          className="min-h-0 h-full overflow-y-auto px-4 py-6 [scrollbar-gutter:stable] md:px-8 lg:px-24 xl:px-48"
        >
          <div className="mx-auto flex w-full max-w-250 flex-col gap-8 pb-8">
            {messages.length === 0 && !isTyping && (
              <ChatEmptyState
                hasAvailableModels={hasAvailableModels}
                defaultModelName={defaultModelName}
                isConnected={isGatewayRunning}
                loadError={loadError}
              />
            )}

            {messages.map((msg) => {
              if (msg.kind === "thought" && !showThoughts) {
                return null
              }

              return (
                <div
                  key={msg.id}
                  ref={(node) => {
                    messageRefs.current[msg.id] = node
                  }}
                  className="flex w-full flex-col"
                >
                  {msg.role === "assistant" ? (
                    <AssistantMessage
                      content={msg.content}
                      attachments={msg.attachments}
                      isThought={msg.kind === "thought"}
                      timestamp={msg.timestamp}
                      agentId={msg.agentId}
                      agentName={
                        msg.agentId ? agentNameById.get(msg.agentId) : undefined
                      }
                      modelName={msg.modelName}
                    />
                  ) : (
                    <UserMessage
                      content={msg.content}
                      attachments={msg.attachments}
                    />
                  )}
                </div>
              )
            })}

            {isTyping && !hasStreamingAssistant && <TypingIndicator />}
          </div>
        </div>

        <div className="pointer-events-none absolute inset-y-0 right-3 hidden lg:block">
          {minimapItems.length >= MINIMAP_VISIBLE_THRESHOLD && (
            <div className="bg-background/75 border-border/60 pointer-events-auto absolute top-1/2 right-0 h-56 w-3 -translate-y-1/2 rounded-full border p-0.5 shadow-sm backdrop-blur">
              <div className="relative h-full w-full">
                {minimapItems.map((item) => (
                  <button
                    key={item.id}
                    type="button"
                    title={item.label}
                    aria-label={item.label}
                    className={`absolute left-0 w-full rounded-full transition ${
                      item.id === activeViewportMessageId
                        ? "bg-foreground/80"
                        : "bg-foreground/25 hover:bg-foreground/50"
                    }`}
                    style={{
                      top: `${Math.min(item.top * 100, 97)}%`,
                      height: `${Math.min(item.height * 100, 18)}%`,
                    }}
                    onClick={() => {
                      messageRefs.current[item.id]?.scrollIntoView({
                        behavior: "smooth",
                        block: "center",
                      })
                    }}
                  />
                ))}
              </div>
            </div>
          )}

          <Button
            type="button"
            size="icon"
            variant="secondary"
            className={`border-border/60 text-foreground pointer-events-auto absolute right-0 bottom-4 h-10 w-10 rounded-full border shadow-sm backdrop-blur transition-all duration-300 ${
              isAtBottom
                ? "pointer-events-none translate-y-2 opacity-0"
                : "translate-y-0 opacity-100"
            }`}
            onClick={scrollToBottom}
            aria-label="Scroll to bottom"
            title="Scroll to bottom"
          >
            <IconArrowDown className="size-4" />
          </Button>
        </div>
      </div>

      <input
        ref={fileInputRef}
        type="file"
        accept="image/jpeg,image/png,image/gif,image/webp,image/bmp"
        className="hidden"
        onChange={handleImageSelection}
      />

      <ChatComposer
        input={input}
        attachments={attachments}
        onInputChange={setInput}
        onAddImages={handleAddImages}
        onRemoveAttachment={handleRemoveAttachment}
        onSend={handleSend}
        onContextDetail={() => {
          if (sendMessage({ content: "/context", attachments: [] })) {
            setInput("")
          }
        }}
        inputDisabledReason={inputDisabledReason}
        canSend={canSubmit}
        contextUsage={contextUsage}
      />
    </div>
  )
}
