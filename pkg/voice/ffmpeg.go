package voice

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/youthlin/silk"
)

// EnsureFFmpeg checks if ffmpeg is installed.
// Returns an error if ffmpeg is not found in PATH.
func EnsureFFmpeg() error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}
	return nil
}

// ConvertToWav converts an audio file to WAV format.
// For SILK files (used by QQ), it decodes via the silk library first;
// for other formats it shells out to ffmpeg directly.
func ConvertToWav(inputPath string) (string, error) {
	if err := EnsureFFmpeg(); err != nil {
		return "", fmt.Errorf("ffmpeg is required: %w", err)
	}

	if _, err := os.Stat(inputPath); err != nil {
		return "", fmt.Errorf("file not found: %w", err)
	}

	outputPath := filepath.Base(inputPath)
	outputPath = outputPath[:len(outputPath)-len(filepath.Ext(outputPath))] + ".wav"
	outputPath = filepath.Join(filepath.Dir(inputPath), outputPath)

	data, err := os.ReadFile(inputPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	isSilk := false
	if len(data) >= 10 {
		if bytes.HasPrefix(data, []byte{0x02}) && len(data) >= 11 {
			if string(data[1:10]) == "#!SILK_V3" {
				isSilk = true
			}
		}
		if string(data[:9]) == "#!SILK_V3" {
			isSilk = true
		}
	}

	if isSilk {
		silkData := data
		if len(data) > 0 && data[0] == 0x02 {
			silkData = data[1:]
		}

		pcmData, err := silk.Decode(bytes.NewReader(silkData))
		if err != nil {
			return "", fmt.Errorf("failed to decode SILK: %w", err)
		}

		pcmPath := outputPath + ".pcm"
		if err := os.WriteFile(pcmPath, pcmData, 0644); err != nil {
			return "", fmt.Errorf("failed to write PCM file: %w", err)
		}
		defer os.Remove(pcmPath)

		cmd := exec.Command("ffmpeg", "-y", "-f", "s16le", "-ar", "24000", "-ac", "1", "-i", pcmPath, outputPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("failed to convert PCM to WAV: ffmpeg error: %w, output: %s", err, string(output))
		}

		return outputPath, nil
	}

	cmd := exec.Command("ffmpeg", "-y", "-i", inputPath, "-ar", "16000", "-ac", "1", outputPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg error: %w, output: %s", err, string(output))
	}

	return outputPath, nil
}
