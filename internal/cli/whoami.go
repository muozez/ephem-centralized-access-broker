package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Display the currently logged in user info",
	Run: func(cmd *cobra.Command, args []string) {
		token, err := LoadToken()
		if err != nil || token == "" {
			fmt.Println("You are not logged in.")
			os.Exit(1)
		}

		parser := jwt.NewParser()
		claims := jwt.MapClaims{}
		_, _, err = parser.ParseUnverified(token, claims)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid token: %v\n", err)
			os.Exit(1)
		}

		email, _ := claims["email"].(string)
		name, _ := claims["name"].(string)
		
		var rolesList []string
		if roles, ok := claims["roles"].([]interface{}); ok {
			for _, r := range roles {
				if s, ok := r.(string); ok {
					rolesList = append(rolesList, s)
				}
			}
		}

		fmt.Printf("Logged in as:\n")
		fmt.Printf("  Name:  %s\n", name)
		fmt.Printf("  Email: %s\n", email)
		if len(rolesList) > 0 {
			fmt.Printf("  Roles: %s\n", strings.Join(rolesList, ", "))
		} else {
			fmt.Printf("  Roles: None\n")
		}
	},
}

func init() {
	RootCmd.AddCommand(whoamiCmd)
}
