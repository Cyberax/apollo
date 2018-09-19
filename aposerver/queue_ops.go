package aposerver

import (
	"apollo/data"
	"apollo/proto/gen/models"
	"apollo/proto/gen/restapi/operations/queue"
	"apollo/utils"
	"context"
	"fmt"
	"github.com/go-openapi/runtime/middleware"
	"github.com/sirupsen/logrus"
	"net/http"
	"time"
)

type PutQueueProcessor struct {
	ctx context.Context
	store *data.QueueStore
	principal string
	params queue.PutQueueParams
}

func (l *PutQueueProcessor) respondWithError(err error) middleware.Responder {
	logrus.Warnf("Failed to modify/create a queue: %+v", err.Error())
	return queue.NewPutQueueDefault(http.StatusInternalServerError).
		WithPayload(&models.Error{
			Code: http.StatusInternalServerError, Message: err.Error(),
			RequestID: utils.GetReqIdFromContext(l.ctx)})
}

func (l *PutQueueProcessor) Enact() middleware.Responder {
	logrus.Infof("Creating a queue %s", l.params.Queue.Name)

	st := data.StoredQueue {
		Key: l.params.Queue.Name,
		Queue: *l.params.Queue,
		SubmittedOn: data.FromTime(time.Now()),
		SubmittedBy: l.principal,
	}

	// TODO: moar validation?
	err := l.store.StoreQueue(&st) // Will do locking
	if err != nil {
		return l.respondWithError(err)
	}

	return queue.NewPutQueueOK().WithPayload(&queue.PutQueueOKBody{
		QueueName: l.params.Queue.Name,
	})
}


type ListQueueProcessor struct {
	ctx context.Context
	store *data.QueueStore
	params queue.GetQueueListParams
}

func (l *ListQueueProcessor) respondWithError(err error) middleware.Responder {
	logrus.Warnf("Failed to list queues: %+v", err.Error())
	return queue.NewGetQueueListDefault(http.StatusInternalServerError).
		WithPayload(&models.Error{
			Code: http.StatusInternalServerError, Message: err.Error(),
			RequestID: utils.GetReqIdFromContext(l.ctx)})
}

func (l *ListQueueProcessor) Enact() middleware.Responder {
	var queues []*data.StoredQueue
	if l.params.Queue != nil {
		queues = l.store.ListQueues([]string{*l.params.Queue})
	} else {
		queues = l.store.ListQueues(nil)
	}

	var resArr []*queue.GetQueueListOKBodyItems0
	for _, q := range queues {
		resArr = append(resArr, &queue.GetQueueListOKBodyItems0{
			HostCount: 0,
			QueueInfo: &q.Queue,
		})
	}

	return queue.NewGetQueueListOK().WithPayload(resArr)
}


type DeleteQueueProcessor struct {
	ctx context.Context
	store *data.QueueStore
	taskStore *data.TaskStore
	params queue.DeleteQueueParams
}

func (l *DeleteQueueProcessor) respondWithError(err error) middleware.Responder {
	logrus.Warnf("Failed to delete a queue: %+v", err.Error())
	return queue.NewDeleteQueueDefault(http.StatusInternalServerError).
		WithPayload(&models.Error{
			Code: http.StatusInternalServerError, Message: err.Error(),
			RequestID: utils.GetReqIdFromContext(l.ctx)})
}

func (l *DeleteQueueProcessor) Enact() middleware.Responder {
	logrus.Infof("Deleting a queue %s", l.params.Queue)

	// Lock queue first
	l.store.FullLock()
	defer l.store.FullUnlock()

	// Lock the task store to check that we don't have any tasks with this
	// queue name.
	l.taskStore.WriteLock()
	defer l.taskStore.WriteUnlock()

	tasks := l.taskStore.ListTasks(nil, func(task *data.StoredTask) bool {
		return task.Queue == l.params.Queue
	})
	if len(tasks) != 0 {
		return l.respondWithError(fmt.Errorf("queue %s is still in use", l.params.Queue))
	}

	// No new tasks can be created since we're holding the queue write lock
	err := l.store.DeleteQueueUnlocked(l.params.Queue)
	if err != nil {
		return l.respondWithError(err)
	}

	return queue.NewDeleteQueueOK()
}
