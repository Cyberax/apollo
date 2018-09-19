package apoclient

import (
	"apollo/proto/gen/restcli"
	"apollo/proto/gen/restcli/task"
	"fmt"
	"github.com/spf13/cobra"
)

func MakeDescribeCommand() *cobra.Command {
	var cmdList = &cobra.Command{
		DisableFlagsInUseLine: true,
		Use:          "describe-task [flags] <task-id> [<task-id>, ...]",
		Short:        "Describe a task",
		Long:         `inspect the task details, including its environment`,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := ObtainConnection(cmd)
			if err != nil {
				return err
			}

			return DoDescribeTasks(conn, args)
		},
	}
	return cmdList
}

func DoDescribeTasks(cli *restcli.Apollo, ids []string) error {
	params := task.NewGetTaskListParams()
	params.ID = ids
	var t = true
	params.WithEnv = &t

	tasks, err := cli.Task.GetTaskList(params, nil)
	if err != nil {
		return err
	}

	for _, t := range tasks.Payload {
		bytes, e := t.MarshalBinary()
		if e != nil {
			return e
		}
		fmt.Print(string(bytes)+"\n")
	}

	return nil
}
