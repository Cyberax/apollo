# Locking

The locking is managed mostly at coarse-grained level of store managers, we just lock them
completely when a change is warranted.

Locks must be acquired in the following order to avoid deadlocks:
1. Queue
2. Node
3. Task store
4. Task
5. Subtask

Locks are not recursive and this order MUST also apply even for read-only locks. Otherwise
deadlocks might occur if the upper lock is waiting on getting a full lock.

# Task statuses

Task states are handled somewhat differently, the locking is more fine-grained here - each
subtask has its own lock to manage its node assignments. For the overall task the status
is updated using atomics to avoid locks.
