import { atom, getDefaultStore } from "jotai"
import { atomWithStorage } from "jotai/utils"

import {
  ASSISTANT_DETAIL_VISIBILITY_STORAGE_KEY,
  type AssistantDetailVisibility,
  DEFAULT_ASSISTANT_DETAIL_VISIBILITY,
  assistantDetailVisibilityStorage,
  shouldShowAssistantMessage,
} from "@/features/chat/detail-visibility"
import { getInitialActiveSessionId } from "@/features/chat/state"

export interface ChatAttachment {
  type: "image" | "audio" | "video" | "file"
  url: string
  filename?: string
  contentType?: string
}

export interface ChatToolCallFunction {
  name?: string
  arguments?: string
}

export interface ChatToolCallExtraContent {
  toolFeedbackExplanation?: string
}

export interface ChatToolCall {
  id?: string
  type?: string
  function?: ChatToolCallFunction
  extraContent?: ChatToolCallExtraContent
}

export type AssistantMessageKind = "normal" | "thought" | "tool_calls"

export type MessageDeliveryStatus = "sending" | "sent" | "delivered" | "failed"

export interface ChatMessage {
  id: string
  role: "user" | "assistant"
  content: string
  timestamp: number | string
  reasoningContent?: string
  kind?: AssistantMessageKind
  modelName?: string
  attachments?: ChatAttachment[]
  toolCalls?: ChatToolCall[]
  agentId?: string
  deliveryStatus?: MessageDeliveryStatus
  isStreaming?: boolean
}

export interface ContextUsage {
  used_tokens: number
  total_tokens: number
  history_tokens?: number
  compress_at_tokens: number
  summarize_at_tokens?: number
  used_percent: number
}

export type ConnectionState =
  | "disconnected"
  | "connecting"
  | "connected"
  | "error"

export interface TabState {
  sessionId: string
  messages: ChatMessage[]
  isTyping: boolean
  contextUsage?: ContextUsage
  hasHydrated: boolean
  title?: string
}

export interface ChatStoreState {
  messages: ChatMessage[]
  connectionState: ConnectionState
  isTyping: boolean
  activeSessionId: string
  hasHydratedActiveSession: boolean
  contextUsage?: ContextUsage
  /** Ordered list of open tabs */
  tabs: TabState[]
  /** Index of the currently active tab */
  activeTabIndex: number
}

type ChatStorePatch = Partial<ChatStoreState>

function buildInitialTabs(): TabState[] {
  const sessionId = getInitialActiveSessionId()
  return [
    {
      sessionId,
      messages: [],
      isTyping: false,
      hasHydrated: false,
    },
  ]
}

const DEFAULT_CHAT_STATE: ChatStoreState = {
  messages: [],
  connectionState: "disconnected",
  isTyping: false,
  activeSessionId: getInitialActiveSessionId(),
  hasHydratedActiveSession: false,
  tabs: buildInitialTabs(),
  activeTabIndex: 0,
}

export const chatAtom = atom<ChatStoreState>(DEFAULT_CHAT_STATE)
export const assistantDetailVisibilityAtom =
  atomWithStorage<AssistantDetailVisibility>(
    ASSISTANT_DETAIL_VISIBILITY_STORAGE_KEY,
    DEFAULT_ASSISTANT_DETAIL_VISIBILITY,
    assistantDetailVisibilityStorage,
    { getOnInit: true },
  )
export const showAssistantDetailsAtom = atom(
  (get) => get(assistantDetailVisibilityAtom) !== "none",
)

const store = getDefaultStore()

export function getChatState() {
  return store.get(chatAtom)
}

export function updateChatStore(
  patch:
    | ChatStorePatch
    | ((prev: ChatStoreState) => ChatStorePatch | ChatStoreState),
) {
  store.set(chatAtom, (prev) => {
    const nextPatch = typeof patch === "function" ? patch(prev) : patch
    const result = { ...prev, ...nextPatch }

    // Sync legacy fields (messages, isTyping, contextUsage, activeSessionId)
    // to the active tab when tabs are present
    if (result.tabs.length > 0 && result.activeTabIndex < result.tabs.length) {
      const activeTab = result.tabs[result.activeTabIndex]
      let tabNeedsUpdate = false

      if ("messages" in nextPatch && result.messages !== activeTab.messages) {
        tabNeedsUpdate = true
      }
      if ("isTyping" in nextPatch && result.isTyping !== activeTab.isTyping) {
        tabNeedsUpdate = true
      }
      if ("contextUsage" in nextPatch && result.contextUsage !== activeTab.contextUsage) {
        tabNeedsUpdate = true
      }

      if (tabNeedsUpdate) {
        const updatedTab = {
          ...activeTab,
          ...("messages" in nextPatch ? { messages: result.messages } : {}),
          ...("isTyping" in nextPatch ? { isTyping: result.isTyping } : {}),
          ...("contextUsage" in nextPatch ? { contextUsage: result.contextUsage } : {}),
        }
        const newTabs = [...result.tabs]
        newTabs[result.activeTabIndex] = updatedTab
        result.tabs = newTabs
      }
    }

    return result
  })
}

/**
 * Update only the active tab's state without touching the top-level
 * messages/isTyping/contextUsage. Used by the tab system and protocol handler.
 */
export function updateActiveTabState(
  patch:
    | Partial<TabState>
    | ((prev: TabState) => Partial<TabState>),
) {
  store.set(chatAtom, (prev) => {
    const tabIndex = prev.activeTabIndex
    if (tabIndex < 0 || tabIndex >= prev.tabs.length) {
      return prev
    }
    const tab = { ...prev.tabs[tabIndex] }
    const nextPatch = typeof patch === "function" ? patch(tab) : patch
    const updatedTab = { ...tab, ...nextPatch }
    const newTabs = [...prev.tabs]
    newTabs[tabIndex] = updatedTab

    // Sync to top-level fields for backward compatibility,
    // but only if the relevant fields actually changed
    const result: Partial<ChatStoreState> = { tabs: newTabs }
    if ("messages" in nextPatch && updatedTab.messages !== prev.messages) {
      result.messages = updatedTab.messages
    }
    if ("isTyping" in nextPatch && updatedTab.isTyping !== prev.isTyping) {
      result.isTyping = updatedTab.isTyping
    }
    if ("contextUsage" in nextPatch && updatedTab.contextUsage !== prev.contextUsage) {
      result.contextUsage = updatedTab.contextUsage
    }
    if ("sessionId" in nextPatch && updatedTab.sessionId !== prev.activeSessionId) {
      result.activeSessionId = updatedTab.sessionId
    }

    return { ...prev, ...result }
  })
}

/** Get the active tab's state directly */
export function getActiveTabState(): TabState | undefined {
  const state = getChatState()
  if (state.activeTabIndex < 0 || state.activeTabIndex >= state.tabs.length) {
    return undefined
  }
  return state.tabs[state.activeTabIndex]
}

export {
  shouldShowAssistantMessage,
  DEFAULT_ASSISTANT_DETAIL_VISIBILITY,
}
export type { AssistantDetailVisibility }
