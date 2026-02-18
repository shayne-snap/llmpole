package cli

import (
	"testing"
)

func TestRootCmd_HasSubcommands(t *testing.T) {
	want := map[string]bool{
		"pole":       true,
		"recommend":  true,
		"system":     true,
		"list":       true,
		"search":     true,
		"info":       true,
		"update-list": true,
	}
	cmds := rootCmd.Commands()
	if len(cmds) < len(want) {
		t.Errorf("root has %d subcommands, want at least %d", len(cmds), len(want))
	}
	got := make(map[string]bool)
	for _, c := range cmds {
		got[c.Name()] = true
	}
	for name := range want {
		if !got[name] {
			t.Errorf("root missing subcommand %q", name)
		}
	}
}

func TestPoleCmd_Flags(t *testing.T) {
	perfect := poleCmd.Flags().Lookup("perfect")
	if perfect == nil {
		t.Error("pole command missing --perfect flag")
	}
	limit := poleCmd.Flags().Lookup("limit")
	if limit == nil {
		t.Error("pole command missing -n/--limit flag")
	}
}

func TestRecommendCmd_Flags(t *testing.T) {
	limit := recommendCmd.Flags().Lookup("limit")
	if limit == nil {
		t.Error("recommend command missing --limit flag")
	}
	useCase := recommendCmd.Flags().Lookup("use-case")
	if useCase == nil {
		t.Error("recommend command missing --use-case flag")
	}
}
