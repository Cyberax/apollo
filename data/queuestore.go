package data

import (
	"github.com/sirupsen/logrus"
	"sync"
)

const QueueTable = "queue"

type QueueStore struct {
	store KVStore
	mutex sync.RWMutex

	queuesByName map[string]*StoredQueue
}

// Lock the object for writing
func (ts *QueueStore) WriteLock() {
	ts.mutex.RLock()
}

// Unlock the object for writing
func (ts *QueueStore) WriteUnlock() {
	ts.mutex.RUnlock()
}

func (ts *QueueStore) FullLock() {
	ts.mutex.Lock()
}

func (ts *QueueStore) FullUnlock() {
	ts.mutex.Unlock()
}

func NewQueueStore(store KVStore) *QueueStore {
	return &QueueStore{
		store: store,
		queuesByName: make(map[string]*StoredQueue),
	}
}

func (ts *QueueStore) Hydrate() error {
	var data []*StoredQueue
	err := ts.store.LoadTable(QueueTable, &data)
	if err != nil {
		return NewStoreError("failed hydrate the QueueStore", err)
	}

	ts.FullLock()
	defer ts.FullUnlock()

	for _, t := range data {
		ts.queuesByName[t.Key] = t
	}

	return nil
}

func (ts *QueueStore) StoreQueue(q *StoredQueue) error {
	logrus.Infof("Storing new queue: %s", q.String())

	err, _ := ts.store.StoreValues(QueueTable, []StoredQueue{*q})
	if err != nil {
		return NewStoreError("failed to store queue: " + q.String(), err)
	}

	ts.FullLock()
	defer ts.FullUnlock()

	ts.queuesByName[q.Key] = q
	return nil
}

func (ts *QueueStore) ListQueues(IDs []string) []*StoredQueue {
	ts.WriteLock()
	defer ts.WriteUnlock()

	if IDs != nil && len(IDs) != 0 {
		var res = make([]*StoredQueue, 0, len(IDs))
		for _, k := range IDs {
			queue, ok := ts.queuesByName[k]
			if !ok {
				continue
			}
			res = append(res, queue)
		}
		return res
	} else {
		var res = make([]*StoredQueue, 0, len(ts.queuesByName))
		for _, v := range ts.queuesByName {
			res = append(res, v)
		}
		return res
	}
}

func (ts *QueueStore) DeleteQueueUnlocked(queue string) error {
	err := ts.store.DeleteValue(QueueTable, queue)
	if err != nil {
		return err
	}
	delete(ts.queuesByName, queue)
	return nil
}
