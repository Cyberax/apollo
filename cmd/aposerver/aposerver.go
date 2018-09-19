package main

import (
	"apollo/aposerver"
	"apollo/utils"
	"github.com/juju/errors.git"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh/terminal"
	"log"
	"os"
	"runtime/debug"
)


func parseFlags() error {
	var configFile string

	serverCmd := cobra.Command{
		Use:           "aposerver",
		Short:         "The Apollo server application",
		Long:          "Apollo is the scheduler for computational tasks",
		SilenceUsage:  true, // we don't want to print out usage for EVERY error
		SilenceErrors: true, // we do our own error reporting with fatalf
		Run:           func(cmd *cobra.Command, args []string) {
			cmd.Usage()
		},
	}
	serverCmd.SetOutput(os.Stdout)

	serverCmd.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "run the server",
		Long:  "Run the Apollo server",
		RunE: func(cmd *cobra.Command, args []string) error {
			configData, err := prepareServer(cmd, configFile)
			if err != nil {
				return err
			}
			setupLogging(configData)

			serverContext := &aposerver.ServerContext{}
			defer serverContext.Close()

			err = serverContext.InitRegistry(configData)
			if err != nil {
				return err
			}
			return runTheServer(serverContext)
		},
	})

	serverCmd.PersistentFlags().StringVarP(
		&configFile, "config-file", "c", "", "Path to the configuration file to use")
	serverCmd.PersistentFlags().BoolP("verbose", "v", false, "Verbose output")
	serverCmd.PersistentFlags().BoolP("text-output", "t", false, "Use text output instead of JSON")

	serverCmd.PersistentFlags().String("tls-interface", "", "TLS interface")
	serverCmd.PersistentFlags().Int("tls-port", 9443, "TLS listen port")
	serverCmd.PersistentFlags().String("tls-cert", "", "Path to TLS certificate file")
	serverCmd.PersistentFlags().String("tls-key", "", "Path to TLS key file")

	// Run the cmdline parser
	if err := serverCmd.Execute(); err != nil {
		return err
	}

	return nil
}

func setupLogging(v *viper.Viper) {
	if terminal.IsTerminal(int(os.Stdout.Fd())) || v.GetBool("text-output") {
		// When launched with a TTY - use text output
		logrus.SetFormatter(&logrus.TextFormatter{
			DisableTimestamp: true,
		})
	} else {
		// Log as JSON instead of the default ASCII formatter.
		logrus.SetFormatter(&logrus.JSONFormatter{})
	}
	// Add filename to output
	logrus.AddHook(utils.NewFilenameLoggerHook())

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	logrus.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	if v.GetBool("verbose") {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	// Use logrus for standard log output
	// Note that `log` here references stdlib's log
	// Not logrus imported under the name `log`.
	log.SetFlags(log.Lshortfile)
	log.SetOutput(logrus.StandardLogger().Writer())
}

func prepareServer(cmd *cobra.Command, configFile string) (*viper.Viper, error) {
	v := viper.New()
	if configFile != "" {
		v.SetConfigFile(configFile)
	} else {
		v.SetConfigName("apollo-config.yaml")
		v.SetConfigType("yaml")
		v.AddConfigPath("$HOME/.apollo")
		v.AddConfigPath(".")
	}

	v.SetEnvPrefix("APOLLO")
	v.AutomaticEnv()

	// Bind the flag overrides
	v.BindPFlag("listen.tls.interface", cmd.Flags().Lookup("tls-interface"))
	v.BindPFlag("listen.tls.port", cmd.Flags().Lookup("tls-port"))
	v.BindPFlag("listen.tls.certfile", cmd.Flags().Lookup("tls-cert"))
	v.BindPFlag("listen.tls.keyfile", cmd.Flags().Lookup("tls-key"))

	v.BindPFlag("verbose", cmd.Flags().Lookup("verbose"))
	v.BindPFlag("text-output", cmd.Flags().Lookup("text-output"))

	err := v.ReadInConfig()
	if err != nil {
		return nil, err
	}

	return v, nil
}

func runTheServer(serverContext *aposerver.ServerContext) error {
	logrus.Info("Running the server")
	// Run the server in a goroutine to have a nice stack trace
	var done = make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic during server's lifetime")
				log.Printf("%s:\n%s", r, debug.Stack()) // line 20

				err := &aposerver.ServerError{
					Err: errors.NewErr("Panic during server's lifetime"),
				}
				err.SetLocation(1)
				done <- err
			}
		}()
		done <- aposerver.RunServer(serverContext)
	}()
	return <-done
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Failed to start the Apollo server, check configuration")
			log.Printf("%s:\n%s", r, debug.Stack()) // line 20
			os.Exit(1)
		}
	}()
	err := parseFlags()
	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
	os.Exit(0)
}
