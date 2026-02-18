package cli

import (
	"fmt"
	"os"

	"github.com/shayne-snap/llmpole/internal/display"
	"github.com/shayne-snap/llmpole/internal/hardware"
	"github.com/shayne-snap/llmpole/internal/models"
	"github.com/shayne-snap/llmpole/internal/pole"
	"github.com/shayne-snap/llmpole/internal/tui"

	"github.com/spf13/cobra"
)

// Version is set by main from ldflags or "dev". Used for --version / -v.
var Version string

var (
	globalPerfect bool
	globalLimit   uint
	globalJSON    bool
	globalCLI     bool
	showVersion   bool
)

var rootCmd = &cobra.Command{
	Use:   "llmpole",
	Short: "Right-size LLM models to your system's hardware",
	Long:  "LLM pole — find your pole-position models. Right-sizes LLM models to your hardware: detects RAM/CPU/GPU, scores models (quality, speed, fit, context), and shows which will run well. TUI by default; use --cli for table output. Supports multi-GPU, MoE, and quantization.",
	RunE:  runDefault,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if showVersion {
			if Version == "" {
				Version = "dev"
			}
			fmt.Println(Version)
			os.Exit(0)
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&globalPerfect, "perfect", false, "Show only models that perfectly match recommended specs")
	rootCmd.PersistentFlags().UintVarP(&globalLimit, "limit", "n", 0, "Limit number of results (0 = no limit)")
	rootCmd.PersistentFlags().BoolVar(&globalJSON, "json", false, "Output results as JSON")
	rootCmd.PersistentFlags().BoolVar(&globalCLI, "cli", false, "Use classic CLI table output instead of TUI (when no subcommand)")
	rootCmd.PersistentFlags().BoolVarP(&showVersion, "version", "v", false, "Print version and exit")

	rootCmd.AddCommand(systemCmd, listCmd, poleCmd, searchCmd, infoCmd, recommendCmd, updateListCmd)
}

// Execute runs the root command. Returns error for exit code handling.
func Execute() error {
	return rootCmd.Execute()
}

func runDefault(cmd *cobra.Command, args []string) error {
	specs, err := hardware.Detect()
	if err != nil {
		return err
	}
	db, err := models.NewDB()
	if err != nil {
		return err
	}
	fits := pole.AnalyzeAll(db.GetAllModels(), specs)
	fits = pole.RankModelsByFit(fits)

	if globalCLI {
		perfect := globalPerfect
		limit := globalLimit
		useJSON := globalJSON
		if perfect {
			fits = pole.FilterPerfectOnly(fits)
		}
		if limit > 0 && len(fits) > int(limit) {
			fits = fits[:limit]
		}
		display.Pole(os.Stdout, specs, fits, useJSON)
		return nil
	}
	return tui.Run(specs, fits)
}
