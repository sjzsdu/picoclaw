import { createFileRoute } from "@tanstack/react-router"

import { ManagePage } from "@/components/agent/manage/manage-page"

export const Route = createFileRoute("/agent/manage")({
  component: AgentManageRoute,
})

function AgentManageRoute() {
  return <ManagePage />
}
