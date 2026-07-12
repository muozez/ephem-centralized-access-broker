package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var ApiURL string

var RootCmd = &cobra.Command{
	Use:   "ephem",
	Short: "ephem - Centralized Ephemeral Access Broker",
	Long:  `ephem is a centralized access broker that issues short-lived infrastructure access on demand.`,
}

func init() {
	RootCmd.PersistentFlags().StringVarP(&ApiURL, "api-url", "u", "http://localhost:8080", "ephem API server URL")
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
