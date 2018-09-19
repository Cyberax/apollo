package data

import (
	"github.com/sirupsen/logrus"
	"sync"
)

const TaskTable = "task"
const TaskInstanceTable = "task_instance"


type TaskStore struct {
	store KVStore
	mutex sync.RWMutex

	tasksByKey map[string]*StoredTask
	taskInstancesByKey map[string]*TaskInstance
	taskInstancesByParent map[TaskInstanceKey]*TaskInstance
}

// Lock the object for writing
func (ts *TaskStore) WriteLock() {
	ts.mutex.RLock()
}

// Unlock the object for writing
func (ts *TaskStore) WriteUnlock() {
	ts.mutex.RUnlock()
}

func NewTaskStore(store KVStore) *TaskStore {
	return &TaskStore{
		store: store,
		tasksByKey: make(map[string]*StoredTask),
		taskInstancesByKey: make(map[string]*TaskInstance),
		taskInstancesByParent: make(map[TaskInstanceKey]*TaskInstance),
	}
}

func (ts *TaskStore) Hydrate() error {
	var data []*StoredTask
	err := ts.store.LoadTable(TaskTable, &data)
	if err != nil {
		return NewStoreError("failed hydrate the TaskStore", err)
	}

	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	for _, t := range data {
		ts.tasksByKey[t.Key] = t
	}

	return nil
}

func (ts *TaskStore) StoreTask(task *StoredTask) error {
	logrus.Infof("Storing new task: %s", task.String())

	err, _ := ts.store.StoreValues(TaskTable, []StoredTask{*task})
	if err != nil {
		return NewStoreError("failed to store task: " + task.String(), err)
	}

	ts.mutex.Lock()
	defer ts.mutex.Unlock()
	ts.tasksByKey[task.Key] = task
	return nil
}

func (ts *TaskStore) ListTasks(IDs []string, filter func(*StoredTask)(bool)) []*StoredTask {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	if IDs != nil && len(IDs) != 0 {
		var res = make([]*StoredTask, 0, len(IDs))
		for _, k := range IDs {
			task, ok := ts.tasksByKey[k]
			if !ok {
				continue
			}
			if filter == nil || filter(task) {
				res = append(res, task)
			}
		}
		return res
	} else {
		var res = make([]*StoredTask, 0, len(ts.tasksByKey))
		for _, v := range ts.tasksByKey {
			if filter == nil || filter(v) {
				res = append(res, v)
			}
		}
		return res
	}
}
