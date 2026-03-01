package qq

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/tencent-connect/botgo"
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/event"
	"github.com/tencent-connect/botgo/openapi"
	"github.com/tencent-connect/botgo/token"
	"golang.org/x/oauth2"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/identity"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/utils"
)

type QQChannel struct {
	*channels.BaseChannel
	config         config.QQConfig
	api            openapi.OpenAPI
	tokenSource    oauth2.TokenSource
	ctx            context.Context
	cancel         context.CancelFunc
	sessionManager botgo.SessionManager
	processedIDs   map[string]bool
	mu             sync.RWMutex
}

func NewQQChannel(cfg config.QQConfig, messageBus *bus.MessageBus) (*QQChannel, error) {
	base := channels.NewBaseChannel("qq", cfg, messageBus, cfg.AllowFrom,
		channels.WithGroupTrigger(cfg.GroupTrigger),
		channels.WithReasoningChannelID(cfg.ReasoningChannelID),
	)

	return &QQChannel{
		BaseChannel:  base,
		config:       cfg,
		processedIDs: make(map[string]bool),
	}, nil
}

func (c *QQChannel) Start(ctx context.Context) error {
	if c.config.AppID == "" || c.config.AppSecret == "" {
		return fmt.Errorf("QQ app_id and app_secret not configured")
	}

	logger.InfoC("qq", "Starting QQ bot (WebSocket mode)")

	credentials := &token.QQBotCredentials{
		AppID:     c.config.AppID,
		AppSecret: c.config.AppSecret,
	}
	c.tokenSource = token.NewQQBotTokenSource(credentials)

	c.ctx, c.cancel = context.WithCancel(ctx)

	if err := token.StartRefreshAccessToken(c.ctx, c.tokenSource); err != nil {
		return fmt.Errorf("failed to start token refresh: %w", err)
	}

	c.api = botgo.NewOpenAPI(c.config.AppID, c.tokenSource).WithTimeout(5 * time.Second)

	intent := event.RegisterHandlers(
		c.handleC2CMessage(),
		c.handleGroupATMessage(),
	)

	wsInfo, err := c.api.WS(c.ctx, nil, "")
	if err != nil {
		return fmt.Errorf("failed to get websocket info: %w", err)
	}

	logger.InfoCF("qq", "Got WebSocket info", map[string]any{
		"shards": wsInfo.Shards,
	})

	c.sessionManager = botgo.NewSessionManager()

	go func() {
		if err := c.sessionManager.Start(wsInfo, c.tokenSource, &intent); err != nil {
			logger.ErrorCF("qq", "WebSocket session error", map[string]any{
				"error": err.Error(),
			})
			c.SetRunning(false)
		}
	}()

	c.SetRunning(true)
	logger.InfoC("qq", "QQ bot started successfully")

	return nil
}

func (c *QQChannel) Stop(ctx context.Context) error {
	logger.InfoC("qq", "Stopping QQ bot")
	c.SetRunning(false)

	if c.cancel != nil {
		c.cancel()
	}

	return nil
}

func (c *QQChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	if !c.IsRunning() {
		return channels.ErrNotRunning
	}

	msgToCreate := &dto.MessageToCreate{
		Content: msg.Content,
	}

	_, err := c.api.PostC2CMessage(ctx, msg.ChatID, msgToCreate)
	if err != nil {
		logger.ErrorCF("qq", "Failed to send C2C message", map[string]any{
			"error": err.Error(),
		})
		return fmt.Errorf("qq send: %w", channels.ErrTemporary)
	}

	return nil
}

type AudioFile struct {
	URL      string
	Filename string
}

func (c *QQChannel) extractAudioFiles(data *dto.WSC2CMessageData) []AudioFile {
	var audioFiles []AudioFile

	for _, att := range data.Attachments {
		if att.ContentType == "voice" || isAudioFile(att.FileName) {
			audioFiles = append(audioFiles, AudioFile{
				URL:      att.URL,
				Filename: att.FileName,
			})
			logger.DebugCF("qq", "Extracted audio file from C2C message", map[string]any{
				"filename": att.FileName,
				"size":     att.Size,
			})
		}
	}

	return audioFiles
}

func isAudioFile(filename string) bool {
	extensions := []string{".amr", ".silk", ".mp3", ".wav", ".ogg", ".m4a"}
	for _, ext := range extensions {
		if len(filename) >= len(ext) && filename[len(filename)-len(ext):] == ext {
			return true
		}
	}
	return false
}

func (c *QQChannel) extractGroupAudioFiles(data *dto.WSGroupATMessageData) []AudioFile {
	var audioFiles []AudioFile

	for _, att := range data.Attachments {
		if att.ContentType == "voice" || isAudioFile(att.FileName) {
			audioFiles = append(audioFiles, AudioFile{
				URL:      att.URL,
				Filename: att.FileName,
			})
			logger.DebugCF("qq", "Extracted audio file from group message", map[string]any{
				"filename": att.FileName,
				"size":     att.Size,
			})
		}
	}

	return audioFiles
}

func (c *QQChannel) downloadAttachment(url, filename string) string {
	if url == "" || filename == "" {
		return ""
	}

	logger.DebugCF("qq", "Downloading attachment", map[string]any{
		"url":      url,
		"filename": filename,
	})

	return utils.DownloadFile(url, filename, utils.DownloadOptions{
		LoggerPrefix: "qq",
	})
}

