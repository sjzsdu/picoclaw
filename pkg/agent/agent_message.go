// PicoClaw - Ultra-lightweight personal AI agent

package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/constants"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/routing"
	"github.com/sipeed/picoclaw/pkg/session"
	"github.com/sipeed/picoclaw/pkg/utils"
)

func defaultResponseForChannel(channel string) string {
	if strings.EqualFold(strings.TrimSpace(channel), "pico") {
		return ""
	}
	return defaultResponse
}

func (al *AgentLoop) buildContinuationTarget(msg bus.InboundMessage) (*continuationTarget, error) {
	if msg.Channel == "system" {
		return nil, nil
	}

	route, _, err := al.resolveMessageRoute(msg)
	if err != nil {
		return nil, err
	}
	allocation := al.allocateRouteSession(route, msg)

	return &continuationTarget{
		SessionKey: resolveScopeKey(allocation.SessionKey, msg.SessionKey),
		Channel:    msg.Channel,
		ChatID:     msg.ChatID,
	}, nil
}

func (al *AgentLoop) ProcessDirect(
	ctx context.Context,
	content, sessionKey string,
) (string, error) {
	return al.ProcessDirectWithChannel(ctx, content, sessionKey, "cli", "direct")
}

func (al *AgentLoop) ProcessDirectWithChannel(
	ctx context.Context,
	content, sessionKey, channel, chatID string,
) (string, error) {
	if err := al.ensureHooksInitialized(ctx); err != nil {
		return "", err
	}
	if err := al.ensureMCPInitialized(ctx); err != nil {
		return "", err
	}

	msg := bus.InboundMessage{
		Context: bus.InboundContext{
			Channel:  channel,
			ChatID:   chatID,
			ChatType: "direct",
			SenderID: "cron",
		},
		Content:    content,
		SessionKey: sessionKey,
	}

	return al.processMessage(ctx, msg)
}

func (al *AgentLoop) ProcessDirectForAgent(
	ctx context.Context,
	content, sessionKey, agentID string,
) (string, error) {
	return al.ProcessDirectWithTarget(ctx, content, sessionKey, agentID, "cli", "direct")
}

func (al *AgentLoop) ProcessDirectWithTarget(
	ctx context.Context,
	content, sessionKey, agentID, channel, chatID string,
) (string, error) {
	if err := al.ensureHooksInitialized(ctx); err != nil {
		return "", err
	}
	if err := al.ensureMCPInitialized(ctx); err != nil {
		return "", err
	}

	raw := map[string]string{}
	if trimmed := strings.TrimSpace(agentID); trimmed != "" {
		raw["agent_id"] = trimmed
	}

	msg := bus.InboundMessage{
		Context: bus.InboundContext{
			Channel:  channel,
			ChatID:   chatID,
			ChatType: "direct",
			SenderID: "cron",
			Raw:      raw,
		},
		Content:    content,
		SessionKey: sessionKey,
	}

	return al.processMessage(ctx, msg)
}

func (al *AgentLoop) ProcessHeartbeat(
	ctx context.Context,
	content, channel, chatID string,
) (string, error) {
	if err := al.ensureHooksInitialized(ctx); err != nil {
		return "", err
	}
	if err := al.ensureMCPInitialized(ctx); err != nil {
		return "", err
	}

	agent := al.GetRegistry().GetDefaultAgent()
	if agent == nil {
		return "", fmt.Errorf("no default agent for heartbeat")
	}
	dispatch := DispatchRequest{
		SessionKey:  "heartbeat",
		UserMessage: content,
	}
	if channel != "" || chatID != "" {
		dispatch.InboundContext = &bus.InboundContext{
			Channel:  channel,
			ChatID:   chatID,
			ChatType: "direct",
			SenderID: "heartbeat",
		}
	}
	return al.runAgentLoop(ctx, agent, processOptions{
		Dispatch:               dispatch,
		DefaultResponse:        defaultResponseForChannel(channel),
		EnableSummary:          false,
		SendResponse:           false,
		SuppressToolFeedback:   true,
		NoHistory:              true, // Don't load session history for heartbeat
		SkipSessionPersistence: true,
	})
}

func (al *AgentLoop) prepareInboundMessageForAgent(
	ctx context.Context,
	msg bus.InboundMessage,
) bus.InboundMessage {
	msg = bus.NormalizeInboundMessage(msg)

	var hadAudio bool
	msg, hadAudio = al.transcribeAudioInMessage(ctx, msg)

	// For audio messages the placeholder was deferred by the channel.
	// Now that transcription (and optional feedback) is done, send it.
	if hadAudio && al.channelManager != nil {
		al.channelManager.SendPlaceholder(ctx, msg.Channel, msg.ChatID)
	}

	return msg
}

