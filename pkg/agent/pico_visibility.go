package agent

import (
	"strings"

	"github.com/sipeed/picoclaw/pkg/tools"
)

func picoToolDeliveredVisibleOutput(channel, toolName string, toolResult *tools.ToolResult) bool {
	if !strings.EqualFold(strings.TrimSpace(channel), "pico") || toolResult == nil || toolResult.IsError {
		return false
	}

	// The message tool writes directly to the target chat and reports Silent=true,
	// so Pico should still treat the turn as having produced visible output.
	return toolName == "message"
}

func (al *AgentLoop) picoMessageToolSentToCurrentChat(ts *turnState) bool {
	if ts == nil || !strings.EqualFold(strings.TrimSpace(ts.channel), "pico") {
		return false
	}

	defaultAgent := al.GetRegistry().GetDefaultAgent()
	if defaultAgent == nil {
		return false
	}

	tool, ok := defaultAgent.Tools.Get("message")
	if !ok {
		return false
	}

	mt, ok := tool.(*tools.MessageTool)
	return ok && mt.HasSentTo(ts.sessionKey, ts.channel, ts.chatID)
}
