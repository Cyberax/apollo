package data

import (
	"apollo/proto/gen/models"
	"encoding/json"
	"time"
)

type TokenType string
const UserToken = "UserToken"
const NodeToken = "NodeToken"
const TaskToken = "TaskToken"

type AbsoluteTime int64

func (at AbsoluteTime) ToTime() time.Time {
	return time.Unix(int64(at) / 1000, (int64(at) % 1000) * 1e6)
}

func FromTime(tm time.Time) AbsoluteTime {
	return AbsoluteTime(tm.Unix()*1000 + int64(tm.Nanosecond() / 1e6))
}

// Authentication token: there are several token types,
// each of them linked to a different entity: node, user, or task
type AuthToken struct {
	Key     string
	Expires AbsoluteTime
	Type    TokenType
	// The entity key this token is linked to (or account ID for user tokens)
	EntityKey string
	// The requesting entity
	RequestedBy string
	RequestedOn AbsoluteTime
}

func jsonString(a interface{}) string {
	bytes, e := json.Marshal(a)
	if e != nil {
		panic(e.Error())
	}
	return string(bytes)
}

func (a *AuthToken) String() string {
	return jsonString(a)
}

func (a *AuthToken) RenderEntity() string {
	if a.Type == UserToken {
		return "user/" + a.EntityKey
	}
	if a.Type == NodeToken {
		return "node/" + a.EntityKey
	}
	if a.Type == TaskToken {
		return "task/" + a.EntityKey
	}
	panic("Unknown token type: " + a.Type)
}

// Queue
type StoredQueue struct {
	models.Queue
	Key string

	SubmittedOn AbsoluteTime
	SubmittedBy string
}

func (a *StoredQueue) String() string {
	return jsonString(a)
}

// Job Info
type JobInfo struct {
	JobName string
	MaxFailedPercentage float64
	MaxFailedCount int
}

// Node
type StoredNode struct {
	Key string
	Queue string
	CloudID string

	Managed bool
	State models.NodeStateEnum
	CreatedOn AbsoluteTime
	LastTransitionTime AbsoluteTime

	Info models.NodeInfo
}

func (a *StoredNode) String() string {
	return jsonString(a)
}

// The representation of the task array (multiple tasks that differ
// only by their index within the parent task)
type StoredTask struct {
	models.TaskStruct

	Key string
	SubmittedOn AbsoluteTime
	SubmittedBy string
}

func (a *StoredTask) String() string {
	return jsonString(a)
}

type TaskInstanceKey struct {
	ParentKey string
	Index int
}

type TaskInstance struct {
	Key string
	InstanceKey TaskInstanceKey

	ExitCode *int
	RetryNum int
}

func (a *TaskInstance) String() string {
	return jsonString(a)
}
