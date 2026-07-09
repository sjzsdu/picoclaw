import { useTranslation } from "react-i18next"

import type { AgentInfo } from "@/api/agents"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

interface AgentSelectorProps {
  selectedAgentId: string
  agents: AgentInfo[]
  onValueChange: (agentId: string) => void
}

export function AgentSelector({
  selectedAgentId,
  agents,
  onValueChange,
}: AgentSelectorProps) {
  const { t } = useTranslation()

  return (
    <Select value={selectedAgentId} onValueChange={onValueChange}>
      <SelectTrigger
        size="sm"
        className="text-muted-foreground hover:text-foreground focus-visible:border-input h-8 max-w-[180px] min-w-[96px] bg-transparent shadow-none focus-visible:ring-0 sm:max-w-[240px]"
      >
        <SelectValue placeholder={t("chat.noAgent")} />
      </SelectTrigger>
      <SelectContent position="popper" align="start">
        {agents.map((agent) => (
          <SelectItem key={agent.id} value={agent.id}>
            {agent.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
