import { getDefaultStore } from "jotai"
import { toast } from "sonner"

import { type SessionHistoryError } from "@/api/sessions"
import {
  type SessionMessagesResult,
  loadSessionMessages,
  mergeHistoryMessages,
} from "@/features/chat/history"
import {
  CHAT_SESSION_TITLE_EVENT,
  CHAT_SESSION_USER_MESSAGE_EVENT,
  type ChatSessionTitleDetail,
  type ChatSessionUserMessageDetail,
  type PicoMessage,
  handlePicoMessage,
  notifySessionActivity,
  notifySessionUserMessage,
} from "@/features/chat/protocol"
import {
  clearStoredSessionId,
  generateSessionId,
  readStoredSessionId,
  writeStoredSessionId,
} from "@/features/chat/state"
import { invalidateSocket, isCurrentSocket } from "@/features/chat/websocket"
import i18n from "@/i18n"
import {
  type ChatAttachment,
  type ChatMessage,
  type TabState,
  getChatState,
  updateChatStore,
} from "@/store/chat"
import { type GatewayState, gatewayAtom } from "@/store/gateway"

const store = getDefaultStore()
const HYDRATE_RETRY_DELAYS_MS = [300, 700, 1500, 2500]

let wsRef: WebSocket | null = null
let isConnecting = false
let msgIdCounter = 0
let activeSessionIdRef = getChatState().activeSessionId
let initialized = false
let unsubscribeGateway: (() => void) | null = null
let hydratePromise: Promise<void> | null = null
let connectionGeneration = 0
let reconnectTimer: number | null = null
let reconnectAttempts = 0
let shouldMaintainConnection = false
let scrollPositionsRef = new Map<string, number>()

export const MAX_TABS = 10