func (c *QQChannel) processAudioFiles(audioFiles []AudioFile) (string, []string, []string) {
	if len(audioFiles) == 0 {
		return "", []string{}, []string{}
	}

	content := ""
	mediaPaths := []string{}
	localFiles := []string{}

	for _, audioFile := range audioFiles {
		localPath := c.downloadAttachment(audioFile.URL, audioFile.Filename)
		if localPath != "" {
			localFiles = append(localFiles, localPath)
			mediaPaths = append(mediaPaths, localPath)

			// Check if audio processor is available
			if c.GetAudioProcessor() != nil && c.GetAudioProcessor().IsAvailable() {
				tCtx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
				transcribedText, err := c.GetAudioProcessor().ProcessAudio(tCtx, localPath)
				cancel()

				if err != nil {
					logger.ErrorCF("qq", "Audio transcription failed", map[string]any{
						"error": err.Error(),
						"file":  localPath,
					})
					transcribedText = fmt.Sprintf("[audio: %s (transcription failed)]", audioFile.Filename)
				} else {
					transcribedText = fmt.Sprintf("[voice transcription: %s]", transcribedText)
				}

				if content != "" {
					content += "\n"
				}
				content += transcribedText
			} else {
				// Audio processor not available, just reference the file
				if content != "" {
					content += "\n"
				}
				content += fmt.Sprintf("[audio: %s]", audioFile.Filename)
			}
		}
	}

	return content, mediaPaths, localFiles
}

func (c *QQChannel) cleanupTempFiles(files []string) {
	for _, file := range files {
		if err := os.Remove(file); err != nil {
			logger.DebugCF("qq", "Failed to cleanup temp file", map[string]any{
				"file":  file,
				"error": err.Error(),
			})
		}
	}
}

func (c *QQChannel) extractSenderID(author *dto.User) (string, error) {
	if author != nil && author.ID != "" {
		return author.ID, nil
	}
	return "", fmt.Errorf("no sender ID")
}

func (c *QQChannel) handleC2CMessage() event.C2CMessageEventHandler {
	return func(event *dto.WSPayload, data *dto.WSC2CMessageData) error {
		if c.isDuplicate(data.ID) {
			return nil
		}

		senderID, err := c.extractSenderID(data.Author)
		if err != nil {
			logger.WarnC("qq", err.Error())
			return nil
		}

		content := data.Content
		audioContent, mediaPaths, localFiles := c.processAudioFiles(c.extractAudioFiles(data))
		defer c.cleanupTempFiles(localFiles)

		if content != "" {
			logger.InfoCF("qq", "Received C2C message", map[string]any{
				"sender": senderID,
				"length": len(content),
			})
		}

		if audioContent != "" {
			if content != "" {
				content += "\n"
			}
			content += audioContent
		}

		if content == "" && len(mediaPaths) == 0 {
			logger.DebugC("qq", "Received empty message, ignoring")
			return nil
		}

		logger.InfoCF("qq", "Received C2C message", map[string]any{
			"sender": senderID,
			"length": len(content),
		})

		// 转发到消息总线
		metadata := map[string]string{}

		sender := bus.SenderInfo{
			Platform:    "qq",
			PlatformID:  data.Author.ID,
			CanonicalID: identity.BuildCanonicalID("qq", data.Author.ID),
		}

		if !c.IsAllowedSender(sender) {
			return nil
		}

		c.HandleMessage(c.ctx,
			bus.Peer{Kind: "direct", ID: senderID},
			data.ID,
			senderID,
			senderID,
			content,
			[]string{},
			metadata,
			sender,
		)
		return nil
	}
}

// handleGroupATMessage handles QQ group @ messages
func (c *QQChannel) handleGroupATMessage() event.GroupATMessageEventHandler {
	return func(event *dto.WSPayload, data *dto.WSGroupATMessageData) error {
		if c.isDuplicate(data.ID) {
			return nil
		}

		senderID, err := c.extractSenderID(data.Author)
		if err != nil {
			logger.WarnC("qq", err.Error())
			return nil
		}

		// extract message content (remove @ bot part)
		content := data.Content
		audioContent, mediaPaths, localFiles := c.processAudioFiles(c.extractGroupAudioFiles(data))
		defer c.cleanupTempFiles(localFiles)

		if content != "" {
			logger.InfoCF("qq", "Received group AT message", map[string]any{
				"sender": senderID,
				"group":  data.GroupID,
				"length": len(content),
			})
		}

		if audioContent != "" {
			if content != "" {
				content += "\n"
			}
			content += audioContent
		}

		if content == "" && len(mediaPaths) == 0 {
			logger.DebugC("qq", "Received empty group message, ignoring")
			return nil
		}

		// GroupAT event means bot is always mentioned; apply group trigger filtering
		respond, cleaned := c.ShouldRespondInGroup(true, content)
		if !respond {
			return nil
		}
		content = cleaned

		logger.InfoCF("qq", "Received group AT message", map[string]any{
			"sender": senderID,
			"group":  data.GroupID,
			"length": len(content),
		})

		// 转发到消息总线（使用 GroupID 作为 ChatID）
		metadata := map[string]string{
			"group_id": data.GroupID,
		}

		sender := bus.SenderInfo{
			Platform:    "qq",
			PlatformID:  data.Author.ID,
			CanonicalID: identity.BuildCanonicalID("qq", data.Author.ID),
		}

		if !c.IsAllowedSender(sender) {
			return nil
		}

		c.HandleMessage(c.ctx,
			bus.Peer{Kind: "group", ID: data.GroupID},
			data.ID,
			senderID,
			data.GroupID,
			content,
			[]string{},
			metadata,
			sender,
		)
		return nil
	}
}

// isDuplicate 检查消息是否重复
func (c *QQChannel) isDuplicate(messageID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.processedIDs[messageID] {
		return true
	}

	c.processedIDs[messageID] = true

	// 简单清理：限制 map 大小
	if len(c.processedIDs) > 10000 {
		// 清空一半
		count := 0
		for id := range c.processedIDs {
			if count >= 5000 {
				break
			}
			delete(c.processedIDs, id)
			count++
		}
	}

	return false
}