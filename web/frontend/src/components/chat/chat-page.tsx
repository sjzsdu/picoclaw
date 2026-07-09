import { useAtom } from "jotai"
import { IconArrowDown } from "@tabler/icons-react"
import { useNavigate } from "@tanstack/react-router"
import {
  type ChangeEvent,
  type ClipboardEvent,
  type DragEvent,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react"
import { useTranslation } from "react-i18next"

import { AgentSelector } from "@/components/chat/agent-selector"
import { AssistantMessage } from "@/components/chat/assistant-message"
import {
  ChatComposer,
  type ChatInputDisabledReason,
} from "@/components/chat/chat-composer"
import { ChatEmptyState } from "@/components/chat/chat-empty-state"
import { ChatTabBar } from "@/components/chat/chat-tab-bar"
import { ModelSelector } from "@/components/chat/model-selector"
import { SessionHistoryDropdown } from "@/components/chat/session-history-dropdown"
import { TypingIndicator } from "@/components/chat/typing-indicator"
import { UserMessage } from "@/components/chat/user-message"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  CHAT_IMAGE_ACCEPT,
  buildChatImageAttachments,
  getTransferredFiles,
  hasFileTransfer,
} from "@/features/chat/image-input"
import { useChatAgents } from "@/hooks/use-chat-agents"
import { useChatModels } from "@/hooks/use-chat-models"
import { useGateway } from "@/hooks/use-gateway"
import { usePicoChat } from "@/hooks/use-pico-chat"
import { useSessionHistory } from "@/hooks/use-session-history"
import { getScrollPosition, MAX_TABS } from "@/features/chat/controller"
import type {
  AssistantDetailVisibility,
  ChatAttachment,
} from "@/store/chat"
import {
  assistantDetailVisibilityAtom,
  shouldShowAssistantMessage,
} from "@/store/chat"

const MINIMAP_VISIBLE_THRESHOLD = 12

interface MessageMinimapItem {
  id: string
  top: number
  height: number
  label: string
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
  const dragDepthRef = useRef(0)
  const messageRefs = useRef<Record<string, HTMLDivElement | null>>({})
  const [isAtBottom, setIsAtBottom] = useState(true)
  const [hasScrolled, setHasScrolled] = useState(false)
  const [input, setInput] = useState("")
  const [attachments, setAttachments] = useState<ChatAttachment[]>([])
  const [isDragActive, setIsDragActive] = useState(false)
  const [assistantDetailVisibility, setAssistantDetailVisibility] = useAtom(
    assistantDetailVisibilityAtom,
  )
  const [activeViewportMessageId, setActiveViewportMessageId] = useState("")
  const [minimapItems, setMinimapItems] = useState<MessageMinimapItem[]>([])

  const assistantDetailVisibilityOptions: Array<{
    value: AssistantDetailVisibility
    label: string
  }> = [
    { value: "none", label: t("chat.assistantDetailVisibility.none") },
    { value: "thought", label: t("chat.assistantDetailVisibility.thought") },
    {
      value: "tool_calls",
      label: t("chat.assistantDetailVisibility.toolCalls"),
    },
    { value: "all", label: t("chat.assistantDetailVisibility.all") },
  ]

  const {
    messages,
    connectionState,
    isTyping,
    activeSessionId,
    contextUsage,
    tabs,
    activeTabIndex,
    sendMessage,
    switchSession,
    newChat,
    closeTab: closeChatTab,
    switchToTab,
    reorderTabs,
  } = usePicoChat()

  const { state: gwState } = useGateway()
  const isGatewayRunning = gwState === "running"
  const isChatConnected = connectionState === "connected"
  const {
    agents,
    selectedAgentId,
    hasSelectableAgents,
    handleSelectAgent,
  } = useChatAgents(activeSessionId)
  const agentNameById = useMemo(
    () =>
      new Map(
        agents.map((agent) => [agent.id, agent.name?.trim() || agent.id]),
      ),
    [agents],
  )

  const {
    selectedModelName,
    hasAvailableModels,
    apiKeyModels,
    oauthModels,
    localModels,
    handleSelectModel,
    loadError,
  } = useChatModels({
    isConnected: isGatewayRunning,
    activeSessionId,
  })
  const inputDisabledReason = resolveChatInputDisabledReason({
    hasDefaultModel: Boolean(selectedModelName),
    connectionState,
    gatewayState: gwState,
  })
  const canSend = isChatConnected && Boolean(selectedModelName) && !isTyping

