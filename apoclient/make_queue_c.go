package apoclient

import (
	"apollo/proto/gen/models"
	"apollo/proto/gen/restcli"
	"apollo/proto/gen/restcli/queue"
	. "apollo/utils"
	"fmt"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"strings"
)

func MakePutQueueCommand() *cobra.Command {
	var cmdPut = &cobra.Command{
		Use:          "put-queue",
		Short:        "Create or modify a queue",
		Long:         `Create or modify a queue`,
		Args:         cobra.MinimumNArgs(0),

		SilenceUsage: true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := ObtainConnection(cmd)
			if err != nil {
				return err
			}

			return DoPutQueue(conn, cmd)
		},
	}
	cmdPut.Flags().SortFlags = false

	cmdPut.Flags().StringP("queue", "q", "", "Queue Name")
	cmdPut.Flags().StringP("launch-template-id", "e", "",
		"Launch Template ID")
	cmdPut.Flags().StringP("instance-types", "i", "",
		"Comma-separated instance types")
	cmdPut.Flags().StringP("docker-repository", "r", "",
		"Docker repository URL")
	cmdPut.Flags().StringP("docker-login", "l", "",
		"Docker repository login")
	cmdPut.Flags().StringP("docker-password", "p", "",
		"Docker repository password, use '-' to read it from stdin")

	cmdPut.MarkFlagRequired("queue")
	cmdPut.MarkFlagRequired("launch-template-id")
	cmdPut.MarkFlagRequired("instance-types")

	cmdPut.MarkFlagRequired("docker-repository")
	cmdPut.MarkFlagRequired("docker-login")
	cmdPut.MarkFlagRequired("docker-password")

	return cmdPut
}

func DoPutQueue(cli *restcli.Apollo, cmd *cobra.Command) error {
	params := queue.NewPutQueueParams()

	pass := GetFlagS(cmd,"docker-password")
	if pass == "-" {
		passBytes, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		pass = strings.TrimSpace(string(passBytes))
	}

	params.WithQueue(&models.Queue{
		Name: GetFlagS(cmd,"queue"),
		LaunchTemplateID: GetFlagS(cmd,"launch-template-id"),
		InstanceTypes: strings.Split(GetFlagS(cmd,"instance-types"), ","),
		DockerRepository: GetFlagS(cmd,"docker-repository"),
		DockerLogin: GetFlagS(cmd,"docker-login"),
		DockerPassword: pass,
	})

	_, err := cli.Queue.PutQueue(params, nil)
	if err != nil {
		return err
	}

	fmt.Print("OK\t"+GetFlagS(cmd,"queue")+"\n")
	return nil
}
