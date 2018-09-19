package apoclient

import (
	"apollo/proto/gen/models"
	"apollo/proto/gen/restcli"
	"apollo/proto/gen/restcli/task"
	"apollo/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
)

func MakeSubmitCmd() *cobra.Command {
	taskStruct := models.TaskStruct{
		Tags: map[string]string{},
		TaskEnv: map[string]string{},
	}

	var cmdSubmit = &cobra.Command{
		Use:          "submit [flags] [--] command line",
		Short:        "Submit a task",
		Long:         `submit will put a task (or possibly an array of tasks) into the specified queue`,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			taskStruct.Cmdline = args

			// Parse job
			if cmd.Flag("job-name").Value.String() != "" {
				val, err := cmd.Flags().GetInt64("max-failed-tasks")
				if err != nil {
					return err
				}
				taskStruct.Job = &models.Job {
					JobName: cmd.Flag("job-name").Value.String(),
					MaxFailedCount: val,
				}
			}

			// Do the task env
			inherit, err := cmd.Flags().GetBool("inherit-env")
			if err != nil {
				return nil
			}
			//noinspection GoPreferNilSlice
			kvArr := []string{}
			if inherit {
				kvArr = append(kvArr, os.Environ()...)
			}
			envs, err := cmd.Flags().GetStringArray("env")
			if err != nil {
				return nil
			}
			kvArr = append(kvArr, envs...)
			envMap, err := utils.KvListToMap(kvArr)
			if err != nil {
				return err
			}
			taskStruct.TaskEnv = envMap

			// Do the tags
			tags, err := cmd.Flags().GetStringArray("env")
			if err != nil {
				return nil
			}
			tagMap, err := utils.KvListToMap(tags)
			if err != nil {
				return err
			}
			taskStruct.Tags = tagMap

			validateErr := taskStruct.Validate(nil)
			if validateErr != nil {
				return validateErr
			}

			conn, err := ObtainConnection(cmd)
			if err != nil {
				return err
			}

			return DoSubmit(taskStruct, conn)
		},
	}

	cmdSubmit.Flags().SortFlags = false

	cmdSubmit.Flags().StringVarP(&taskStruct.Queue, "queue",
		"q", "", "The queue to submit the task")
	cmdSubmit.Flags().StringVarP(&taskStruct.Pwd, "pwd",
		"w", "/tmp", "The task's working directory within the image")
	// Cmdline
	cmdSubmit.Flags().Int64VarP(&taskStruct.StartArrayIndex,"start-index", "s",
		0, "Start task array index")
	cmdSubmit.Flags().Int64VarP(&taskStruct.EndArrayIndex,"end-index", "e",
		1, "End task array index")

	// Job
	cmdSubmit.Flags().StringP("job-name", "j", "",
		"The job name associated with this task")
	cmdSubmit.Flags().Int64("max-failed-tasks", -1,
		"How many task instances within a job need to fail before the job is failed, -1 is no limit")

	cmdSubmit.Flags().StringArrayVar(&taskStruct.TaskDependencies, "task-deps",
		[]string{}, "The list of task dependencies of this task")
	cmdSubmit.Flags().StringArrayVar(&taskStruct.SubtaskDependencies,
		"subtask-deps", []string{}, "The list of subtask dependencies of this task")

	cmdSubmit.Flags().Int64VarP(&taskStruct.MaxRAMMb,"max-ram-mb", "m",
		1024, "Maximum amount of RAM for the task")
	cmdSubmit.Flags().Int64VarP(&taskStruct.ExpectedRAMMb,"expected-ram-mb", "x",
		512, "Expected amount of RAM for the task")
	cmdSubmit.Flags().StringVarP(&taskStruct.DockerImageID, "docker-id",
		"d", "", "Docker ID to run this task")
	cmdSubmit.Flags().StringVarP(&taskStruct.Repo, "repo",
		"p", "", "Docker repository to use")

	// Task env
	cmdSubmit.Flags().Bool("inherit-env", false, "Inherit the whole environment")
	cmdSubmit.Flags().StringArray("env", []string{}, "Environment variables to set")

	cmdSubmit.Flags().BoolVarP(&taskStruct.CanUseAllCpus,"can-use-all-cpus", "u",
		true, "Can the task use all available CPUs?")
	cmdSubmit.Flags().Int64VarP(&taskStruct.TimeoutSeconds,"timeout", "o",
		600, "The timeout for the task in seconds")
	cmdSubmit.Flags().Int64VarP(&taskStruct.Retries,"retries", "r",
		3, "The number of retries (within the total timeout) allowed")

	// Tags
	cmdSubmit.Flags().StringArray("tag", []string{}, "Arbitrary tags to associate with the task")

	return cmdSubmit
}

func DoSubmit(taskStruct models.TaskStruct, conn *restcli.Apollo) error {
	bytes, _ := taskStruct.MarshalBinary()
	logrus.Debugf("Submitting task: %s", bytes)

	params := task.NewPutTaskParams().WithTask(&taskStruct)
	res, err := conn.Task.PutTask(params, nil)
	if err != nil {
		return err
	}
	print("TaskID\t", res.Payload.TaskID)
	return nil
}
