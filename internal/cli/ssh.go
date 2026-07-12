package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var sshCmd = &cobra.Command{
	Use:   "ssh [env]",
	Short: "Convenience shortcut to access a server via SSH",
	Long:  `Shortcut for 'ephem exec ssh-<env> -- ssh'.`,
	Run: func(cmd *cobra.Command, args []string) {
		env := "staging"
		if len(args) > 0 {
			env = args[0]
		}

		resourceName := fmt.Sprintf("ssh-%s", env)

		// Find path of current executing binary
		self, err := os.Executable()
		if err != nil {
			self = "ephem" // fallback
		}

		// Execute ephem exec ssh-<env> -- ssh
		execArgs := []string{"exec", resourceName, "ssh"}

		child := exec.Command(self, execArgs...)
		child.Stdin = os.Stdin
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr

		err = child.Run()
		if err != nil {
			if exitCode, ok := err.(*exec.ExitError); ok {
				os.Exit(exitCode.ExitCode())
			}
			os.Exit(1)
		}
	},
}

func init() {
	RootCmd.AddCommand(sshCmd)
}
