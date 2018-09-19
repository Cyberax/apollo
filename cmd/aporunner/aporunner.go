package main

import (
	"apollo/apoclient"
	"apollo/aporunner"
	"apollo/proto/gen"
	"apollo/proto/gen/restcli"
	"apollo/proto/gen/restcli/login"
	"apollo/utils"
	"context"
	"errors"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"net"
	"os"
	"time"
)


var SdNotifyNoSocketErr = errors.New("no socket")

func SdNotifyReady() error {
	return SdNotify("READY=1")
}

func SdNotify(state string) error {
	socketAddr := &net.UnixAddr{
		Name: os.Getenv("NOTIFY_SOCKET"),
		Net:  "unixgram",
	}

	if socketAddr.Name == "" {
		return SdNotifyNoSocketErr
	}

	conn, err := net.DialUnix(socketAddr.Net, nil, socketAddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Write([]byte(state))
	return err
}


func connectRunner(cmd *cobra.Command) (*restcli.Apollo, error) {
	host := utils.GetFlagS(cmd, "host")
	var tokenStr string
	if host != "" {
		// Do the SigV4 login flow
		v4Res, err := apoclient.SendSigv4Auth(utils.GetFlagS(cmd, "profile"), host)
		if err != nil {
			return nil, err
		}
		tokenStr = host + "#" + v4Res.AuthToken + "#" + v4Res.ServerCert
	} else {
		// Obtain connection from the environment
		tokenStr = os.Getenv(apoclient.ApolloConnectionKey)
		if tokenStr == "" {
			return nil, fmt.Errorf("there's no APOLLO_CONNECTION environment variable and host is not specified")
		}
	}

	info, err := apoclient.DecodeTokenString(tokenStr)
	if err != nil {
		return nil, err
	}

	apollo, err := apoclient.MakeConnection(info)
	if err != nil {
		return nil, err
	}
	return apollo, nil
}

func main() {
	runnerCmd := cobra.Command{
		Use:           "aporunner [flags]",
		Short:         "The Apollo node runner",
		Long:          "Start the actual runner, listening to ",
		SilenceUsage:  true, // we don't want to print out usage for EVERY error
		SilenceErrors: true, // we do our own error reporting with fatalf
		RunE:           func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")
			utils.SetupClientLogging(verbose)

			logrus.Info("Connecting to Docker")
			dockerCli, err := client.NewEnvClient()
			if err != nil {
				return err
			}

			// Check that the connection actually works
			_, err = dockerCli.Info(context.Background())
			if err != nil {
				return err
			}
			logrus.Info("Docker connection is operable")

			// Connect to Apollo
			logrus.Info("Connecting to Apollo")
			apollo, err := connectRunner(cmd)
			if err != nil {
				return err
			}

			// Check connectivity
			_, err = apollo.Login.GetPing(login.NewGetPingParams(), nil)
			if err != nil {
				return err
			}
			logrus.Info("Apollo connection is operable")

			logrus.Info("Running the server")
			ctx := aporunner.NewRunnerContext(apollo, dockerCli,
				time.Duration(utils.GetFlagI(cmd, "suicide-delay-sec"))*time.Second)

			// All is OK - notify systemd (if it's used)
			if err = SdNotifyReady(); err != SdNotifyNoSocketErr {
				return err
			}

			return ctx.RunUntilDone()
		},
	}
	runnerCmd.SetOutput(os.Stdout)

	runnerCmd.Flags().SortFlags = false
	runnerCmd.PersistentFlags().BoolP("verbose", "v", false, "Verbose output")
	runnerCmd.PersistentFlags().StringP("profile", "p", "default", "AWS profile")
	runnerCmd.PersistentFlags().StringP("host", "s", "", "Server's host and port")
	runnerCmd.PersistentFlags().Int64("suicide-delay-sec", 2000, "The node suicide delay " +
		"if the connection is lost")

	// Run the cmdline parser
	if err := runnerCmd.Execute(); err != nil {
		gen.PrintError(err)
		os.Exit(1)
	}

	//ctx := context.Background()
	//cli, err := client.NewEnvClient()
	//if err != nil {
	//	panic(err)
	//}
	//
	//_, err = cli.ImagePull(ctx, "docker.io/library/alpine", types.ImagePullOptions{})
	//if err != nil {
	//	panic(err)
	//}
	//
	//resp, err := cli.ContainerCreate(ctx, &container.Config{
	//	Image: "alpine",
	//	Cmd:   []string{"echo", "hello world"},
	//}, nil, nil, "")
	//if err != nil {
	//	panic(err)
	//}
	//
	//if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
	//	panic(err)
	//}
	//
	//statusCh, err := cli.ContainerWait(ctx, resp.ID)
	//if err != nil {
	//	panic(err)
	//}
	//if statusCh != 0 {
	//	panic(statusCh)
	//}
	//
	//out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true})
	//if err != nil {
	//	panic(err)
	//}
	//
	//stdcopy.StdCopy(os.Stdout, os.Stderr, out)
}
