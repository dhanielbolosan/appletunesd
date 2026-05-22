package main

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "appletunes",
		Short: "CLI interface for the appletunesd",
	}

	var loginCmd = &cobra.Command{
		Use:   "login",
		Short: "Log in with Apple Music account",
		Run: func(cmd *cobra.Command, args []string) {
			resp, err := http.Post("http://localhost:8080/login", "application/json", nil)

			if err != nil {
				fmt.Printf("Error contacting daemon: %v\n", err)
				return
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("%s", body)
		},
	}

	var logoutCmd = &cobra.Command{
		Use:   "logout",
		Short: "Log out of App;e Music account",
		Run: func(cmd *cobra.Command, args []string) {
			resp, err := http.Post("http://localhost:8080/logout", "application/json", nil)

			if err != nil {
				fmt.Printf("Error contacting daemon: %v\n", err)
				return
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("%s", body)
		},
	}

	var accountCmd = &cobra.Command{
		Use:   "account",
		Short: "Check account logged in Apple Music",
		Run: func(cmd *cobra.Command, args []string) {
			resp, err := http.Get("http://localhost:8080/account")

			if err != nil {
				fmt.Printf("Error contacting daemon: %v\n", err)
				return
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("%s\n", body)
		},
	}

	var quitCmd = &cobra.Command{
		Use:   "quit",
		Short: "Kill the appletunesd process",
		Run: func(cmd *cobra.Command, args []string) {
			resp, err := http.Post("http://localhost:8080/quit", "application/json", nil)

			if err != nil {
				fmt.Printf("Error contacting daemon: %v\n", err)
				return
			}

			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("%s", body)
		},
	}

	rootCmd.AddCommand(loginCmd, logoutCmd, accountCmd, quitCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