  const { sessions, handleDeleteSession, handleRenameSession } =
    useSessionHistory({
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

  // Track whether user was at bottom to decide auto-scroll vs scroll restore
  const isAtBottomRef = useRef(isAtBottom)
  const prevSessionIdRef = useRef(activeSessionId)
  useEffect(() => {
    isAtBottomRef.current = isAtBottom
  }, [isAtBottom])

  // Combined scroll logic: restore saved position on tab switch,
  // or auto-scroll to bottom when new messages arrive
  useEffect(() => {
    if (!scrollRef.current) return

    const container = scrollRef.current
    const sessionJustChanged = prevSessionIdRef.current !== activeSessionId
    prevSessionIdRef.current = activeSessionId

    const savedScrollTop = getScrollPosition(activeSessionId)

    if (savedScrollTop !== undefined && savedScrollTop > 0) {
      // Tab was switched — restore saved scroll position
      container.scrollTop = savedScrollTop
    } else if (!sessionJustChanged && isAtBottomRef.current) {
      // Same session, user was at bottom — stay at bottom (auto-scroll for new messages)
      container.scrollTop = container.scrollHeight
    }

    syncScrollState(container)
  }, [messages, activeSessionId, isTyping])

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
        agentId: selectedAgentId || undefined,
        modelName: selectedModelName || undefined,
      })
    ) {
      setInput("")
      setAttachments([])
    }
  }

  const handleQuickPrompt = (prompt: string) => {
    if (!canSend) {
      setInput(prompt)
      return
    }
    if (
      sendMessage({
        content: prompt,
        attachments: [],
        agentId: selectedAgentId || undefined,
        modelName: selectedModelName || undefined,
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

  const appendImageFiles = async (files: readonly File[]) => {
    if (!canSend || files.length === 0) {
      return
    }

    const nextAttachments = await buildChatImageAttachments(files, t)
    if (nextAttachments.length === 0) {
      return
    }

    setAttachments((prev) => [...prev, ...nextAttachments])
  }

  const handleImageSelection = async (event: ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files ?? [])
    event.target.value = ""

    if (files.length === 0) {
      return
    }

    await appendImageFiles(files)
  }

  const resetDragState = () => {
    dragDepthRef.current = 0
    setIsDragActive(false)
  }

  const handleComposerPaste = async (
    event: ClipboardEvent<HTMLTextAreaElement>,
  ) => {
    const files = getTransferredFiles(event.clipboardData)
    if (files.length === 0) {
      return
    }

    await appendImageFiles(files)
  }

  const handleComposerDragEnter = (event: DragEvent<HTMLDivElement>) => {
    if (!hasFileTransfer(event.dataTransfer)) {
      return
    }

    event.preventDefault()
    if (!canSend) {
      return
    }
    dragDepthRef.current += 1
    setIsDragActive(true)
  }

  const handleComposerDragLeave = (event: DragEvent<HTMLDivElement>) => {
    if (!hasFileTransfer(event.dataTransfer)) {
      return
    }

    event.preventDefault()
    if (!canSend) {
      resetDragState()
      return
    }
    dragDepthRef.current = Math.max(0, dragDepthRef.current - 1)
    if (dragDepthRef.current === 0) {
      setIsDragActive(false)
    }
  }

  const handleComposerDragOver = (event: DragEvent<HTMLDivElement>) => {
    if (!hasFileTransfer(event.dataTransfer)) {
      return
    }

    event.preventDefault()
    event.dataTransfer.dropEffect = canSend ? "copy" : "none"
  }

  const handleComposerDrop = async (event: DragEvent<HTMLDivElement>) => {
    if (!hasFileTransfer(event.dataTransfer)) {
      return
    }

    event.preventDefault()
    const files = getTransferredFiles(event.dataTransfer)
    resetDragState()

    if (!canSend || files.length === 0) {
      return
    }

    await appendImageFiles(files)
  }

  const hasStreamingAssistant = useMemo(
    () => messages.some((msg) => msg.role === "assistant" && msg.isStreaming),
    [messages],
  )

  const canSubmit = canSend && (Boolean(input.trim()) || attachments.length > 0)

  const handleNewTab = useCallback(() => {
    void navigate({
      to: "/",
      search: (prev) => ({ ...prev, history: undefined }),
      replace: true,
    })
    void newChat()
  }, [navigate, newChat])

  const handleSelectTab = useCallback(
    (index: number) => {
      const tab = tabs[index]
      if (!tab) return
      if (index === activeTabIndex) return
      switchToTab(index)
    },
    [tabs, activeTabIndex, switchToTab],
  )

  const handleCloseTab = useCallback(
    (index: number) => {
      closeChatTab(index)
    },
    [closeChatTab],
  )

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
                selectedModelName={selectedModelName}
                apiKeyModels={apiKeyModels}
                oauthModels={oauthModels}
                localModels={localModels}
                onValueChange={handleSelectModel}
              />
            )}
          </div>
        }
      >
        <div className="border-border/60 hidden items-center gap-2 rounded-lg border px-3 py-1.5 sm:flex">
          <span className="text-muted-foreground text-sm">
            {t("chat.showAssistantDetails")}
          </span>
          <Select
            value={assistantDetailVisibility}
            onValueChange={(value) =>
              setAssistantDetailVisibility(value as AssistantDetailVisibility)
            }
          >
            <SelectTrigger
              size="sm"
              aria-label={t("chat.showAssistantDetails")}
              className="text-muted-foreground hover:text-foreground focus-visible:border-input h-8 min-w-[104px] bg-transparent shadow-none focus-visible:ring-0"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent align="end">
              {assistantDetailVisibilityOptions.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <SessionHistoryDropdown
          sessions={sessions}
          activeSessionId={activeSessionId}
          onSwitchSession={(session) => {
            void navigate({
              to: "/",
              search: (prev) => ({ ...prev, history: session.id }),
            })
            void switchSession(session.id, session.title)
          }}
          onRenameSession={handleRenameSession}
          onDeleteSession={(session) => {
            void handleDeleteSession(session)
          }}
        />
      </PageHeader>

      <ChatTabBar
        tabs={tabs}
        activeTabIndex={activeTabIndex}
        onSelectTab={handleSelectTab}
        onCloseTab={handleCloseTab}
        onNewTab={handleNewTab}
        onReorderTabs={reorderTabs}
        maxTabsReached={tabs.length >= MAX_TABS}
      />

      <div className="relative min-h-0 flex-1">
        <div
          ref={scrollRef}
          onScroll={handleScroll}
          className="h-full min-h-0 overflow-y-auto px-4 py-6 [scrollbar-gutter:stable] md:px-8 lg:px-24 xl:px-48"
        >
          <div className="mx-auto flex w-full max-w-250 flex-col gap-8 pb-8">
            {messages.length === 0 && !isTyping && (
              <ChatEmptyState
                hasAvailableModels={hasAvailableModels}
                defaultModelName={selectedModelName}
                isConnected={isGatewayRunning}
                loadError={loadError}
              />
            )}

            {messages.map((msg) => {
              if (
                !shouldShowAssistantMessage(assistantDetailVisibility, msg.kind)
              ) {
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
                      kind={msg.kind}
                      modelName={msg.modelName}
                      toolCalls={msg.toolCalls}
                      timestamp={msg.timestamp}
                      agentId={msg.agentId}
                      agentName={
                        msg.agentId ? agentNameById.get(msg.agentId) : undefined
                      }
                      onQuickPrompt={handleQuickPrompt}
                      isStreaming={msg.isStreaming}
                    />
                  ) : (
                    <UserMessage
                      content={msg.content}
                      attachments={msg.attachments}
                      timestamp={msg.timestamp}
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
        accept={CHAT_IMAGE_ACCEPT}
        multiple
        className="hidden"
        onChange={handleImageSelection}
      />

      <ChatComposer
        input={input}
        attachments={attachments}
        onInputChange={setInput}
        onAddImages={handleAddImages}
        onPaste={handleComposerPaste}
        onDragEnter={handleComposerDragEnter}
        onDragLeave={handleComposerDragLeave}
        onDragOver={handleComposerDragOver}
        onDrop={handleComposerDrop}
        onRemoveAttachment={handleRemoveAttachment}
        onSend={handleSend}
        onContextDetail={() => {
          if (
            sendMessage({
              content: "/context",
              attachments: [],
              modelName: selectedModelName || undefined,
            })
          ) {
            setInput("")
          }
        }}
        inputDisabledReason={inputDisabledReason}
        canSend={canSubmit}
        isDragActive={isDragActive}
        contextUsage={contextUsage}
      />
    </div>
  )
}
