package cli

import (
	"os"

	"github.com/shayne-snap/llmpole/internal/display"
	"github.com/shayne-snap/llmpole/internal/hardware"

	"github.com/spf13/cobra"
)

var systemCmd = &cobra.Command{
	Use:   "system",
	Short: "Show system hardware specifications",
	RunE:  runSystem,
}

func runSystem(cmd *cobra.Command, args []string) error {
	specs, err := hardware.Detect()
	if err != nil {
		return err
	}
	display.System(os.Stdout, specs, globalJSON)
	return nil
}
