package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var postgresCmd = &cobra.Command{
	Use:   "postgres [env]",
	Short: "Convenience shortcut to access a PostgreSQL resource",
	Long:  `Shortcut for 'ephem exec postgres-<env> -- psql'.`,
	Run: func(cmd *cobra.Command, args []string) {
		env := "staging"
		if len(args) > 0 {
			env = args[0]
		}

		resourceName := fmt.Sprintf("postgres-%s", env)

		// Find path of current executing binary
		self, err := os.Executable()
		if err != nil {
			self = "ephem" // fallback
		}

		// Execute ephem exec postgres-<env> -- psql
		execArgs := []string{"exec", resourceName, "psql"}

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
	RootCmd.AddCommand(postgresCmd)
}