function clearReconnectTimer() {
  if (reconnectTimer !== null) {
    window.clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
}

function persistActiveSessionId(sessionId: string) {
  writeStoredSessionId(sessionId)
}

function shouldReconnectFor(generation: number, sessionId: string): boolean {
  return (
    shouldMaintainConnection &&
    generation === connectionGeneration &&
    sessionId === activeSessionIdRef &&
    store.get(gatewayAtom).status === "running"
  )
}

function scheduleReconnect(generation: number, sessionId: string) {
  if (!shouldReconnectFor(generation, sessionId) || reconnectTimer !== null) {
    return
  }

  const delay = Math.min(1000 * 2 ** reconnectAttempts, 5000)
  reconnectAttempts += 1
  reconnectTimer = window.setTimeout(() => {
    reconnectTimer = null
    if (!shouldReconnectFor(generation, sessionId)) {
      return
    }
    void connectChat()
  }, delay)
}

function needsActiveSessionHydration(): boolean {
  const state = getChatState()
  const storedSessionId = readStoredSessionId()

  return Boolean(
    storedSessionId &&
    storedSessionId === state.activeSessionId &&
    !state.hasHydratedActiveSession,
  )
}

function isMissingStoredSessionError(
  error: unknown,
): error is SessionHistoryError {
  return (
    typeof error === "object" &&
    error !== null &&
    "status" in error &&
    (error as { status?: unknown }).status === 404
  )
}

function clearRestoreFailureForStoredSession() {
  clearStoredSessionId()
}

async function loadSessionMessagesWithRetry(sessionId: string) {
  let lastError: unknown
  for (
    let attempt = 0;
    attempt <= HYDRATE_RETRY_DELAYS_MS.length;
    attempt += 1
  ) {
    try {
      return await loadSessionMessages(sessionId)
    } catch (error) {
      lastError = error
      if (
        !isMissingStoredSessionError(error) ||
        attempt >= HYDRATE_RETRY_DELAYS_MS.length
      ) {
        throw error
      }
      await new Promise((resolve) => {
        window.setTimeout(resolve, HYDRATE_RETRY_DELAYS_MS[attempt])
      })
    }
  }
  throw lastError
}

function handleSessionTitleEvent(event: Event) {
  const detail = (event as CustomEvent<ChatSessionTitleDetail>).detail
  if (!detail?.sessionId) {
    return
  }

  const title = detail.title.trim()
  updateChatStore((prev) => {
    const index = prev.tabs.findIndex((t) => t.sessionId === detail.sessionId)
    if (index < 0 || prev.tabs[index].title === (title || undefined)) {
      return prev
    }
    return {
      tabs: prev.tabs.map((t, i) =>
        i === index ? { ...t, title: title || undefined } : t,
      ),
    }
  })
}

function handleCrossTabUserMessage(event: Event) {
  const detail = (event as CustomEvent<ChatSessionUserMessageDetail>).detail
  if (!detail?.sessionId || detail.sessionId !== activeSessionIdRef) {
    return
  }

  updateChatStore((prev) => {
    if (prev.messages.some((message) => message.id === detail.message.id)) {
      return prev
    }
    return {
      messages: [...prev.messages, detail.message],
    }
  })
}

function disconnectChatInternal({
  clearDesiredConnection,
}: {
  clearDesiredConnection: boolean
}) {
  connectionGeneration += 1
  clearReconnectTimer()

  if (clearDesiredConnection) {
    shouldMaintainConnection = false
  }

  const socket = wsRef
  wsRef = null
  isConnecting = false

  invalidateSocket(socket)

  updateChatStore({
    connectionState: "disconnected",
    isTyping: false,
  })
}

export async function connectChat() {
  if (
    store.get(gatewayAtom).status !== "running" ||
    needsActiveSessionHydration()
  ) {
    return
  }

  if (
    isConnecting ||
    (wsRef &&
      (wsRef.readyState === WebSocket.OPEN ||
        wsRef.readyState === WebSocket.CONNECTING))
  ) {
    return
  }

  const generation = connectionGeneration + 1
  connectionGeneration = generation
  isConnecting = true
  clearReconnectTimer()
  updateChatStore({ connectionState: "connecting" })

  try {
    const sessionId = activeSessionIdRef

    if (generation !== connectionGeneration) {
      isConnecting = false
      return
    }

    const wsScheme = window.location.protocol === "https:" ? "wss:" : "ws:"
    const wsUrl = `${wsScheme}//${window.location.host}/pico/ws`
    const url = `${wsUrl}?session_id=${encodeURIComponent(sessionId)}`
    const socket = new WebSocket(url)

    if (generation !== connectionGeneration) {
      isConnecting = false
      invalidateSocket(socket)
      return
    }

    socket.onopen = () => {
      if (
        !isCurrentSocket({
          socket,
          currentSocket: wsRef,
          generation,
          currentGeneration: connectionGeneration,
          sessionId,
          currentSessionId: activeSessionIdRef,
        })
      ) {
        return
      }
      updateChatStore({ connectionState: "connected" })
      isConnecting = false
      reconnectAttempts = 0
    }

    socket.onmessage = (event) => {
      if (
        !isCurrentSocket({
          socket,
          currentSocket: wsRef,
          generation,
          currentGeneration: connectionGeneration,
          sessionId,
          currentSessionId: activeSessionIdRef,
        })
      ) {
        return
      }

      try {
        const message = JSON.parse(event.data) as PicoMessage
        handlePicoMessage(message, sessionId)
      } catch {
        console.warn("Non-JSON message from pico:", event.data)
      }
    }

    socket.onclose = () => {
      if (
        !isCurrentSocket({
          socket,
          currentSocket: wsRef,
          generation,
          currentGeneration: connectionGeneration,
          sessionId,
          currentSessionId: activeSessionIdRef,
        })
      ) {
        return
      }
      wsRef = null
      isConnecting = false
      updateChatStore({
        connectionState: "disconnected",
        isTyping: false,
      })
      scheduleReconnect(generation, sessionId)
    }

    socket.onerror = () => {
      if (
        !isCurrentSocket({
          socket,
          currentSocket: wsRef,
          generation,
          currentGeneration: connectionGeneration,
          sessionId,
          currentSessionId: activeSessionIdRef,
        })
      ) {
        return
      }
      isConnecting = false
      updateChatStore({ connectionState: "error" })
      scheduleReconnect(generation, sessionId)
    }

    wsRef = socket
  } catch (error) {
    if (generation !== connectionGeneration) {
      isConnecting = false
      return
    }
    console.error("Failed to connect to pico:", error)
    updateChatStore({ connectionState: "error" })
    isConnecting = false
    scheduleReconnect(generation, activeSessionIdRef)
  }
}

export function disconnectChat() {
  disconnectChatInternal({ clearDesiredConnection: true })
}

export async function openChatHistory(sessionId: string, title?: string) {
  if (sessionId === activeSessionIdRef) {
    return
  }
  // Check if this session is already open in a tab
  const state = getChatState()
  const existingTabIndex = state.tabs.findIndex(
    (t) => t.sessionId === sessionId,
  )
  if (existingTabIndex >= 0) {
    switchToTab(existingTabIndex)
    return
  }
  // Otherwise, load and open in a new tab
  await switchChatSession(sessionId, title)
}

async function hydrateInitialTab() {
  const state = getChatState()
  const storedSessionId = readStoredSessionId()

  if (
    !storedSessionId ||
    state.hasHydratedActiveSession
  ) {
    if (!state.hasHydratedActiveSession) {
      updateChatStore({ hasHydratedActiveSession: true })
    }
    return
  }

  // Find the tab with the stored session ID
  const tabIndex = state.tabs.findIndex(
    (t) => t.sessionId === storedSessionId && !t.hasHydrated,
  )
  if (tabIndex < 0) {
    updateChatStore({ hasHydratedActiveSession: true })
    return
  }

  hydratePromise = loadSessionMessagesWithRetry(storedSessionId)
    .then(({ messages: historyMessages }) => {
      const currentState = getChatState()
      const currentTabIndex = currentState.tabs.findIndex(
        (t) => t.sessionId === storedSessionId,
      )
      if (currentTabIndex < 0) {
        return
      }
      const currentTab = currentState.tabs[currentTabIndex]

      if (currentTab.messages.length > 0) {
        const mergedMessages = mergeHistoryMessages(
          historyMessages,
          currentTab.messages,
        )
        if (currentTabIndex === currentState.activeTabIndex) {
          updateChatStore({
            messages: mergedMessages,
            hasHydratedActiveSession: true,
          })
        } else {
          updateChatStore({
            tabs: currentState.tabs.map((t, i) =>
              i === currentTabIndex
                ? { ...t, messages: mergedMessages, hasHydrated: true }
                : t,
            ),
            hasHydratedActiveSession: true,
          })
        }
        return
      }

      if (currentTabIndex === currentState.activeTabIndex) {
        updateChatStore({
          messages: historyMessages,
          isTyping: false,
          hasHydratedActiveSession: true,
        })
      } else {
        updateChatStore({
          tabs: currentState.tabs.map((t, i) =>
            i === currentTabIndex
              ? { ...t, messages: historyMessages, isTyping: false, hasHydrated: true }
              : t,
          ),
          hasHydratedActiveSession: true,
        })
      }
    })
    .catch((error) => {
      if (isMissingStoredSessionError(error)) {
        console.info(
          "Stored chat session no longer exists; starting a new session instead.",
          error.sessionId,
        )
      } else {
        console.error("Failed to restore last session history:", error)
      }

      const currentState = getChatState()
      const currentTabIndex = currentState.tabs.findIndex(
        (t) => t.sessionId === storedSessionId,
      )
      if (currentTabIndex < 0) {
        return
      }
      const currentTab = currentState.tabs[currentTabIndex]

      if (currentTab.messages.length > 0) {
        updateChatStore({
          tabs: currentState.tabs.map((t, i) =>
            i === currentTabIndex ? { ...t, hasHydrated: true } : t,
          ),
          hasHydratedActiveSession: true,
        })
        return
      }

      const nextSessionId = generateSessionId()
      clearRestoreFailureForStoredSession()
      const newTab: TabState = {
        sessionId: nextSessionId,
        messages: [],
        isTyping: false,
        hasHydrated: true,
      }
      const newTabs = currentState.tabs.map((t, i) =>
        i === currentTabIndex ? newTab : t,
      )
      if (currentTabIndex === currentState.activeTabIndex) {
        activeSessionIdRef = nextSessionId
        updateChatStore({
          tabs: newTabs,
          activeSessionId: nextSessionId,
          messages: [],
          isTyping: false,
          hasHydratedActiveSession: true,
          contextUsage: undefined,
        })
      } else {
        updateChatStore({
          tabs: newTabs,
          hasHydratedActiveSession: true,
        })
      }
    })
    .finally(() => {
      hydratePromise = null
    })

  return hydratePromise
}

export async function hydrateActiveSession() {
  return hydrateInitialTab()
}

interface SendChatMessageInput {
  content: string
  attachments?: ChatAttachment[]
  agentId?: string
  modelName?: string
}

export function sendChatMessage({
  content,
  attachments = [],
  agentId,
  modelName,
}: SendChatMessageInput) {
  if (!wsRef || wsRef.readyState !== WebSocket.OPEN) {
    console.warn("WebSocket not connected")
    return false
  }

  const normalizedContent = content.trim()
  const normalizedAttachments = attachments
    .filter((attachment) => attachment.type === "image" && attachment.url)
    .map((attachment) => ({ ...attachment }))

  if (!normalizedContent && normalizedAttachments.length === 0) {
    return false
  }

  const socket = wsRef
  const id = `msg-${++msgIdCounter}-${Date.now()}`

  const userMessage = {
    id,
    role: "user" as const,
    content: normalizedContent,
    attachments:
      normalizedAttachments.length > 0 ? normalizedAttachments : undefined,
    timestamp: Date.now(),
  }

  updateChatStore((prev) => ({
    messages: [...prev.messages, userMessage],
    isTyping: true,
  }))

  try {
    persistActiveSessionId(activeSessionIdRef)
    socket.send(
      JSON.stringify({
        type: "message.send",
        id,
        session_id: activeSessionIdRef,
        payload: {
          content: normalizedContent,
          ...(agentId ? { agent_id: agentId } : {}),
          ...(modelName ? { model_name: modelName } : {}),
          media: normalizedAttachments.map((attachment) => attachment.url),
        },
      }),
    )
    notifySessionActivity({
      sessionId: activeSessionIdRef,
      preview:
        normalizedContent ||
        (normalizedAttachments.length > 0 ? "[image]" : ""),
      timestamp: new Date().toISOString(),
    })
    notifySessionUserMessage({
      sessionId: activeSessionIdRef,
      message: userMessage,
    })
    return true
  } catch (error) {
    console.error("Failed to send pico message:", error)
    updateChatStore((prev) => ({
      messages: prev.messages.filter((message) => message.id !== id),
      isTyping: false,
    }))
    return false
  }
}

/**
 * Switch to an existing tab by index.
 */
export function switchToTab(tabIndex: number) {
  const state = getChatState()
  if (
    tabIndex === state.activeTabIndex ||
    tabIndex < 0 ||
    tabIndex >= state.tabs.length
  ) {
    return
  }

  const targetTab = state.tabs[tabIndex]
  if (!targetTab) return

  // Save current scroll position
  saveScrollPosition()

  disconnectChatInternal({ clearDesiredConnection: false })
  activeSessionIdRef = targetTab.sessionId
  persistActiveSessionId(targetTab.sessionId)
  updateChatStore({
    activeTabIndex: tabIndex,
    activeSessionId: targetTab.sessionId,
    messages: targetTab.messages,
    isTyping: targetTab.isTyping,
    contextUsage: targetTab.contextUsage,
  })

  if (store.get(gatewayAtom).status === "running") {
    shouldMaintainConnection = true
    void connectChat()
  }

  // Background refresh: the tab may have received messages while it was
  // inactive (its WebSocket was disconnected). Re-fetch history from the
  // backend and merge so the user sees the latest without a manual refresh.
  void refreshTabFromHistory(targetTab.sessionId)
}

/**
 * Re-fetch a session's history from the backend and merge it into the matching
 * tab. Used when switching back to a tab whose WebSocket was disconnected while
 * inactive, so newly-arrived messages appear without a manual page refresh.
 *
 * Guards against races: the result is only applied to the tab that still has
 * the same sessionId, and top-level fields are synced only when that tab is
 * still the active one.
 */
async function refreshTabFromHistory(sessionId: string) {
  let result: SessionMessagesResult
  try {
    result = await loadSessionMessages(sessionId)
  } catch (error) {
    if (!isMissingStoredSessionError(error)) {
      console.error("Failed to refresh tab history:", error)
    }
    return
  }

  const { messages: historyMessages, title } = result

  updateChatStore((prev) => {
    const tabIndex = prev.tabs.findIndex((t) => t.sessionId === sessionId)
    if (tabIndex < 0) {
      return prev
    }

    const tab = prev.tabs[tabIndex]
    const mergedMessages = mergeHistoryMessages(historyMessages, tab.messages)
    const normalizedTitle = title?.trim() || undefined
    const messagesUnchanged = chatMessagesEqual(mergedMessages, tab.messages)
    const titleUnchanged = tab.title === normalizedTitle

    if (messagesUnchanged && titleUnchanged) {
      // Nothing changed — avoid unnecessary re-render.
      return prev
    }

    const updatedTab: TabState = {
      ...tab,
      messages: mergedMessages,
      ...(normalizedTitle !== undefined ? { title: normalizedTitle } : {}),
    }

    const newTabs = prev.tabs.map((t, i) =>
      i === tabIndex ? updatedTab : t,
    )

    if (tabIndex === prev.activeTabIndex) {
      return { ...prev, tabs: newTabs, messages: mergedMessages }
    }
    return { ...prev, tabs: newTabs }
  })
}

/**
 * Shallow comparison of two message lists by id and content, used to skip
 * unnecessary store updates after a history refresh.
 */
function chatMessagesEqual(a: ChatMessage[], b: ChatMessage[]): boolean {
  if (a.length !== b.length) {
    return false
  }
  for (let i = 0; i < a.length; i += 1) {
    if (a[i].id !== b[i].id || a[i].content !== b[i].content) {
      return false
    }
  }
  return true
}

/**
 * Close a tab by index. The last remaining tab cannot be closed.
 */
export function closeTab(tabIndex: number) {
  const state = getChatState()
  if (state.tabs.length <= 1) {
    return
  }
  if (tabIndex < 0 || tabIndex >= state.tabs.length) {
    return
  }

  const wasActive = tabIndex === state.activeTabIndex

  const tabToClose = state.tabs[tabIndex]

  const newTabs = [...state.tabs]
  newTabs.splice(tabIndex, 1)

  // Determine new active index
  let newActiveIndex = state.activeTabIndex
  if (wasActive) {
    newActiveIndex = Math.min(tabIndex, newTabs.length - 1)
  } else if (tabIndex < state.activeTabIndex) {
    newActiveIndex -= 1
  }

  const newActiveTab = newTabs[newActiveIndex]

  if (wasActive) {
    // Save scroll position for the tab being closed, then switch to new active tab
    scrollPositionsRef.delete(tabToClose.sessionId)
    disconnectChatInternal({ clearDesiredConnection: false })
    activeSessionIdRef = newActiveTab.sessionId
    persistActiveSessionId(newActiveTab.sessionId)
    updateChatStore({
      tabs: newTabs,
      activeTabIndex: newActiveIndex,
      activeSessionId: newActiveTab.sessionId,
      messages: newActiveTab.messages,
      isTyping: newActiveTab.isTyping,
      contextUsage: newActiveTab.contextUsage,
    })

    if (store.get(gatewayAtom).status === "running") {
      shouldMaintainConnection = true
      void connectChat()
    }
  } else {
    // Remove scroll position for the non-active tab being closed
    scrollPositionsRef.delete(tabToClose.sessionId)
    // Adjust activeTabIndex if needed
    updateChatStore({
      tabs: newTabs,
      activeTabIndex: newActiveIndex,
    })
  }
}

export async function switchChatSession(sessionId: string, title?: string) {
  if (sessionId === activeSessionIdRef) {
    return
  }

  // Check if already open in a tab
  const state = getChatState()
  const existingTabIndex = state.tabs.findIndex(
    (t) => t.sessionId === sessionId,
  )
  if (existingTabIndex >= 0) {
    switchToTab(existingTabIndex)
    return
  }

  if (state.tabs.length >= MAX_TABS) {
    toast.error(i18n.t("chat.tabsLimitReached", { max: MAX_TABS }))
    return
  }

  try {
    const { messages: historyMessages, title: serverTitle } =
      await loadSessionMessagesWithRetry(sessionId)

    const normalizedTitle = (title ?? serverTitle)?.trim()
    const newTab: TabState = {
      sessionId,
      messages: historyMessages,
      isTyping: false,
      hasHydrated: true,
      ...(normalizedTitle ? { title: normalizedTitle } : {}),
    }

    disconnectChatInternal({ clearDesiredConnection: false })
    activeSessionIdRef = sessionId
    persistActiveSessionId(sessionId)
    updateChatStore((prev) => ({
      tabs: [...prev.tabs, newTab],
      activeTabIndex: prev.tabs.length,
      activeSessionId: sessionId,
      messages: historyMessages,
      isTyping: false,
      hasHydratedActiveSession: true,
      contextUsage: undefined,
    }))

    if (store.get(gatewayAtom).status === "running") {
      shouldMaintainConnection = true
      await connectChat()
    }
  } catch (error) {
    console.error("Failed to load session history:", error)
    toast.error(i18n.t("chat.historyOpenFailed"))
  }
}

export async function newChatSession() {
  const state = getChatState()
  if (state.tabs.length >= MAX_TABS) {
    toast.error(i18n.t("chat.tabsLimitReached", { max: MAX_TABS }))
    return
  }

  // Always create a new tab, even if current tab is empty
  const newId = generateSessionId()
  const newTab: TabState = {
    sessionId: newId,
    messages: [],
    isTyping: false,
    hasHydrated: true,
  }

  disconnectChatInternal({ clearDesiredConnection: false })
  activeSessionIdRef = newId
  persistActiveSessionId(newId)
  updateChatStore((prev) => ({
    tabs: [...prev.tabs, newTab],
    activeTabIndex: prev.tabs.length,
    activeSessionId: newId,
    messages: [],
    isTyping: false,
    hasHydratedActiveSession: true,
    contextUsage: undefined,
  }))

  if (store.get(gatewayAtom).status === "running") {
    shouldMaintainConnection = true
    await connectChat()
  }
}

/**
 * Reorder tabs by moving the tab at `fromIndex` to `toIndex`.
 */
export function reorderTabs(fromIndex: number, toIndex: number) {
  if (fromIndex === toIndex) return

  const state = getChatState()
  if (
    fromIndex < 0 || fromIndex >= state.tabs.length ||
    toIndex < 0 || toIndex >= state.tabs.length
  ) {
    return
  }

  const newTabs = [...state.tabs]
  const [movedTab] = newTabs.splice(fromIndex, 1)
  newTabs.splice(toIndex, 0, movedTab)

  let newActiveIndex = state.activeTabIndex
  if (fromIndex === state.activeTabIndex) {
    // The active tab was moved
    newActiveIndex = toIndex
  } else if (fromIndex < state.activeTabIndex && toIndex >= state.activeTabIndex) {
    // A tab before the active tab was moved to after it
    newActiveIndex -= 1
  } else if (fromIndex > state.activeTabIndex && toIndex <= state.activeTabIndex) {
    // A tab after the active tab was moved to before it
    newActiveIndex += 1
  }

  // Also reorder scroll positions to match new tab order
  const newScrollPositions = new Map<string, number>()
  for (const tab of newTabs) {
    const saved = scrollPositionsRef.get(tab.sessionId)
    if (saved !== undefined) {
      newScrollPositions.set(tab.sessionId, saved)
    }
  }
  scrollPositionsRef = newScrollPositions

  updateChatStore({
    tabs: newTabs,
    activeTabIndex: newActiveIndex,
  })
}

/**
 * Save current scroll position for the active tab.
 */
export function saveScrollPosition(scrollTop?: number) {
  const state = getChatState()
  const tab = state.tabs[state.activeTabIndex]
  if (tab) {
    scrollPositionsRef.set(tab.sessionId, scrollTop ?? 0)
  }
}

/**
 * Get saved scroll position for the given session.
 */
export function getScrollPosition(sessionId: string): number | undefined {
  return scrollPositionsRef.get(sessionId)
}

export function initializeChatStore() {
  if (initialized) {
    return
  }

  initialized = true
  activeSessionIdRef = getChatState().activeSessionId
  let lastGatewayStatus: GatewayState | null = null

  const syncConnectionWithGateway = (force: boolean = false) => {
    const gatewayStatus = store.get(gatewayAtom).status
    if (!force && gatewayStatus === lastGatewayStatus) {
      return
    }
    lastGatewayStatus = gatewayStatus

    if (gatewayStatus === "running") {
      shouldMaintainConnection = true
      if (needsActiveSessionHydration()) {
        return
      }
      void connectChat()
      return
    }

    if (gatewayStatus === "stopped" || gatewayStatus === "error") {
      disconnectChatInternal({ clearDesiredConnection: true })
    }
  }

  unsubscribeGateway = store.sub(gatewayAtom, syncConnectionWithGateway)
  window.addEventListener(
    CHAT_SESSION_USER_MESSAGE_EVENT,
    handleCrossTabUserMessage as EventListener,
  )
  window.addEventListener(
    CHAT_SESSION_TITLE_EVENT,
    handleSessionTitleEvent as EventListener,
  )

  const storedSessionId = readStoredSessionId()
  if (!storedSessionId) {
    updateChatStore((prev) => ({
      hasHydratedActiveSession: true,
      tabs: prev.tabs.map((t) =>
        t.sessionId === prev.activeSessionId
          ? { ...t, hasHydrated: true }
          : t,
      ),
    }))
    syncConnectionWithGateway(true)
    return
  }

  void hydrateActiveSession().finally(() => {
    if (!initialized) {
      return
    }
    syncConnectionWithGateway(true)
  })
}

export function teardownChatStore() {
  window.removeEventListener(
    CHAT_SESSION_USER_MESSAGE_EVENT,
    handleCrossTabUserMessage as EventListener,
  )
  window.removeEventListener(
    CHAT_SESSION_TITLE_EVENT,
    handleSessionTitleEvent as EventListener,
  )
  unsubscribeGateway?.()
  unsubscribeGateway = null
  initialized = false
  disconnectChat()
}
