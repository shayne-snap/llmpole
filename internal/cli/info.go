package cli

import (
	"fmt"
	"os"

	"github.com/shayne-snap/llmpole/internal/display"
	"github.com/shayne-snap/llmpole/internal/fetch"
	"github.com/shayne-snap/llmpole/internal/hardware"
	"github.com/shayne-snap/llmpole/internal/models"
	"github.com/shayne-snap/llmpole/internal/pole"

	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info [model]",
	Short: "Show detailed information about a model",
	Args:  cobra.ExactArgs(1),
	RunE:  runInfo,
}

func runInfo(cmd *cobra.Command, args []string) error {
	query := args[0]
	db, err := models.NewDB()
	if err != nil {
		return err
	}
	specs, err := hardware.Detect()
	if err != nil {
		return err
	}
	results := db.FindModel(query)
	if len(results) == 0 && looksLikeRepoID(query) {
		if confirmFetch(query) {
			m, err := fetch.FetchModel(query)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Could not fetch model: %v\n", err)
				return nil
			}
			if err := models.AppendModelToCache(m); err != nil {
				fmt.Fprintf(os.Stderr, "Could not save to cache: %v\n", err)
				return nil
			}
			db, _ = models.NewDB()
			results = db.FindModel(query)
		}
	}
	if len(results) == 0 {
		fmt.Printf("\nNo model found matching '%s'\n", query)
		return nil
	}
	if len(results) > 1 {
		fmt.Println("\nMultiple models found. Please be more specific:")
		for _, m := range results {
			fmt.Printf("  - %s\n", m.Name)
		}
		return nil
	}
	fit := pole.Analyze(results[0], specs)
	display.Info(os.Stdout, specs, fit, globalJSON)
	return nil
}
