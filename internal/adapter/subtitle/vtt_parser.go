package subtitle

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
)

var (
	// Matches VTT timestamp lines like "00:01:23.456 --> 00:01:27.890"
	timestampRe = regexp.MustCompile(`^(\d{2}:\d{2}:\d{2}\.\d{3})\s+-->\s+(\d{2}:\d{2}:\d{2}\.\d{3})`)
	// Matches VTT tags like <c>, </c>, <00:01:23.456>
	tagRe = regexp.MustCompile(`<[^>]+>`)
)

// ParseVTT reads a WebVTT file and returns transcript segments.
// Deduplicates repeated lines (common in auto-generated YouTube subtitles).
func ParseVTT(path string) (model.Transcript, error) {
	f, err := os.Open(path)
	if err != nil {
		return model.Transcript{}, fmt.Errorf("open vtt: %w", err)
	}
	defer f.Close()

	var segments []model.TranscriptSegment
	scanner := bufio.NewScanner(f)

	var startSec, endSec float64
	var hasTimestamp bool
	var textLines []string
	lastText := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip WEBVTT header and empty lines
		if line == "WEBVTT" || line == "" || strings.HasPrefix(line, "Kind:") || strings.HasPrefix(line, "Language:") || strings.HasPrefix(line, "NOTE") {
			// Flush previous segment if we have one
			if hasTimestamp && len(textLines) > 0 {
				text := cleanVTTText(strings.Join(textLines, " "))
				if text != "" && text != lastText {
					segments = append(segments, model.TranscriptSegment{
						StartSec: startSec,
						EndSec:   endSec,
						Text:     text,
					})
					lastText = text
				}
			}
			hasTimestamp = false
			textLines = nil
			continue
		}

		// Check for timestamp line
		matches := timestampRe.FindStringSubmatch(line)
		if len(matches) == 3 {
			// Flush previous segment
			if hasTimestamp && len(textLines) > 0 {
				text := cleanVTTText(strings.Join(textLines, " "))
				if text != "" && text != lastText {
					segments = append(segments, model.TranscriptSegment{
						StartSec: startSec,
						EndSec:   endSec,
						Text:     text,
					})
					lastText = text
				}
			}

			startSec = parseVTTTime(matches[1])
			endSec = parseVTTTime(matches[2])
			hasTimestamp = true
			textLines = nil
			continue
		}

		// Skip cue ID lines (numeric only)
		if isNumeric(line) {
			continue
		}

		// Collect text lines
		if hasTimestamp {
			textLines = append(textLines, line)
		}
	}

	// Flush last segment
	if hasTimestamp && len(textLines) > 0 {
		text := cleanVTTText(strings.Join(textLines, " "))
		if text != "" && text != lastText {
			segments = append(segments, model.TranscriptSegment{
				StartSec: startSec,
				EndSec:   endSec,
				Text:     text,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return model.Transcript{}, fmt.Errorf("scan vtt: %w", err)
	}

	// Detect language from filename (subs.en.vtt → "en")
	lang := detectLanguageFromPath(path)

	return model.Transcript{
		Segments: segments,
		Language: lang,
	}, nil
}

// parseVTTTime converts "HH:MM:SS.mmm" to seconds.
func parseVTTTime(s string) float64 {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0
	}

	hours, _ := strconv.ParseFloat(parts[0], 64)
	minutes, _ := strconv.ParseFloat(parts[1], 64)

	secParts := strings.Split(parts[2], ".")
	secs, _ := strconv.ParseFloat(secParts[0], 64)

	var millis float64
	if len(secParts) > 1 {
		millis, _ = strconv.ParseFloat("0."+secParts[1], 64)
	}

	return hours*3600 + minutes*60 + secs + millis
}

// cleanVTTText removes VTT tags and normalizes whitespace.
func cleanVTTText(s string) string {
	s = tagRe.ReplaceAllString(s, "")
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

func isNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func detectLanguageFromPath(path string) string {
	// Extract from filename like "subs.en.vtt"
	base := strings.TrimSuffix(path, ".vtt")
	parts := strings.Split(base, ".")
	if len(parts) >= 2 {
		return parts[len(parts)-1]
	}
	return "unknown"
}
