package data

import (
	"sync"
	"reflect"
	"encoding/json"
)

type FakeMemStore struct {
	theMutex sync.Mutex
	data map[string]map[string]string
	counters map[string]int64
}

func (fs *FakeMemStore) StoreValues(tableName string, data interface{}) (error, map[string]bool) {
	fs.theMutex.Lock()
	defer fs.theMutex.Unlock()
	val := reflect.ValueOf(data)
	if val.Kind() != reflect.Array && val.Kind() != reflect.Slice {
		panic("A slice or an array is expected")
	}

	var success = make(map[string]bool)
	table := fs.data[tableName]

	for i := 0; i < val.Len(); i++ {
		value := val.Index(i)
		key := value.FieldByName("Key").String()

		bytes, e := json.Marshal(value.Interface())
		if e != nil {
			return e, success
		}
		table[key] = string(bytes)

		success[key] = true
	}
	return nil, success
}

func (fs *FakeMemStore) DeleteValue(table string, key string) error {
	fs.theMutex.Lock()
	defer fs.theMutex.Unlock()
	delete(fs.data[table], key)
	return nil
}

func (fs *FakeMemStore) LoadTable(table string, output interface{}) error {
	fs.theMutex.Lock()
	defer fs.theMutex.Unlock()

	tableData := fs.data[table]

	outputVal := reflect.ValueOf(output)
	if outputVal.Kind() != reflect.Ptr || reflect.Indirect(outputVal).Kind() != reflect.Slice {
		panic("Was expecting a pointer to a slice")
	}

	sliceType := reflect.Indirect(outputVal).Type()
	result := reflect.MakeSlice(sliceType, 0, len(tableData)+1)

	for _, v := range tableData {
		var res = reflect.New(sliceType.Elem())
		err := json.Unmarshal([]byte(v), res.Interface())
		if err != nil {
			return err
		}
		result = reflect.Append(result, reflect.Indirect(res))
	}

	outputVal.Elem().Set(result)
	return nil
}

func (fs *FakeMemStore) GetCounter(counterName string) (int64, error) {
	fs.theMutex.Lock()
	defer fs.theMutex.Unlock()
	val, ok := fs.counters[counterName]
	if !ok {
		fs.counters[counterName] = 2
		return 1, nil
	}
	fs.counters[counterName] = val + 1
	return val, nil
}

func (fs *FakeMemStore) InitSchema(tables map[string]int64) error {
	fs.theMutex.Lock()
	defer fs.theMutex.Unlock()
	for k := range tables {
		fs.data[k] = map[string]string{}
	}
	return nil
}

func NewFakeMemStore() *FakeMemStore {
	return &FakeMemStore{
		data: map[string]map[string]string{},
		counters: map[string]int64{},
	}
}

