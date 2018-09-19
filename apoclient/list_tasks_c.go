package apoclient

import (
	"apollo/proto/gen/restcli"
	"apollo/proto/gen/restcli/task"
	. "apollo/utils"
	"fmt"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"os"
	"sort"
	"strings"
)

func MakeListCmd() *cobra.Command {
	var cmdList = &cobra.Command{
		Use:          "list",
		Short:        "List tasks with optional filtering",
		Long:         `list tasks, applying optional filters`,
		Args:         cobra.MinimumNArgs(0),
		SilenceUsage: true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := ObtainConnection(cmd)
			if err != nil {
				return err
			}

			return DoListTasks(conn, GetFlagS(cmd,"queue"),
				GetFlagS(cmd,"job"), GetFlagB(cmd,"json"))
		},
	}
	cmdList.Flags().SortFlags = false

	cmdList.Flags().StringP("queue", "q", "", "Queue Name")
	cmdList.Flags().StringP("job", "j", "", "Job Name")
	cmdList.Flags().Bool("json", false, "JSON output")
	return cmdList
}

func DoListTasks(cli *restcli.Apollo, queue string, job string, json bool) error {
	params := task.NewGetTaskListParams()
	if queue != "" {
		params.Queue = &queue
	}
	if job != "" {
		params.Job = &job
	}

	tasks, err := cli.Task.GetTaskList(params, nil)
	if err != nil {
		return err
	}

	if json {
		for _, t := range tasks.Payload {
			bytes, e := t.MarshalBinary()
			if e != nil {
				return e
			}
			fmt.Print(string(bytes)+"\n")
		}
		return nil
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Queue", "ID", "Cmdline", "Job (+)", "Exp RAM", "Max RAM",
		"Scales?", "Progress (*)"})
	table.SetRowLine(true)         // Enable row line
	table.SetAutoWrapText(false)

	var data [][]string

	for _, t := range tasks.Payload {
		progress := fmt.Sprintf("Done: %d/%d\n",
			0, t.TaskStruct.EndArrayIndex - t.TaskStruct.StartArrayIndex)
		progress += "A: 1 D: 2 F: 2"

		var job = ""
		if t.TaskStruct.Job != nil {
			job = t.TaskStruct.Job.JobName
			job += fmt.Sprintf("\nMF: %d, CF: %d", t.TaskStruct.Job.MaxFailedCount, 0)
		}
		data = append(data, []string{
			t.TaskStruct.Queue,
			t.TaskID,
			renderCmdline(t.TaskStruct.Cmdline, 40),
			job,
			renderMb(t.TaskStruct.ExpectedRAMMb),
			renderMb(t.TaskStruct.MaxRAMMb),
			fmt.Sprintf("%v", t.TaskStruct.CanUseAllCpus),
			progress,
		})
	}

	sort.Slice(data, func(i, j int) bool {
		return strings.Compare(data[i][0], data[j][0]) < 0 ||
			strings.Compare(data[i][1], data[j][1]) < 0
	})

	table.AppendBulk(data)
	table.Render()
	fmt.Printf("(+) MF: - maximum failed count, CF: currently failed\n")
	fmt.Printf("(*) A: - Number of active subtasks, D: - done, F: - failed\n")

	return nil
}

func renderMb(mb int64) string {
	if mb < 10000 {
		return fmt.Sprintf("%d MB", mb)
	}
	return fmt.Sprintf("%d GB", mb/1024)
}

func renderCmdline(cmdline []string, maxSz int) string {
	res := ""
	for _, s := range cmdline {
		if res != "" {
			res += " "
		}
		if strings.ContainsAny(s, "' ") {
			res += "\"" + s + "\""
		} else {
			res += s
		}
	}
	if len(res) > maxSz {
		return res[0:maxSz] + "..."
	}
	return res
}
