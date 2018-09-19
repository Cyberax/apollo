package data

import (
	"sync"
	"time"
)

const TokenStoreTable = "token_store"
const NeverExpires = -1

type TokenStore struct {
	store KVStore
	mutex sync.RWMutex

	tokensByKey map[string]AuthToken
}

func NewTokenStore(store KVStore) *TokenStore {
	return &TokenStore{
		store: store,
		tokensByKey: make(map[string]AuthToken),
	}
}

func (ts *TokenStore) StoreToken(token AuthToken) error {
	err, _ := ts.store.StoreValues(TokenStoreTable, []AuthToken{token})
	if err != nil {
		return NewStoreError("failed to store token: " + token.String(), err)
	}

	ts.mutex.Lock()
	defer ts.mutex.Unlock()
	ts.tokensByKey[token.Key] = token
	return nil
}

func (ts *TokenStore) GetTokenByKey(key string) (AuthToken, bool) {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()
	token, ok := ts.tokensByKey[key]
	return token, ok
}

func (ts *TokenStore) Hydrate() error {
	var data []AuthToken

	err := ts.store.LoadTable(TokenStoreTable, &data)
	if err != nil {
		return NewStoreError("failed hydrate the TokenStore", err)
	}

	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	for _, t := range data {
		ts.tokensByKey[t.Key] = t
	}

	return nil
}

func (ts *TokenStore) ReapTokens(expireAfter time.Time) error {
	ts.mutex.RLock()
	tokensToKill := make([]string, 0, 100)
	for k, v := range ts.tokensByKey {
		if v.Expires != NeverExpires && v.Expires.ToTime().Before(expireAfter) {
			tokensToKill = append(tokensToKill, k)
		}
	}
	ts.mutex.RUnlock()

	for _, key := range tokensToKill {
		err := ts.store.DeleteValue(TokenStoreTable, key)
		if err != nil {
			return NewStoreError("failed to delete key "+key, err)
		}

		// We only grab the lock briefly to avoid holding it during
		// kvstore operations
		ts.mutex.Lock()
		delete(ts.tokensByKey, key)
		ts.mutex.Unlock()
	}

	return nil
}
