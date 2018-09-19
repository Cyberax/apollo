package apoclient

import (
	"apollo/proto/gen/restcli"
	"apollo/proto/gen/restcli/queue"
	. "apollo/utils"
	"fmt"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"os"
	"sort"
	"strconv"
	"strings"
)

func MakeQueueListCmd() *cobra.Command {
	var cmdList = &cobra.Command{
		Use:          "list-queues",
		Short:        "List task queues",
		Long:         `list queues`,
		Args:         cobra.MinimumNArgs(0),
		SilenceUsage: true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := ObtainConnection(cmd)
			if err != nil {
				return err
			}

			return DoListQueues(conn, GetFlagS(cmd,"queue"), GetFlagB(cmd,"json"))
		},
	}
	cmdList.Flags().StringP("queue", "q", "", "Queue Name")
	cmdList.Flags().Bool("json", false, "JSON output")
	return cmdList
}

func DoListQueues(cli *restcli.Apollo, queueName string, json bool) error {
	params := queue.NewGetQueueListParams()
	if queueName != "" {
		params.Queue = &queueName
	}

	queues, err := cli.Queue.GetQueueList(params, nil)
	if err != nil {
		return err
	}

	if json {
		for _, t := range queues.Payload {
			bytes, e := t.MarshalBinary()
			if e != nil {
				return e
			}
			fmt.Print(string(bytes)+"\n")
		}
		return nil
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Queue", "Launch Template ID", "Instance Types",
		"Docker Repo", "Docker Login", "Host Count"})
	table.SetRowLine(true)         // Enable row line
	table.SetAutoWrapText(false)

	var data [][]string

	for _, q := range queues.Payload {
		data = append(data, []string{
			q.QueueInfo.Name,
			q.QueueInfo.LaunchTemplateID,
			strings.Join(q.QueueInfo.InstanceTypes, ","),
			q.QueueInfo.DockerRepository,
			q.QueueInfo.DockerLogin,
			strconv.Itoa(int(q.HostCount)),
		})
	}

	sort.Slice(data, func(i, j int) bool {
		return strings.Compare(data[i][0], data[j][0]) < 0 ||
			strings.Compare(data[i][1], data[j][1]) < 0
	})

	table.AppendBulk(data)
	table.Render()

	return nil
}
