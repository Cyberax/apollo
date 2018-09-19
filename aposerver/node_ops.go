package aposerver

import (
	"apollo/data"
	"apollo/proto/gen/models"
	"apollo/proto/gen/restapi/operations/node"
	"apollo/utils"
	"context"
	"fmt"
	"github.com/go-openapi/runtime/middleware"
	"github.com/sirupsen/logrus"
	"net/http"
	"time"
)

type PutUnmanagedNodeProcessor struct {
	ctx context.Context
	store *data.NodeStore
	queueStore *data.QueueStore
	params node.PutUnmanagedNodeParams
	principal data.AuthToken
}

func (l *PutUnmanagedNodeProcessor) respondWithError(err error) middleware.Responder {
	logrus.Warnf("Failed to list Nodes: %+v", err.Error())
	return node.NewPutUnmanagedNodeDefault(http.StatusInternalServerError).
		WithPayload(&models.Error{
			Code: http.StatusInternalServerError, Message: err.Error(),
			RequestID: utils.GetReqIdFromContext(l.ctx)})
}

func (l *PutUnmanagedNodeProcessor) Enact() middleware.Responder {
	l.queueStore.FullLock()
	defer l.queueStore.FullUnlock()
	l.store.FullLock()
	defer l.store.FullUnlock()

	nodes := l.store.ListNodes([]string{l.params.Node.NodeID}, nil)
	if len(nodes) != 0 && nodes[0].Queue != l.params.Node.Queue {
		return l.respondWithError(fmt.Errorf(
			"there's an existing node with conflicting queue"))
	}

	now := data.FromTime(time.Now())
	newNode := data.StoredNode {
		Key:       l.params.Node.NodeID,
		Managed:   false,
		State:     models.NodeStateEnumInitializing,
		CreatedOn: now,
		LastTransitionTime: now,
		Queue: l.params.Node.Queue,
	}

	err := l.store.StoreNode(&newNode)
	if err != nil {
		return l.respondWithError(err)
	}

	//TODO: link into the queue indexes

	return &node.PutUnmanagedNodeOK{}
}


type ListNodesProcessor struct {
	ctx context.Context
	store *data.NodeStore
	params node.GetNodeListParams
}

func (l *ListNodesProcessor) respondWithError(err error) middleware.Responder {
	logrus.Warnf("Failed to list Nodes: %+v", err.Error())
	return node.NewGetNodeListDefault(http.StatusInternalServerError).
		WithPayload(&models.Error{
			Code: http.StatusInternalServerError, Message: err.Error(),
			RequestID: utils.GetReqIdFromContext(l.ctx)})
}

func (l *ListNodesProcessor) Enact() middleware.Responder {
	Nodes := l.store.ListNodes(l.params.NodeID, func(node *data.StoredNode) bool {
		if l.params.QueueName != nil && node.Queue != *l.params.QueueName {
			return false
		}
		return true
	})

	// Format Nodes
	var resArr []*node.GetNodeListOKBodyItems0
	for _, t := range Nodes {
		resArr = append(resArr, &node.GetNodeListOKBodyItems0{
			ManagedNode: t.Managed,
			NodeID:     t.Key,
			NodeState: t.State,
			NodeInfo: t.Info,
		})
	}

	return &node.GetNodeListOK{Payload: resArr}
}


type PostNodeStateProcessor struct {
	ctx context.Context
	store *data.NodeStore
	params node.PostNodeStateParams
}
