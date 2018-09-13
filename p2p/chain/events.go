package chain

// StartEvent is triggered when downloading is started
type StartEvent struct{}

// DoneEvent is triggered when downloading is done
type DoneEvent struct{ Err error}
