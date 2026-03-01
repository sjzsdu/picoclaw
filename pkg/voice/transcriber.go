package voice

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/utils"
)

// isSilkFormat checks if the audio file is in SILK format by examining the file header.
// SILK format can have: #!SILK_V3 (standard) or 0x02#!SILK_V3 (QQ style)
func isSilkFormat(filePath string) bool {
	data, err := os.ReadFile(filePath)
	if err != nil || len(data) < 10 {
		return false
	}

	// Check for QQ SILK format: 0x02 prefix followed by #!SILK_V3
	if len(data) >= 11 && data[0] == 0x02 && string(data[1:10]) == "#!SILK_V3" {
		return true
	}

	// Check for standard SILK format: #!SILK_V3
	return string(data[:9]) == "#!SILK_V3"
}

// Transcriber defines the interface for audio transcription services.
type Transcriber interface {
	Transcribe(ctx context.Context, audioPath string) (*TranscriptionResult, error)
	IsAvailable() bool
}

// TranscriptionResult contains the result of a transcription operation.
type TranscriptionResult struct {
	Text     string  `json:"text"`
	Language string  `json:"language,omitempty"`
	Duration float64 `json:"duration,omitempty"`
}

// AudioProcessor handles audio file transcription using a Transcriber.
type AudioProcessor struct {
	transcriber Transcriber
}

// NewAudioProcessor creates a new AudioProcessor with the given transcriber.
func NewAudioProcessor(transcriber Transcriber) *AudioProcessor {
	return &AudioProcessor{
		transcriber: transcriber,
	}
}

// ProcessAudio transcribes an audio file and returns the text content.
// Returns an error if the transcriber is not available or transcription fails.
// If the audio format is not supported by the provider, it will be converted using ffmpeg.
func (p *AudioProcessor) ProcessAudio(ctx context.Context, audioPath string) (string, error) {
	if p.transcriber == nil || !p.transcriber.IsAvailable() {
		return "", errors.New("transcriber not available")
	}

	// Check and convert audio format if needed
	audioPath, cleanup, err := p.processAudioWithConversion(ctx, audioPath)
	if err != nil {
		return "", fmt.Errorf("audio format conversion failed: %w", err)
	}
	defer cleanup()

	result, err := p.transcriber.Transcribe(ctx, audioPath)
	if err != nil {
		return "", err
	}

	return result.Text, nil
}

// IsAvailable returns true if the audio processor has a available transcriber.
func (p *AudioProcessor) IsAvailable() bool {
	return p.transcriber != nil && p.transcriber.IsAvailable()
}

// Supported audio formats for each provider
var supportedFormats = map[string][]string{
	"groq":    {".flac", ".mp3", ".mp4", ".mpeg", ".mpga", ".m4a", ".ogg", ".wav", ".webm"},
	"alibaba": {".flac", ".mp3", ".mp4", ".m4a", ".ogg", ".wav", ".webm"},
	"openai":  {".mp3", ".mp4", ".mpeg", ".mpga", ".m4a", ".ogg", ".wav", ".webm"},
}

// isFormatSupported checks if the audio format is supported by the given provider.
func isFormatSupported(filename, provider string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	formats, ok := supportedFormats[provider]
	if !ok {
		// Unknown provider, assume not supported
		return false
	}
	for _, f := range formats {
		if ext == f {
			return true
		}
	}
	return false
}

