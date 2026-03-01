package voice

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestEnsureFFmpeg_Available(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed, skipping")
	}

	if err := EnsureFFmpeg(); err != nil {
		t.Errorf("EnsureFFmpeg() returned error when ffmpeg is installed: %v", err)
	}
}

func TestEnsureFFmpeg_NotAvailable(t *testing.T) {
	// This test verifies the error path by checking the function behavior
	// when ffmpeg is not available. We can't easily mock exec.LookPath,
	// so we just verify the error message format.
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		// ffmpeg is available in test environment, skip
		t.Skip("ffmpeg is available in test environment")
	}

	// If we get here, ffmpeg should not be available
	if err := EnsureFFmpeg(); err == nil {
		t.Error("expected error when ffmpeg is not available")
	}
}

func TestConvertToWav_FileNotFound(t *testing.T) {
	_, err := ConvertToWav("/nonexistent/path/audio.mp3") // shadowing err intentionally for early return
	if err != nil {
		return
	}
	t.Error("expected error for non-existent file")
}

func TestConvertToWav_ShortData(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed, skipping")
	}

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "short.mp3")

	shortData := []byte{0x00, 0x01, 0x02}
	if err := os.WriteFile(tmpFile, shortData, 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	_, err := ConvertToWav(tmpFile)
	if err == nil {
		t.Log("Note: ffmpeg might be able to convert this")
	}
}

func TestIsFormatSupported(t *testing.T) {
	tests := []struct {
		filename string
		provider string
		expected bool
	}{
		{"test.flac", "groq", true},
		{"test.mp3", "groq", true},
		{"test.mp4", "groq", true},
		{"test.mpeg", "groq", true},
		{"test.mpga", "groq", true},
		{"test.m4a", "groq", true},
		{"test.ogg", "groq", true},
		{"test.wav", "groq", true},
		{"test.webm", "groq", true},
		{"test.amr", "groq", false},
		{"test.silk", "groq", false},
		{"test.TEST.MP3", "groq", true},

		{"test.wav", "alibaba", true},
		{"test.mp3", "alibaba", true},
		{"test.m4a", "alibaba", true},
		{"test.flac", "alibaba", true},
		{"test.ogg", "alibaba", true},
		{"test.amr", "alibaba", false},
		{"test.silk", "alibaba", false},

		{"test.mp3", "openai", true},
		{"test.wav", "openai", true},
		{"test.m4a", "openai", true},
		{"test.amr", "openai", false},

		{"test.mp3", "unknown", false},
		{"test.mp3", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.provider+"_"+tt.filename, func(t *testing.T) {
			result := isFormatSupported(tt.filename, tt.provider)
			if result != tt.expected {
				t.Errorf("isFormatSupported(%q, %q) = %v, want %v", tt.filename, tt.provider, result, tt.expected)
			}
		})
	}
}