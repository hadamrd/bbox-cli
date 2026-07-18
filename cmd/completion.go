package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "print shell completion script",
	Long: `Print a shell completion script.

Install:

  bash:
    source <(bbox completion bash)
    # persistent (Linux):
    bbox completion bash > /etc/bash_completion.d/bbox

  zsh:
    bbox completion zsh > "${fpath[1]}/_bbox"
    # then restart your shell

  fish:
    bbox completion fish | source
    # persistent:
    bbox completion fish > ~/.config/fish/completions/bbox.fish

  powershell:
    bbox completion powershell | Out-String | Invoke-Expression
    # persistent: append the above line to your $PROFILE`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			return cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			return cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
