package cli

import (
	"fmt"
	"os"

	"github.com/shayne-snap/llmpole/internal/display"
	"github.com/shayne-snap/llmpole/internal/fetch"
	"github.com/shayne-snap/llmpole/internal/models"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search for models by name, provider, or size",
	Args:  cobra.ExactArgs(1),
	RunE:  runSearch,
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := args[0]
	db, err := models.NewDB()
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
	display.Search(os.Stdout, results, query)
	return nil
}
