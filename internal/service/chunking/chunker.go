package chunking

import "github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"

// BuildChunks groups transcript segments into time-based chunks of approximately
// chunkDurationSec seconds each.
func BuildChunks(segments []model.TranscriptSegment, chunkDurationSec int) []model.Chunk {
	if len(segments) == 0 {
		return nil
	}

	if chunkDurationSec <= 0 {
		chunkDurationSec = 45
	}

	return buildSegmentChunks(segments, chunkDurationSec)
}

func buildSegmentChunks(segments []model.TranscriptSegment, chunkDurationSec int) []model.Chunk {
	targetDuration := float64(chunkDurationSec)

	var chunks []model.Chunk
	chunkIndex := 0
	chunkStart := segments[0].StartSec
	var chunkSegments []model.TranscriptSegment

	for _, seg := range segments {
		chunkSegments = append(chunkSegments, seg)

		chunkEnd := seg.EndSec
		duration := chunkEnd - chunkStart

		if duration < targetDuration {
			continue
		}

		chunks = append(chunks, model.Chunk{
			Index:     chunkIndex,
			TimeStart: chunkStart,
			TimeEnd:   chunkEnd,
			Segments:  chunkSegments,
		})

		chunkIndex++
		chunkSegments = nil

		// Next chunk starts where this one ended
		chunkStart = chunkEnd
	}

	// Remaining segments form the last chunk
	if len(chunkSegments) > 0 {
		lastEnd := chunkSegments[len(chunkSegments)-1].EndSec
		chunks = append(chunks, model.Chunk{
			Index:     chunkIndex,
			TimeStart: chunkStart,
			TimeEnd:   lastEnd,
			Segments:  chunkSegments,
		})
	}

	return chunks
}
