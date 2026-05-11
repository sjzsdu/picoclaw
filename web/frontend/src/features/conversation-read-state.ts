const READ_STATE_KEY_PREFIX = "picoclaw:conversation-last-read:"

type ConversationKind = "chat" | "group"

function readStateKey(kind: ConversationKind, id: string) {
  return `${READ_STATE_KEY_PREFIX}${kind}:${id}`
}

export function getConversationLastReadAt(
  kind: ConversationKind,
  id: string,
): number {
  if (!id) {
    return 0
  }
  const raw = globalThis.localStorage?.getItem(readStateKey(kind, id)) || ""
  const parsed = Number(raw)
  return Number.isFinite(parsed) ? parsed : 0
}

export function setConversationLastReadAt(
  kind: ConversationKind,
  id: string,
  timestamp: number,
) {
  if (!id || !Number.isFinite(timestamp)) {
    return
  }
  globalThis.localStorage?.setItem(readStateKey(kind, id), String(timestamp))
}
