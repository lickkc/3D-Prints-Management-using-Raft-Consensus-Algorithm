package raft

// Store defines the interface for persisting Raft state and logs.
type Store interface {
    // SaveState saves the current state of the Raft node.
    SaveState(state []byte) error

    // LoadState loads the saved state of the Raft node.
    LoadState() ([]byte, error)

    // AppendLog appends a new log entry to the storage.
    AppendLog(entry []byte) error

    // GetLogs retrieves all log entries from the storage.
    GetLogs() ([][]byte, error)

    // Close closes the storage connection.
    Close() error
}