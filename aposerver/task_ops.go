package aposerver

import (
	"apollo/data"
	"apollo/proto/gen/models"
	"apollo/proto/gen/restapi/operations/task"
	"apollo/utils"
	"context"
	"github.com/go-openapi/runtime/middleware"
	"github.com/sirupsen/logrus"
	"net/http"
	"strconv"
	"time"
)

type TaskSubmitProcessor struct {
	ctx context.Context
	store *data.TaskStore
	queueStore *data.QueueStore
	kvStore data.KVStore
	principal data.AuthToken
	params task.PutTaskParams
}

func (l *TaskSubmitProcessor) respondWithError(code int64, error string) middleware.Responder {
	logrus.Warnf("Failed task submission: %+v", error)
	return task.NewPutTaskDefault(int(code)).WithPayload(&models.Error{
		Code: code, Message: error, RequestID: utils.GetReqIdFromContext(l.ctx)})
}

func (l *TaskSubmitProcessor) Enact() middleware.Responder {
	val, err := l.kvStore.GetCounter("TaskCounter")
	if err != nil {
		return l.respondWithError(http.StatusInternalServerError, err.Error())
	}

	if l.params.Task.StartArrayIndex >= l.params.Task.EndArrayIndex {
		return l.respondWithError(http.StatusBadRequest,
			"End index is not bigger than the start index")
	}

	if l.params.Task.ExpectedRAMMb > l.params.Task.MaxRAMMb {
		return l.respondWithError(http.StatusBadRequest,
			"Expected RAM is bigger than max RAM")
	}

	// Lock the queue so it won't go away while this method is running
	l.queueStore.WriteLock()
	defer l.queueStore.WriteUnlock()

	st := data.StoredTask {
		TaskStruct: *l.params.Task,

		Key: strconv.FormatInt(val, 10),
		SubmittedOn: data.FromTime(time.Now()),
		SubmittedBy: l.principal.RenderEntity(),
	}

	queues := l.queueStore.ListQueues([]string{l.params.Task.Queue})
	if len(queues) == 0 {
		return l.respondWithError(http.StatusBadRequest,
			"Task queue is not found: " + l.params.Task.Queue)
	}

	err = l.store.StoreTask(&st)
	if err != nil {
		return l.respondWithError(http.StatusInternalServerError, err.Error())
	}

	return task.NewPutTaskOK().WithPayload(&task.PutTaskOKBody{
		TaskID: st.Key,
	})
}


type ListTasksProcessor struct {
	ctx context.Context
	store *data.TaskStore
	params task.GetTaskListParams
}

func (l *ListTasksProcessor) respondWithError(err error) middleware.Responder {
	logrus.Warnf("Failed to list tasks: %+v", err.Error())
	return task.NewGetTaskListDefault(http.StatusInternalServerError).
		WithPayload(&models.Error{
			Code: http.StatusInternalServerError, Message: err.Error(),
			RequestID: utils.GetReqIdFromContext(l.ctx)})
}

func (l *ListTasksProcessor) Enact() middleware.Responder {
	tasks := l.store.ListTasks(l.params.ID, func(task *data.StoredTask) bool {
		if l.params.Job != nil && (task.Job == nil || task.Job.JobName != *l.params.Job) {
			return false
		}
		if l.params.Queue != nil && task.Queue != *l.params.Queue {
			return false
		}
		return true
	})

	// Format tasks
	var resArr []*task.GetTaskListOKBodyItems0
	for _, t := range tasks {
		taskStruct := &(t.TaskStruct)

		if l.params.WithEnv == nil || !*l.params.WithEnv {
			// We need to remove the environment from the task output
			tsCopy := *taskStruct
			tsCopy.TaskEnv = nil
			taskStruct = &tsCopy
		}

		resArr = append(resArr, &task.GetTaskListOKBodyItems0{
			TaskID:     t.Key,
			TaskStruct: taskStruct,
		})
	}

	return &task.GetTaskListOK{Payload: resArr}
}
