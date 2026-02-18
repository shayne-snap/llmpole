package cli

import (
	"os"

	"github.com/shayne-snap/llmpole/internal/display"
	"github.com/shayne-snap/llmpole/internal/hardware"
	"github.com/shayne-snap/llmpole/internal/models"
	"github.com/shayne-snap/llmpole/internal/pole"

	"github.com/spf13/cobra"
)

var recommendCmd = &cobra.Command{
	Use:   "recommend",
	Short: "Recommend top models for your hardware",
	RunE:  runRecommend,
}

func init() {
	recommendCmd.Flags().UintP("limit", "n", 5, "Limit number of recommendations")
	recommendCmd.Flags().String("use-case", "", "Filter by use case: general, coding, reasoning, chat, multimodal, embedding")
	recommendCmd.Flags().Bool("json", true, "Output as JSON")
}

func runRecommend(cmd *cobra.Command, args []string) error {
	specs, err := hardware.Detect()
	if err != nil {
		return err
	}
	db, err := models.NewDB()
	if err != nil {
		return err
	}
	limit, _ := cmd.Flags().GetUint("limit")
	useCase, _ := cmd.Flags().GetString("use-case")
	useJSON, _ := cmd.Flags().GetBool("json")
	fits := pole.AnalyzeAll(db.GetAllModels(), specs)
	if useCase != "" {
		fits = pole.FilterByUseCase(fits, useCase)
	}
	fits = pole.RankModelsByFit(fits)
	if uint(len(fits)) > limit {
		fits = fits[:limit]
	}
	display.Recommend(os.Stdout, specs, fits, useJSON)
	return nil
}
