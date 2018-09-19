package data

import (
	"testing"
	"time"
	"github.com/stretchr/testify/assert"
)

func TestTokenStore(t *testing.T) {
	fakeMemStore := NewFakeMemStore()
	fakeMemStore.InitSchema(map[string]int64{TokenStoreTable: 200})
	store := NewTokenStore(fakeMemStore)

	expire := time.Now()
	at1 := AuthToken{
		Key:       "key1",
		Expires:   FromTime(expire),
		Type:      NodeToken,
		EntityKey: "node-1",
	}
	at2 := AuthToken{
		Key:       "key2",
		Expires:   FromTime(expire.Add(100*time.Second)),
		Type:      NodeToken,
		EntityKey: "node-2",
	}
	assert.NoError(t, store.StoreToken(at1))
	assert.NoError(t, store.StoreToken(at2))

	token1, ok := store.GetTokenByKey("key1")
	assert.True(t, ok)
	assert.Equal(t, at1, token1)

	// Create another store and hydrate it
	store2 := NewTokenStore(fakeMemStore)
	assert.NoError(t, store2.Hydrate())

	token1, ok = store2.GetTokenByKey("key1")
	assert.True(t, ok)
	assert.Equal(t, at1, token1)

	// Reap the first key
	assert.NoError(t, store2.ReapTokens(expire.Add(1*time.Second)))
	token1, ok = store2.GetTokenByKey("key1")
	assert.False(t, ok)

	// The second key is still here
	token2, ok := store2.GetTokenByKey("key2")
	assert.True(t, ok)
	assert.Equal(t, at2, token2)
}
