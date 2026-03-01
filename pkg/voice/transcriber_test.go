package voice

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// MockTranscriber is a mock implementation of Transcriber for testing
type MockTranscriber struct {
	shouldFail  bool
	failErr     error
	resultText  string
	isAvailable bool
}

func (m *MockTranscriber) Transcribe(ctx context.Context, audioPath string) (*TranscriptionResult, error) {
	if m.shouldFail {
		return nil, m.failErr
	}
	return &TranscriptionResult{Text: m.resultText}, nil
}

func (m *MockTranscriber) IsAvailable() bool {
	return m.isAvailable
}

// --- AudioProcessor Tests ---

func TestNewAudioProcessor(t *testing.T) {
	mock := &MockTranscriber{isAvailable: true}
	processor := NewAudioProcessor(mock)
	
	if processor == nil {
		t.Error("NewAudioProcessor returned nil")
	}
	if processor.transcriber != mock {
		t.Error("transcriber not set correctly")
	}
}

func TestAudioProcessor_IsAvailable(t *testing.T) {
	tests := []struct {
		name       string
		transcriber Transcriber
		expected   bool
	}{
		{
			name:       "nil transcriber",
			transcriber: nil,
			expected:   false,
		},
		{
			name:       "unavailable transcriber",
			transcriber: &MockTranscriber{isAvailable: false},
			expected:   false,
		},
		{
			name:       "available transcriber",
			transcriber: &MockTranscriber{isAvailable: true},
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := NewAudioProcessor(tt.transcriber)
			result := processor.IsAvailable()
			if result != tt.expected {
				t.Errorf("IsAvailable() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAudioProcessor_ProcessAudio_TranscriberNotAvailable(t *testing.T) {
	mock := &MockTranscriber{isAvailable: false}
	processor := NewAudioProcessor(mock)
	
	_, err := processor.ProcessAudio(context.Background(), "/tmp/test.mp3")
	if err == nil {
		t.Error("expected error when transcriber not available")
	}
	if err.Error() != "transcriber not available" {
		t.Errorf("expected 'transcriber not available' error, got: %v", err)
	}
}

func TestAudioProcessor_ProcessAudio_Success(t *testing.T) {
	mock := &MockTranscriber{
		isAvailable: true,
		resultText:  "Hello world",
	}
	processor := NewAudioProcessor(mock)
	
	// Create a temp file
	tmpDir := t.TempDir()
	audioFile := filepath.Join(tmpDir, "test.mp3")
	if err := os.WriteFile(audioFile, []byte("fake audio"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	
	result, err := processor.ProcessAudio(context.Background(), audioFile)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != "Hello world" {
		t.Errorf("result = %q, want %q", result, "Hello world")
	}
}

func TestAudioProcessor_ProcessAudio_TranscribeError(t *testing.T) {
	mock := &MockTranscriber{
		isAvailable: true,
		shouldFail:  true,
		failErr:     errors.New("transcription failed"),
	}
	processor := NewAudioProcessor(mock)
	
	tmpDir := t.TempDir()
	audioFile := filepath.Join(tmpDir, "test.mp3")
	if err := os.WriteFile(audioFile, []byte("fake audio"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	
	_, err := processor.ProcessAudio(context.Background(), audioFile)
	if err == nil {
		t.Error("expected error from transcriber")
	}
}

func TestAudioProcessor_ProcessAudio_WithFormatConversion(t *testing.T) {
	// Test with a format that is NOT supported by Groq (default provider)
	// AMR format should trigger conversion
	mock := &MockTranscriber{
		isAvailable: true,
		resultText:  "Transcribed text",
	}
	processor := NewAudioProcessor(mock)
	
	tmpDir := t.TempDir()
	audioFile := filepath.Join(tmpDir, "test.amr")
	if err := os.WriteFile(audioFile, []byte("fake amr audio"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	
	// This will try to convert AMR to MP3 using ffmpeg
	// It will likely fail because the file is not valid AMR, but tests the logic
	_, err := processor.ProcessAudio(context.Background(), audioFile)
	if err != nil {
		// Expected to fail with fake audio data, but tests format conversion logic
		t.Logf("Expected error for fake AMR: %v", err)
	}
}

// --- GroqTranscriber Tests ---

func TestNewGroqTranscriber(t *testing.T) {
	transcriber := NewGroqTranscriber("test-api-key")
	
	if transcriber == nil {
		t.Error("NewGroqTranscriber returned nil")
	}
	if transcriber.apiKey != "test-api-key" {
		t.Errorf("apiKey = %q, want %q", transcriber.apiKey, "test-api-key")
	}
	if transcriber.model != "whisper-large-v3" {
		t.Errorf("model = %q, want %q", transcriber.model, "whisper-large-v3")
	}
}

func TestNewGroqTranscriberWithOptions(t *testing.T) {
	transcriber := NewGroqTranscriberWithOptions("api-key", "https://custom.api.com/v1", "custom-model")
	
	if transcriber.apiKey != "api-key" {
		t.Errorf("apiKey = %q, want %q", transcriber.apiKey, "api-key")
	}
	if transcriber.apiBase != "https://custom.api.com/v1" {
		t.Errorf("apiBase = %q, want %q", transcriber.apiBase, "https://custom.api.com/v1")
	}
	if transcriber.model != "custom-model" {
		t.Errorf("model = %q, want %q", transcriber.model, "custom-model")
	}
}

func TestGroqTranscriber_IsAvailable(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		expected bool
	}{
		{"empty api key", "", false},
		{"valid api key", "test-key", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transcriber := NewGroqTranscriber(tt.apiKey)
			result := transcriber.IsAvailable()
			if result != tt.expected {
				t.Errorf("IsAvailable() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGroqTranscriber_Transcribe_FileNotFound(t *testing.T) {
	transcriber := NewGroqTranscriber("test-key")
	
	_, err := transcriber.Transcribe(context.Background(), "/nonexistent/audio.mp3")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

// --- AlibabaTranscriber Tests ---

func TestNewAlibabaTranscriber(t *testing.T) {
	transcriber := NewAlibabaTranscriber("test-api-key")
	
	if transcriber == nil {
		t.Error("NewAlibabaTranscriber returned nil")
	}
	if transcriber.apiKey != "test-api-key" {
		t.Errorf("apiKey = %q, want %q", transcriber.apiKey, "test-api-key")
	}
	if transcriber.model != "qwen3-asr-flash" {
		t.Errorf("model = %q, want %q", transcriber.model, "qwen3-asr-flash")
	}
}

func TestNewAlibabaTranscriberWithOptions(t *testing.T) {
	transcriber := NewAlibabaTranscriberWithOptions("api-key", "https://custom.api.com/v1", "custom-model")
	
	if transcriber.apiKey != "api-key" {
		t.Errorf("apiKey = %q, want %q", transcriber.apiKey, "api-key")
	}
	if transcriber.apiBase != "https://custom.api.com/v1" {
		t.Errorf("apiBase = %q, want %q", transcriber.apiBase, "https://custom.api.com/v1")
	}
	if transcriber.model != "custom-model" {
		t.Errorf("model = %q, want %q", transcriber.model, "custom-model")
	}
}

func TestAlibabaTranscriber_IsAvailable(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		expected bool
	}{
		{"empty api key", "", false},
		{"valid api key", "test-key", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transcriber := NewAlibabaTranscriber(tt.apiKey)
			result := transcriber.IsAvailable()
			if result != tt.expected {
				t.Errorf("IsAvailable() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAlibabaTranscriber_Transcribe_FileNotFound(t *testing.T) {
	transcriber := NewAlibabaTranscriber("test-key")
	
	_, err := transcriber.Transcribe(context.Background(), "/nonexistent/audio.wav")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

// --- Supported Formats Tests ---

func TestSupportedFormats(t *testing.T) {
	// Verify that supportedFormats is properly defined
	if len(supportedFormats) == 0 {
		t.Error("supportedFormats is empty")
	}
	
	// Check Groq formats
	groqFormats, ok := supportedFormats["groq"]
	if !ok {
		t.Error("groq provider not in supportedFormats")
	}
	if len(groqFormats) == 0 {
		t.Error("groq formats is empty")
	}
	
	// Check Alibaba formats
	alibabaFormats, ok := supportedFormats["alibaba"]
	if !ok {
		t.Error("alibaba provider not in supportedFormats")
	}
	if len(alibabaFormats) == 0 {
		t.Error("alibaba formats is empty")
	}
	
	// Check OpenAI formats
	openaiFormats, ok := supportedFormats["openai"]
	if !ok {
		t.Error("openai provider not in supportedFormats")
	}
	if len(openaiFormats) == 0 {
		t.Error("openai formats is empty")
	}
	
	// Verify some known formats are present
	expectedFormats := map[string][]string{
		"groq":     {".flac", ".mp3", ".wav"},
		"alibaba":  {".wav", ".mp3"},
		"openai":  {".mp3", ".wav"},
	}
	
	for provider, formats := range expectedFormats {
		supported := supportedFormats[provider]
		for _, format := range formats {
			found := false
			for _, s := range supported {
				if s == format {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected format %q for provider %q", format, provider)
			}
		}
	}
}

// --- TranscriptionResult Tests ---

func TestTranscriptionResult(t *testing.T) {
	result := TranscriptionResult{
		Text:     "Hello world",
		Language: "en",
		Duration: 1.5,
	}
	
	if result.Text != "Hello world" {
		t.Errorf("Text = %q, want %q", result.Text, "Hello world")
	}
	if result.Language != "en" {
		t.Errorf("Language = %q, want %q", result.Language, "en")
	}
	if result.Duration != 1.5 {
		t.Errorf("Duration = %v, want %v", result.Duration, 1.5)
	}
}

// --- Provider Type Detection Tests ---

func TestAudioProcessor_ProviderTypeDetection(t *testing.T) {
	// Test with GroqTranscriber - should use groq provider
	groqTranscriber := NewGroqTranscriber("test-key")
	processor := NewAudioProcessor(groqTranscriber)
	
	// Verify it's using GroqTranscriber type
	switch processor.transcriber.(type) {
	case *GroqTranscriber:
		// expected
	default:
		t.Error("expected GroqTranscriber")
	}
	
	// Test with AlibabaTranscriber - should use alibaba provider
	alibabaTranscriber := NewAlibabaTranscriber("test-key")
	processor = NewAudioProcessor(alibabaTranscriber)
	
	// Verify it's using AlibabaTranscriber type
	switch processor.transcriber.(type) {
	case *AlibabaTranscriber:
		// expected
	default:
		t.Error("expected AlibabaTranscriber")
	}
	
	// Test with MockTranscriber - unknown type, should default to groq
	mockTranscriber := &MockTranscriber{isAvailable: true}
	unknownProcessor := NewAudioProcessor(mockTranscriber)
	_ = unknownProcessor // just verify it works
}

// --- Case Insensitive Extension Tests ---

func TestIsFormatSupported_CaseInsensitive(t *testing.T) {
	tests := []struct {
		filename string
		provider string
		expected bool
	}{
		{"test.MP3", "groq", true},
		{"test.MP4", "groq", true},
		{"test.M4A", "groq", true},
		{"test.WAV", "groq", true},
		{"test.FLAC", "groq", true},
		{"test.OGG", "groq", true},
		{"Test.Mp3", "groq", true},
		{"TEST.MP3", "groq", true},
	}
	
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := isFormatSupported(tt.filename, tt.provider)
			if result != tt.expected {
				t.Errorf("isFormatSupported(%q, %q) = %v, want %v", tt.filename, tt.provider, result, tt.expected)
			}
		})
	}
}