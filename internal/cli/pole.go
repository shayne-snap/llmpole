package cli

import (
	"os"

	"github.com/shayne-snap/llmpole/internal/display"
	"github.com/shayne-snap/llmpole/internal/hardware"
	"github.com/shayne-snap/llmpole/internal/models"
	"github.com/shayne-snap/llmpole/internal/pole"

	"github.com/spf13/cobra"
)

var poleCmd = &cobra.Command{
	Use:   "pole",
	Short: "Pole/adaptation analysis: list models that fit your system, sorted by score",
	RunE:  runPole,
}

func init() {
	poleCmd.Flags().BoolP("perfect", "p", false, "Show only perfect fit")
	poleCmd.Flags().UintP("limit", "n", 0, "Limit number of results")
}

func runPole(cmd *cobra.Command, args []string) error {
	specs, err := hardware.Detect()
	if err != nil {
		return err
	}
	db, err := models.NewDB()
	if err != nil {
		return err
	}
	perfect := globalPerfect
	if cmd.Flags().Changed("perfect") {
		p, _ := cmd.Flags().GetBool("perfect")
		perfect = p
	}
	limit := globalLimit
	if cmd.Flags().Changed("limit") {
		n, _ := cmd.Flags().GetUint("limit")
		limit = n
	}
	useJSON := globalJSON
	fits := pole.AnalyzeAll(db.GetAllModels(), specs)
	fits = pole.RankModelsByFit(fits)
	if perfect {
		fits = pole.FilterPerfectOnly(fits)
	}
	if limit > 0 && len(fits) > int(limit) {
		fits = fits[:limit]
	}
	display.Pole(os.Stdout, specs, fits, useJSON)
	return nil
}
