package main

import (
	"apollo/apoclient"
	"apollo/proto/gen"
	"apollo/utils"
	"github.com/spf13/cobra"
	"os"
	"path"
)

func makeCompletionCmd() *cobra.Command {
	var completionCmd = &cobra.Command{
		Use:   "completion",
		Short: "Generates bash completion scripts",
		Long: `To load completion run
	. <(completion)
	To configure your bash shell to load completions for each session add to your bashrc
	# ~/.bashrc or ~/.profile
	. <(completion)
	`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Parent().GenBashCompletion(os.Stdout)
		},
	}
	return completionCmd
}

func main() {
	var rootCmd = &cobra.Command{
		Use: "apollo",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")
			utils.SetupClientLogging(verbose)
			return utils.CheckRequiredFlags(cmd.Flags())
		},
	}

	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Verbose mode")
	home := os.Getenv("HOME")
	rootCmd.PersistentFlags().StringP("token-file", "t",
		path.Join(home, ".apollo-token"),
		"The token file containing the connection token")

	rootCmd.AddCommand(makeCompletionCmd())
	// Login
	rootCmd.AddCommand(apoclient.MakeLoginCmd())
	rootCmd.AddCommand(apoclient.MakeGetNodeTokenCmd())
	rootCmd.AddCommand(apoclient.MakePingCmd())
	// Task
	rootCmd.AddCommand(apoclient.MakeSubmitCmd())
	rootCmd.AddCommand(apoclient.MakeListCmd())
	rootCmd.AddCommand(apoclient.MakeDescribeCommand())
	// Queue
	rootCmd.AddCommand(apoclient.MakeQueueListCmd())
	rootCmd.AddCommand(apoclient.MakePutQueueCommand())
	rootCmd.AddCommand(apoclient.MakeDeleteQueueCommand())

	err := rootCmd.Execute()
	if err != nil {
		gen.PrintError(err)
		os.Exit(1)
	}
}
