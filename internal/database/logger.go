package ttlcache

type TransactionLogger interface {
	WriteDelete(key string)
	WriteAdd(key, value string)
	Err() <-chan error
	ReadEvents() (<-chan Event, <-chan error)
	Run()
	Close() error
}
