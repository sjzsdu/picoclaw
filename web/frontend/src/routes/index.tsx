import { createFileRoute } from "@tanstack/react-router"

import { ChatPage } from "@/components/chat/chat-page"

type ChatRouteSearch = {
  history?: string
}

export const Route = createFileRoute("/")({
  validateSearch: (search): ChatRouteSearch => ({
    history:
      typeof search.history === "string" && search.history.trim()
        ? search.history.trim()
        : undefined,
  }),
  component: ChatPage,
})
