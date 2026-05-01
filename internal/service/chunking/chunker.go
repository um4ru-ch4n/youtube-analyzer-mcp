package chunking

import "github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"

// BuildChunks groups transcript segments into time-based chunks of approximately
// chunkDurationSec seconds each, and assigns frame contents to chunks by timestamp.
func BuildChunks(segments []model.TranscriptSegment, frames []model.FrameContent, chunkDurationSec int) []model.Chunk {
	if len(segments) == 0 {
		return nil
	}

	if chunkDurationSec <= 0 {
		chunkDurationSec = 45
	}

	chunks := buildSegmentChunks(segments, chunkDurationSec)
	chunks = assignFramesToChunks(chunks, frames)

	return chunks
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

func assignFramesToChunks(chunks []model.Chunk, frames []model.FrameContent) []model.Chunk {
	if len(frames) == 0 {
		return chunks
	}

	for i := range chunks {
		for _, fc := range frames {
			if fc.TimestampSec < chunks[i].TimeStart {
				continue
			}
			if fc.TimestampSec > chunks[i].TimeEnd {
				continue
			}
			chunks[i].Frames = append(chunks[i].Frames, fc)
		}
	}

	return chunks
}
