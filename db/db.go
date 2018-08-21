package db

// Database interface of Database.
type Database interface {
	PutDeleter

	// Has checks if a key exists.
	Has(key []byte) bool

	// Get return the value to the key in Database.
	Get(key []byte) ([]byte, error)

	// Close close the connection.
	Close()

	// NewBatch creates a batch for atomic changes.
	NewBatch() Batch
}

type Batch interface {
	PutDeleter

	// Write write and flush pending batch write.
	Write()

	// Reset resets the batch for reuse
	Reset()
}

type PutDeleter interface {
	// Put put the key-value entry to Database.
	Put(key []byte, value []byte) error

	// Del delete the key entry in Database.
	Del(key []byte) error
}
