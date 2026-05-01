package model

// VideoMeta contains metadata extracted from a downloaded video.
type VideoMeta struct {
	Title           string  `json:"title"`
	DurationSeconds float64 `json:"duration_seconds"`
	URL             string  `json:"url"`
}

// DownloadResult holds the output paths and metadata from the download step.
type DownloadResult struct {
	VideoPath string    `json:"video_path"`
	AudioPath string    `json:"audio_path"`
	Meta      VideoMeta `json:"meta"`
}
