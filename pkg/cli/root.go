package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var Version = "dev"

// rootCmd represents the base command when called without any subcommands
// var rootCmd = &cobra.Command{
// 	Use:   "ssctl",
// 	Short: "SSCTL is a command line tool for interacting with the Sunsynk cloud API.",
// 	Long: `A longer description that spans multiple lines and likely contains
// examples and usage of using your application. For example:

// Cobra is a CLI library for Go that empowers applications.
// This application is a tool to generate the needed files
// to quickly create a Cobra application.`,
// 	// Uncomment the following line if your bare application
// 	// has an action associated with it:
// 	// Run: func(cmd *cobra.Command, args []string) { },
// }

var rootCmd = &cobra.Command{
	Use:   "ssctl",
	Short: "Sunsynk CLI control tool",
	Long: fmt.Sprintf(`
   ███████╗███████╗ ██████╗████████╗██╗     
   ██╔════╝██╔════╝██╔════╝╚══██╔══╝██║     
   ███████╗███████╗██║        ██║   ██║     
   ╚════██║╚════██║██║        ██║   ██║     
   ███████║███████║╚██████╗   ██║   ███████╗
   ╚══════╝╚══════╝ ╚═════╝   ╚═╝   ╚══════╝

   SSCTL — Sunsynk Control CLI
   Version: %s

   A command-line tool for interacting with the Sunsynk API.
   Designed for diagnostics, automation, and power-user workflows.

   Features:
     • Authenticate with Sunsynk cloud
     • Query inverter, battery, and plant data
     • Scriptable and automation-friendly

`, Version),
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.ssctl.yaml)")

	rootCmd.PersistentFlags().Bool("k8s", false, "Use Kubernetes secrets to read and store credentials")
	rootCmd.PersistentFlags().Bool("upload", false, "Upload to influxdb")
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
