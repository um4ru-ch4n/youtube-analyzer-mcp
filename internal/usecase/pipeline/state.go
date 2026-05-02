package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
)

// State carries data between pipeline steps.
type State struct {
	TaskID          string                `json:"task_id"`
	VideoURL        string                `json:"video_url"`
	VideoMeta       model.VideoMeta       `json:"video_meta"`
	DownloadResult  model.DownloadResult  `json:"download_result"`
	Transcript      model.Transcript      `json:"transcript"`
	Frames          []model.Frame         `json:"frames"`
	ProcessedFrames []model.ProcessedFrame `json:"processed_frames"`
	Chunks          []model.Chunk         `json:"chunks"`
	Summaries       []model.ChunkSummary  `json:"summaries"`
	Warnings        []model.Warning       `json:"warnings"`
	TempDir         string                `json:"temp_dir"`
	LastStep        string                `json:"last_step"`
}

// Save persists the state checkpoint to disk.
func (s *State) Save() error {
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	path := filepath.Join(s.TempDir, "checkpoint.json")
	tmpPath := path + ".tmp"

	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write checkpoint: %w", err)
	}

	return os.Rename(tmpPath, path)
}

// LoadCheckpoint tries to load a saved state from disk. Returns nil if no checkpoint exists.
func LoadCheckpoint(taskDir string) (*State, error) {
	path := filepath.Join(taskDir, "checkpoint.json")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read checkpoint: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal checkpoint: %w", err)
	}

	return &state, nil
}
