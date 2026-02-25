package voice

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func ConvertToWav(inputPath string) (string, error) {
	if _, err := os.Stat(inputPath); err != nil {
		return "", fmt.Errorf("file not found: %w", err)
	}

	outputPath := filepath.Base(inputPath)
	outputPath = outputPath[:len(outputPath)-len(filepath.Ext(outputPath))] + ".wav"
	outputPath = filepath.Join(filepath.Dir(inputPath), outputPath)

	// check file header to determine format
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// check if it's SILK format (QQ uses SILK with 0x02 prefix)
	isSilk := false
	if len(data) >= 10 {
		// Standard SILK: #!SILK_V3
		// QQ SILK: 0x02 + #!SILK_V3
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
		// Use ffmpeg to convert SILK to WAV
		// ffmpeg can handle SILK if we specify the input format
		cmd := exec.Command("ffmpeg", "-y", "-f", "silk", "-i", inputPath, "-ar", "16000", "-ac", "1", outputPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			// Try alternative: remove 0x02 prefix and try again
			if len(data) > 0 && data[0] == 0x02 {
				tmpPath := inputPath + ".tmp"
				if err := os.WriteFile(tmpPath, data[1:], 0644); err == nil {
					cmd = exec.Command("ffmpeg", "-y", "-i", tmpPath, "-ar", "16000", "-ac", "1", outputPath)
					if _, err2 := cmd.CombinedOutput(); err2 == nil {
						os.Remove(tmpPath)
						return outputPath, nil
					}
					os.Remove(tmpPath)
					// If still fails, return original error
					return "", fmt.Errorf("ffmpeg error: %w, output: %s", err, string(output))
				}
			}
			return "", fmt.Errorf("ffmpeg error: %w, output: %s", err, string(output))
		}
		return outputPath, nil
	}

	// Try as regular audio (amr, mp3, etc)
	cmd := exec.Command("ffmpeg", "-y", "-i", inputPath, "-ar", "16000", "-ac", "1", outputPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg error: %w, output: %s", err, string(output))
	}

	return outputPath, nil
}
