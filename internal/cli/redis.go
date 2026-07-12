package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var redisCmd = &cobra.Command{
	Use:   "redis [env]",
	Short: "Convenience shortcut to access a Redis resource",
	Long:  `Shortcut for 'ephem exec redis-<env> -- redis-cli'.`,
	Run: func(cmd *cobra.Command, args []string) {
		env := "staging"
		if len(args) > 0 {
			env = args[0]
		}

		resourceName := fmt.Sprintf("redis-%s", env)
		// If they explicitly passed "vds", we can match "vds-redis" or just use the arg directly
		if env == "vds" {
			resourceName = "vds-redis"
		}

		// Find path of current executing binary
		self, err := os.Executable()
		if err != nil {
			self = "ephem" // fallback
		}

		// Execute ephem exec <resourceName> -- redis-cli
		execArgs := []string{"exec", resourceName, "redis-cli"}

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
	RootCmd.AddCommand(redisCmd)
}