// convertAudio converts an audio file to MP3 format using ffmpeg.
// Returns the path to the converted file or an error.
// The caller is responsible for cleaning up the converted file.
// For SILK format files (used by QQ), it first converts to WAV then to MP3.
func convertAudio(ctx context.Context, inputPath string) (string, error) {
	if err := EnsureFFmpeg(); err != nil {
		return "", fmt.Errorf("ffmpeg is required: %w", err)
	}

	// Handle SILK format (used by QQ voice messages)
	// SILK files need to be converted to WAV first, then to MP3
	workingPath := inputPath
	needsCleanup := false

	if isSilkFormat(inputPath) {
		logger.InfoCF("voice", "Detected SILK format, converting to WAV first", map[string]any{
			"input": inputPath,
		})

		wavPath, err := ConvertToWav(inputPath)
		if err != nil {
			logger.ErrorCF("voice", "Failed to convert SILK to WAV", map[string]any{
				"error": err,
				"input": inputPath,
			})
			return "", fmt.Errorf("failed to convert SILK to WAV: %w", err)
		}

		workingPath = wavPath
		needsCleanup = true
		logger.InfoCF("voice", "SILK to WAV conversion successful", map[string]any{
			"wav_path": wavPath,
		})
	}

	// Generate output path
	dir := filepath.Dir(workingPath)
	name := filepath.Base(workingPath)
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	outputPath := filepath.Join(dir, base+"_converted.mp3")

	// If we created a temp WAV file, clean it up after MP3 conversion
	if needsCleanup {
		defer os.Remove(workingPath)
	}

	// Run ffmpeg conversion
	// -i: input file
	// -y: overwrite output file if exists
	// -vn: no video
	// -acodec libmp3lame: use MP3 codec
	// -q:a 2: high quality MP3
	cmd := exec.CommandContext(ctx, "ffmpeg", "-i", workingPath, "-y", "-vn", "-acodec", "libmp3lame", "-q:a", "2", outputPath)

	logger.InfoCF("voice", "Converting audio format", map[string]any{
		"input":  workingPath,
		"output": outputPath,
	})

	if err := cmd.Run(); err != nil {
		logger.ErrorCF("voice", "Failed to convert audio", map[string]any{
			"error": err,
			"input": workingPath,
		})
		return "", fmt.Errorf("failed to convert audio: %w", err)
	}

	logger.InfoCF("voice", "Audio conversion completed", map[string]any{
		"input":  workingPath,
		"output": outputPath,
	})

	return outputPath, nil
}

// processAudioWithConversion checks if the audio format is supported and converts if needed.
// Returns the (possibly converted) audio path and a cleanup function.
func (p *AudioProcessor) processAudioWithConversion(ctx context.Context, audioPath string) (string, func(), error) {
	// Determine the provider
	var provider string
	switch p.transcriber.(type) {
	case *GroqTranscriber:
		provider = "groq"
	case *AlibabaTranscriber:
		provider = "alibaba"
	default:
		provider = "groq" // default
	}

	// Check if conversion is needed
	if isFormatSupported(audioPath, provider) {
		return audioPath, func() {}, nil
	}

	// Convert the audio file
	logger.InfoCF("voice", "Audio format not supported, converting", map[string]any{
		"path":     audioPath,
		"provider": provider,
	})

	convertedPath, err := convertAudio(ctx, audioPath)
	if err != nil {
		return "", nil, err
	}

	// Return cleanup function to remove converted file
	cleanup := func() {
		if convertedPath != audioPath {
			os.Remove(convertedPath)
			logger.DebugCF("voice", "Cleaned up converted audio file", map[string]any{"path": convertedPath})
		}
	}

	return convertedPath, cleanup, nil
}

// GroqTranscriber implements the Transcriber interface using Groq's Whisper API.
type GroqTranscriber struct {
	apiKey     string
	apiBase    string
	httpClient *http.Client
	model      string
}

// TranscriptionResponse is kept for backward compatibility
type TranscriptionResponse struct {
	Text     string  `json:"text"`
	Language string  `json:"language,omitempty"`
	Duration float64 `json:"duration,omitempty"`
}

func NewGroqTranscriber(apiKey string) *GroqTranscriber {
	return NewGroqTranscriberWithOptions(apiKey, "https://api.groq.com/openai/v1", "whisper-large-v3")
}

