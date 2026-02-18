package cli

import (
	"os"

	"github.com/shayne-snap/llmpole/internal/display"
	"github.com/shayne-snap/llmpole/internal/models"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available LLM models",
	RunE:  runList,
}

func runList(cmd *cobra.Command, args []string) error {
	db, err := models.NewDB()
	if err != nil {
		return err
	}
	display.List(os.Stdout, db.GetAllModels())
	return nil
}
