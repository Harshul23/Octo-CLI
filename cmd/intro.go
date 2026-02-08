package main

import (
	"github.com/harshul/octo-cli/internal/ui"
	"github.com/spf13/cobra"
)

// introCmd represents the intro command
var introCmd = &cobra.Command{
	Use:   "intro",
	Short: "Display the Octo CLI intro animation",
	Long: `Display the beautiful entry animation for Octo CLI.

This command shows the animated logo using the best available
terminal graphics protocol:
  - Kitty Graphics Protocol (Kitty, Ghostty)
  - iTerm2 Inline Images (iTerm2, WezTerm)
  - Sixel Graphics (xterm, mlterm)
  - Halfblocks (Unicode fallback for all terminals)

Press Enter or Esc to skip the animation.`,
	Run: func(cmd *cobra.Command, args []string) {
		ui.RunIntro()
	},
}

func init() {
	rootCmd.AddCommand(introCmd)
}
