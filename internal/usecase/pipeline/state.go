package pipeline

import "github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"

// State carries data between pipeline steps.
type State struct {
	TaskID         string
	VideoURL       string
	VideoMeta      model.VideoMeta
	DownloadResult model.DownloadResult
	Transcript     model.Transcript
	Frames         []model.Frame
	FrameContents  []model.FrameContent
	Chunks         []model.Chunk
	Summaries      []model.ChunkSummary
	Warnings       []model.Warning
	TempDir        string
}