func (al *AgentLoop) processMessage(ctx context.Context, msg bus.InboundMessage) (string, error) {
	msg = al.prepareInboundMessageForAgent(ctx, msg)

	// Add message preview to log (show full content for error messages)
	var logContent string
	if strings.Contains(msg.Content, "Error:") || strings.Contains(msg.Content, "error") {
		logContent = msg.Content // Full content for errors
	} else {
		logContent = utils.Truncate(msg.Content, 80)
	}
	logger.InfoCF(
		"agent",
		fmt.Sprintf("Processing message from %s:%s: %s", msg.Channel, msg.SenderID, logContent),
		map[string]any{
			"channel":     msg.Channel,
			"chat_id":     msg.ChatID,
			"sender_id":   msg.SenderID,
			"session_key": msg.SessionKey,
		},
	)

	// Route system messages to processSystemMessage
	if msg.Channel == "system" {
		return al.processSystemMessage(ctx, msg)
	}

	route, agent, routeErr := al.resolveMessageRoute(msg)
	if routeErr != nil {
		return "", routeErr
	}

	forcedAgentID := strings.TrimSpace(msg.Context.Raw["agent_id"])

	// CLI/direct channel messages must bypass routing and use the default agent,
	// unless the caller explicitly forced a target agent.
	if forcedAgentID == "" && (msg.Context.Channel == "cli" || msg.Context.Channel == "direct") {
		if def := al.GetRegistry().GetDefaultAgent(); def != nil {
			agent = def
			// Ensure the route snapshot reflects the default agent as well.
			route.AgentID = def.ID
		}
	}

	allocation := al.allocateRouteSession(route, msg)

	// Resolve session key from the route allocation, while preserving explicit
	// agent-scoped keys supplied by the caller.
	scopeKey := resolveScopeKey(allocation.SessionKey, msg.SessionKey)
	sessionKey := scopeKey

	// Reset message-tool state for this round so we don't skip publishing due to a previous round.
	if tool, ok := agent.Tools.Get("message"); ok {
		if resetter, ok := tool.(interface{ ResetSentInRound(sessionKey string) }); ok {
			resetter.ResetSentInRound(sessionKey)
		}
	}

	logger.InfoCF("agent", "Routed message",
		map[string]any{
			"agent_id":           agent.ID,
			"model_name":         agent.Model,
			"scope_key":          scopeKey,
			"session_key":        sessionKey,
			"matched_by":         route.MatchedBy,
			"route_agent":        route.AgentID,
			"route_channel":      route.Channel,
			"route_main_session": allocation.MainSessionKey,
		})

	if err := al.applyInboundModelOverride(agent, msg.Context.Raw["model_name"]); err != nil {
		logger.WarnCF("agent", "Ignoring invalid inbound model override",
			map[string]any{
				"agent_id":   agent.ID,
				"model_name": strings.TrimSpace(msg.Context.Raw["model_name"]),
				"error":      err.Error(),
			})
	}

	opts := processOptions{
		Dispatch: DispatchRequest{
			SessionKey:     sessionKey,
			SessionAliases: buildSessionAliases(sessionKey, append(allocation.SessionAliases, msg.SessionKey)...),
			InboundContext: cloneInboundContext(&msg.Context),
			RouteResult:    cloneResolvedRoute(&route),
			SessionScope:   session.CloneScope(&allocation.Scope),
			UserMessage:    msg.Content,
			Media:          append([]string(nil), msg.Media...),
		},
		SenderID:                msg.SenderID,
		SenderDisplayName:       msg.Sender.DisplayName,
		DefaultResponse:         defaultResponseForChannel(msg.Channel),
		EnableSummary:           true,
		SendResponse:            false,
		AllowInterimPicoPublish: true,
		NoHistory:               agent.NoHistory,
	}
	var err error
	opts, err = resolveTurnProfileOptions(al.GetConfig(), opts)
	if err != nil {
		return "", err
	}

	// context-dependent commands check their own Runtime fields and report
	// "unavailable" when the required capability is nil.
	if response, handled := al.handleCommand(ctx, msg, agent, &opts); handled {
		return response, nil
	}

	if pending := al.takePendingSkills(opts.Dispatch.SessionKey); len(pending) > 0 {
		opts.ForcedSkills = append(opts.ForcedSkills, pending...)
		logger.InfoCF("agent", "Applying pending skill override",
			map[string]any{
				"session_key": opts.Dispatch.SessionKey,
				"skills":      strings.Join(pending, ","),
			})
	}

	return al.runAgentLoop(ctx, agent, opts)
}

