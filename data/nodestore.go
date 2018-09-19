package data

import (
	"github.com/sirupsen/logrus"
	"sync"
)

const NodeTable = "node"

type NodeStore struct {
	store KVStore
	mutex sync.RWMutex

	NodesByName map[string]*StoredNode
}

// Lock the object for writing
func (ts *NodeStore) WriteLock() {
	ts.mutex.RLock()
}

// Unlock the object for writing
func (ts *NodeStore) WriteUnlock() {
	ts.mutex.RUnlock()
}

func (ts *NodeStore) FullLock() {
	ts.mutex.Lock()
}

func (ts *NodeStore) FullUnlock() {
	ts.mutex.Unlock()
}

func NewNodeStore(store KVStore) *NodeStore {
	return &NodeStore{
		store: store,
		NodesByName: make(map[string]*StoredNode),
	}
}

func (ts *NodeStore) Hydrate() error {
	var data []*StoredNode
	err := ts.store.LoadTable(NodeTable, &data)
	if err != nil {
		return NewStoreError("failed hydrate the NodeStore", err)
	}

	ts.FullLock()
	defer ts.FullUnlock()

	for _, t := range data {
		ts.NodesByName[t.Key] = t
	}

	return nil
}

func (ts *NodeStore) StoreNode(q *StoredNode) error {
	logrus.Infof("Storing new node: %s", q.String())

	err, _ := ts.store.StoreValues(NodeTable, []StoredNode{*q})
	if err != nil {
		return NewStoreError("failed to store node: " + q.String(), err)
	}

	ts.FullLock()
	defer ts.FullUnlock()

	ts.NodesByName[q.Key] = q
	return nil
}

func (ts *NodeStore) ListNodes(IDs []string, filter func(node *StoredNode) bool) []*StoredNode {
	ts.WriteLock()
	defer ts.WriteUnlock()

	if IDs != nil && len(IDs) != 0 {
		var res = make([]*StoredNode, 0, len(IDs))
		for _, k := range IDs {
			node, ok := ts.NodesByName[k]
			if !ok {
				continue
			}
			if filter != nil && !filter(node) {
				continue
			}
			res = append(res, node)
		}
		return res
	} else {
		var res = make([]*StoredNode, 0, len(ts.NodesByName))
		for _, v := range ts.NodesByName {
			if filter != nil && !filter(v) {
				continue
			}
			res = append(res, v)
		}
		return res
	}
}

func (ts *NodeStore) DeleteNodeUnlocked(node string) error {
	err := ts.store.DeleteValue(NodeTable, node)
	if err != nil {
		return err
	}
	delete(ts.NodesByName, node)
	return nil
}
