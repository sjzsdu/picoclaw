import { createFileRoute } from "@tanstack/react-router"

import { AgentDetailPage } from "@/components/agent/detail/agent-detail-page"

export const Route = createFileRoute("/agent/$agentId")({
  component: AgentDetailRoute,
})

function AgentDetailRoute() {
  const { agentId } = Route.useParams()
  return <AgentDetailPage agentId={agentId} />
}
