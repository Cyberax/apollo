# Data store structure

Data store is modeled as an in-memory native Go maps, with native locking. The write operations
are simple write-through writes into the underlying database.

# Error handling

Error handling for the storage is an all-or-nothing strategy. We never read from the database
except during the startup so we rely on writes always going through or failing cleanly.
 
If a write to the database does fail with an unclear error response (timeout or Server Internal) 
we can't assume anything about its state, so we lock the storage entirely and try to read
back the value that was being written. If read succeeds then we proceed to succeed or fail the
write request. However if the store can't confirm the outcome of a write within a reasonable 
amount of time then we _hard-fail_ the server to avoid inconsistent data.
