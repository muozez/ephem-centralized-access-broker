package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out of ephem",
	Run: func(cmd *cobra.Command, args []string) {
		token, err := LoadToken()
		if err != nil || token == "" {
			fmt.Println("You are not logged in.")
			return
		}

		err = RemoveToken()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error logging out: %v\n", err)
			return
		}
		fmt.Println("Successfully logged out.")
	},
}

func init() {
	RootCmd.AddCommand(logoutCmd)
}
