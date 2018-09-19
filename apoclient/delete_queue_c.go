package apoclient

import (
	"apollo/proto/gen/restcli"
	"apollo/proto/gen/restcli/queue"
	. "apollo/utils"
	"fmt"
	"github.com/spf13/cobra"
)

func MakeDeleteQueueCommand() *cobra.Command {
	var cmdDelete = &cobra.Command{
		Use:          "delete-queue",
		Short:        "Delete a queue",
		Long:         `Delete a queue, will fail if there are live tasks within this queue`,
		Args:         cobra.MinimumNArgs(0),
		SilenceUsage: true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := ObtainConnection(cmd)
			if err != nil {
				return err
			}

			return DoDeleteQueue(conn, GetFlagS(cmd,"queue"))
		},
	}
	cmdDelete.Flags().SortFlags = false

	cmdDelete.Flags().StringP("queue", "q", "", "Queue Name")
	cmdDelete.MarkFlagRequired("queue")

	return cmdDelete
}

func DoDeleteQueue(cli *restcli.Apollo, queueName string) error {
	params := queue.NewDeleteQueueParams()
	params.Queue = queueName

	_, err := cli.Queue.DeleteQueue(params, nil)
	if err != nil {
		return err
	}
	fmt.Print("DELETED\t"+queueName+"\n")

	return nil
}
