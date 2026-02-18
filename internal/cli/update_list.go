package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shayne-snap/llmpole/internal/fetch"
	"github.com/shayne-snap/llmpole/internal/models"

	"github.com/spf13/cobra"
)

// DefaultListURL is the URL for update-list (canonical list: data/hf_models.json).
const DefaultListURL = "https://raw.githubusercontent.com/shayne-snap/llmpole/main/data/hf_models.json"

var updateListCmd = &cobra.Command{
	Use:   "update-list",
	Short: "Download the latest model list and save to user cache",
	Long:  "Fetches the curated model list from the project URL and writes it to the user cache. Does not require reinstall.",
	RunE:  runUpdateList,
}

func runUpdateList(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	body, err := fetch.FetchModelList(ctx, DefaultListURL)
	if err != nil {
		return fmt.Errorf("update-list: %w", err)
	}
	var entries []models.LlmModel
	if err := json.Unmarshal(body, &entries); err != nil {
		return fmt.Errorf("could not update list: invalid JSON from server: %w", err)
	}
	if err := models.WriteCacheFile(body); err != nil {
		return fmt.Errorf("could not write cache: %w", err)
	}
	fmt.Printf("Updated model list (%d models) in user cache.\n", len(entries))
	return nil
}