func (al *AgentLoop) applyInboundModelOverride(agent *AgentInstance, modelName string) error {
	if agent == nil {
		return nil
	}
	modelName = strings.TrimSpace(modelName)
	if modelName == "" || modelName == agent.Model {
		return nil
	}

	modelCfg, err := resolvedModelConfig(al.cfg, modelName, agent.Workspace)
	if err != nil {
		return err
	}

	provider, resolvedModel, err := al.providerFactory(modelCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize model %q: %w", modelName, err)
	}
	if strings.TrimSpace(resolvedModel) == "" {
		resolvedModel = modelName
	}

	candidates := resolveModelCandidates(al.cfg, al.cfg.Agents.Defaults.Provider, modelName, agent.Fallbacks)
	if len(candidates) == 0 {
		return fmt.Errorf("model %q did not resolve to any provider candidates", modelName)
	}

	oldProvider := agent.Provider
	agent.Model = modelName
	agent.Provider = provider
	agent.Candidates = candidates
	agent.ThinkingLevel = parseThinkingLevel(modelCfg.ThinkingLevel)

	if oldProvider != nil && oldProvider != provider {
		if stateful, ok := oldProvider.(providers.StatefulProvider); ok {
			stateful.Close()
		}
	}
	logger.InfoCF("agent", "Applied inbound model override",
		map[string]any{
			"agent_id":       agent.ID,
			"model_name":     modelName,
			"resolved_model": resolvedModel,
		})
	return nil
}

func (al *AgentLoop) resolveMessageRoute(msg bus.InboundMessage) (routing.ResolvedRoute, *AgentInstance, error) {
	registry := al.GetRegistry()
	inboundCtx := normalizedInboundContext(msg)
	route := registry.ResolveRoute(inboundCtx)

	agent, ok := registry.GetAgent(route.AgentID)
	if !ok {
		agent = registry.GetDefaultAgent()
	}
	if agent == nil {
		return routing.ResolvedRoute{}, nil, fmt.Errorf("no agent available for route (agent_id=%s)", route.AgentID)
	}

	return route, agent, nil
}

func (al *AgentLoop) allocateRouteSession(route routing.ResolvedRoute, msg bus.InboundMessage) session.Allocation {
	return session.AllocateRouteSession(session.AllocationInput{
		AgentID:       route.AgentID,
		Context:       normalizedInboundContext(msg),
		SessionPolicy: route.SessionPolicy,
	})
}

func (al *AgentLoop) processSystemMessage(
	ctx context.Context,
	msg bus.InboundMessage,
) (string, error) {
	if msg.Channel != "system" {
		return "", fmt.Errorf(
			"processSystemMessage called with non-system message channel: %s",
			msg.Channel,
		)
	}

	logger.InfoCF("agent", "Processing system message",
		map[string]any{
			"sender_id": msg.SenderID,
			"chat_id":   msg.ChatID,
		})

	var originChannel, originChatID string
	if idx := strings.Index(msg.ChatID, ":"); idx > 0 {
		originChannel = msg.ChatID[:idx]
		originChatID = msg.ChatID[idx+1:]
	} else {
		originChannel = "cli"
		originChatID = msg.ChatID
	}

	content := msg.Content
	if idx := strings.Index(content, "Result:\n"); idx >= 0 {
		content = content[idx+8:]
	}

	if constants.IsInternalChannel(originChannel) {
		logger.InfoCF("agent", "Subagent completed (internal channel)",
			map[string]any{
				"sender_id":   msg.SenderID,
				"content_len": len(content),
				"channel":     originChannel,
			})
		return "", nil
	}

	agent := al.GetRegistry().GetDefaultAgent()
	if agent == nil {
		return "", fmt.Errorf("no default agent for system message")
	}

	sessionKey := session.BuildMainSessionKey(agent.ID)
	dispatch := DispatchRequest{
		SessionKey:  sessionKey,
		UserMessage: fmt.Sprintf("[System: %s] %s", msg.SenderID, msg.Content),
	}
	if originChannel != "" || originChatID != "" {
		dispatch.InboundContext = &bus.InboundContext{
			Channel:  originChannel,
			ChatID:   originChatID,
			ChatType: "direct",
			SenderID: msg.SenderID,
		}
	}

	return al.runAgentLoop(ctx, agent, processOptions{
		Dispatch:        dispatch,
		DefaultResponse: "Background task completed.",
		EnableSummary:   false,
		SendResponse:    true,
	})
}