// NewGroqTranscriberWithOptions creates a GroqTranscriber with custom options.
func NewGroqTranscriberWithOptions(apiKey, apiBase, model string) *GroqTranscriber {
	logger.DebugCF("voice", "Creating Groq transcriber", map[string]any{"has_api_key": apiKey != ""})

	return &GroqTranscriber{
		apiKey:  apiKey,
		apiBase: apiBase,
		model:   model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Transcribe implements the Transcriber interface.
func (t *GroqTranscriber) Transcribe(ctx context.Context, audioFilePath string) (*TranscriptionResult, error) {
	logger.InfoCF("voice", "Starting transcription", map[string]any{"audio_file": audioFilePath})

	audioFile, err := os.Open(audioFilePath)
	if err != nil {
		logger.ErrorCF("voice", "Failed to open audio file", map[string]any{"path": audioFilePath, "error": err})
		return nil, fmt.Errorf("failed to open audio file: %w", err)
	}
	defer audioFile.Close()

	fileInfo, err := audioFile.Stat()
	if err != nil {
		logger.ErrorCF("voice", "Failed to get file info", map[string]any{"path": audioFilePath, "error": err})
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	logger.DebugCF("voice", "Audio file details", map[string]any{
		"size_bytes": fileInfo.Size(),
		"file_name":  filepath.Base(audioFilePath),
	})

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	part, err := writer.CreateFormFile("file", filepath.Base(audioFilePath))
	if err != nil {
		logger.ErrorCF("voice", "Failed to create form file", map[string]any{"error": err})
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	copied, err := io.Copy(part, audioFile)
	if err != nil {
		logger.ErrorCF("voice", "Failed to copy file content", map[string]any{"error": err})
		return nil, fmt.Errorf("failed to copy file content: %w", err)
	}

	logger.DebugCF("voice", "File copied to request", map[string]any{"bytes_copied": copied})

	if err = writer.WriteField("model", t.model); err != nil {
		logger.ErrorCF("voice", "Failed to write model field", map[string]any{"error": err})
		return nil, fmt.Errorf("failed to write model field: %w", err)
	}

	if err = writer.WriteField("response_format", "json"); err != nil {
		logger.ErrorCF("voice", "Failed to write response_format field", map[string]any{"error": err})
		return nil, fmt.Errorf("failed to write response_format field: %w", err)
	}

	if err = writer.Close(); err != nil {
		logger.ErrorCF("voice", "Failed to close multipart writer", map[string]any{"error": err})
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	url := t.apiBase + "/audio/transcriptions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, &requestBody)
	if err != nil {
		logger.ErrorCF("voice", "Failed to create request", map[string]any{"error": err})
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	logger.DebugCF("voice", "Sending transcription request to Groq API", map[string]any{
		"url":                url,
		"request_size_bytes": requestBody.Len(),
		"file_size_bytes":    fileInfo.Size(),
	})

	resp, err := t.httpClient.Do(req)
	if err != nil {
		logger.ErrorCF("voice", "Failed to send request", map[string]any{"error": err})
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.ErrorCF("voice", "Failed to read response", map[string]any{"error": err})
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.ErrorCF("voice", "API error", map[string]any{
			"status_code": resp.StatusCode,
			"response":    string(body),
		})
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	logger.DebugCF("voice", "Received response from Groq API", map[string]any{
		"status_code":         resp.StatusCode,
		"response_size_bytes": len(body),
	})

	var result TranscriptionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		logger.ErrorCF("voice", "Failed to unmarshal response", map[string]any{"error": err})
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	logger.InfoCF("voice", "Transcription completed successfully", map[string]any{
		"text_length":           len(result.Text),
		"language":              result.Language,
		"duration_seconds":      result.Duration,
		"transcription_preview": utils.Truncate(result.Text, 50),
	})

	return &TranscriptionResult{
		Text:     result.Text,
		Language: result.Language,
		Duration: result.Duration,
	}, nil
}

func (t *GroqTranscriber) IsAvailable() bool {
	available := t.apiKey != ""
	logger.DebugCF("voice", "Checking transcriber availability", map[string]any{"available": available})
	return available
}

// AlibabaTranscriber implements the Transcriber interface using DashScope's
// OpenAI-compatible API with the Qwen3-ASR-Flash model.
// It sends base64-encoded audio via the chat/completions endpoint for
// synchronous speech recognition.
type AlibabaTranscriber struct {
	apiKey     string
	apiBase    string
	httpClient *http.Client
	model      string
}

// NewAlibabaTranscriber creates a new Alibaba transcriber with default settings.
func NewAlibabaTranscriber(apiKey string) *AlibabaTranscriber {
	return NewAlibabaTranscriberWithOptions(
		apiKey,
		"https://dashscope.aliyuncs.com/compatible-mode/v1",
		"qwen3-asr-flash",
	)
}

// NewAlibabaTranscriberWithOptions creates an Alibaba transcriber with custom options.
func NewAlibabaTranscriberWithOptions(apiKey, apiBase, model string) *AlibabaTranscriber {
	logger.DebugCF("voice", "Creating Alibaba transcriber", map[string]any{
		"has_api_key": apiKey != "",
		"api_base":    apiBase,
		"model":       model,
	})

	return &AlibabaTranscriber{
		apiKey:  apiKey,
		apiBase: apiBase,
		model:   model,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// audioMIMEType returns the MIME sub-type for a given file extension.
func audioMIMEType(filePath string) string {
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".mp3":
		return "mp3"
	case ".wav":
		return "wav"
	case ".flac":
		return "flac"
	case ".m4a":
		return "m4a"
	case ".ogg":
		return "ogg"
	case ".webm":
		return "webm"
	case ".amr":
		return "amr"
	default:
		return "mp3"
	}
}

// Transcribe implements the Transcriber interface for Alibaba Cloud.
// Uses the OpenAI-compatible chat/completions endpoint with base64-encoded audio.
func (t *AlibabaTranscriber) Transcribe(ctx context.Context, audioFilePath string) (*TranscriptionResult, error) {
	logger.InfoCF("voice", "Starting Alibaba transcription", map[string]any{
		"audio_file": audioFilePath,
		"model":      t.model,
	})

	audioData, err := os.ReadFile(audioFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio file: %w", err)
	}

	logger.DebugCF("voice", "Audio file details", map[string]any{
		"size_bytes": len(audioData),
		"file_name":  filepath.Base(audioFilePath),
	})

	mimeType := audioMIMEType(audioFilePath)
	b64Audio := base64.StdEncoding.EncodeToString(audioData)
	dataURI := fmt.Sprintf("data:audio/%s;base64,%s", mimeType, b64Audio)

	// Build OpenAI-compatible chat completion request with input_audio
	reqPayload := map[string]any{
		"model": t.model,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "input_audio",
						"input_audio": map[string]string{
							"data": dataURI,
						},
					},
				},
			},
		},
	}

	jsonBody, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := strings.TrimRight(t.apiBase, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	logger.DebugCF("voice", "Sending transcription request to DashScope", map[string]any{
		"url":             url,
		"model":           t.model,
		"audio_size":      len(audioData),
		"request_size":    len(jsonBody),
	})

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.ErrorCF("voice", "DashScope API error", map[string]any{
			"status_code": resp.StatusCode,
			"response":    string(body),
		})
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	logger.DebugCF("voice", "Received response from DashScope", map[string]any{
		"status_code":     resp.StatusCode,
		"response_size":   len(body),
	})

	// Parse OpenAI-compatible chat completion response
	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &chatResp); err != nil {
		logger.ErrorCF("voice", "Failed to unmarshal response", map[string]any{
			"error":    err,
			"response": string(body),
		})
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(chatResp.Choices) == 0 || chatResp.Choices[0].Message.Content == "" {
		logger.ErrorCF("voice", "Empty transcription result", map[string]any{"response": string(body)})
		return nil, fmt.Errorf("empty transcription result")
	}

	text := chatResp.Choices[0].Message.Content

	logger.InfoCF("voice", "Alibaba transcription completed successfully", map[string]any{
		"text_length":           len(text),
		"transcription_preview": utils.Truncate(text, 50),
	})

	return &TranscriptionResult{
		Text: text,
	}, nil
}

func (t *AlibabaTranscriber) IsAvailable() bool {
	available := t.apiKey != ""
	logger.DebugCF("voice", "Checking Alibaba transcriber availability", map[string]any{"available": available})
	return available
}
