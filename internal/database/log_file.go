package ttlcache

import (
	"bufio"
	"fmt"
	"os"
	"sync/atomic"
	"time"
)
type FileTransactionLogger struct {
	events       chan Event
	errors       chan error
	lastSequence uint64
	file         *os.File
	done         chan struct{}
}

func NewFileTransactionLogger(filename string) (TransactionLogger, error) {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0755)
	if err != nil {
		return nil, fmt.Errorf("cannot open transaction log file: %w", err)
	}

	return &FileTransactionLogger{
		events: make(chan Event, 16),
		errors: make(chan error, 1),
		file:   file,
		done:   make(chan struct{}),
	}, nil
}

func (l *FileTransactionLogger) WriteDelete(key string) {
	select {
	case l.events <- Event{EventType: EventDelete, Key: key}:
	case <-l.done:
	}
}

func (l *FileTransactionLogger) WriteAdd(key, value string) {
	select {
	case l.events <- Event{EventType: EventAdd, Key: key, Value: value}:
	case <-l.done:
	}
}

func (l *FileTransactionLogger) Err() <-chan error {
	return l.errors
}

func (l *FileTransactionLogger) ReadEvents() (<-chan Event, <-chan error) {
	scanner := bufio.NewScanner(l.file)
	outEvent := make(chan Event)
	outError := make(chan error, 1)

	go func() {
		var e Event
		defer close(outEvent)
		defer close(outError)

		for scanner.Scan() {
			line := scanner.Text()
			if _, err := fmt.Sscanf(line, "%d\t%d\t%s\t%s",
				&e.Sequence, &e.EventType, &e.Key, &e.Value); err != nil {
				outError <- fmt.Errorf("input parse error: %w", err)
				return
			}

			if l.lastSequence >= e.Sequence {
				outError <- fmt.Errorf("transaction numbers out of sequence")
				return
			}

			l.lastSequence = e.Sequence
			outEvent <- e
		}

		if err := scanner.Err(); err != nil {
			outError <- fmt.Errorf("transaction log read failure: %w", err)
		}
	}()

	return outEvent, outError
}

func (l *FileTransactionLogger) Run() {
	go func() {
		for {
			select {
			case e := <-l.events:
				sequence := atomic.AddUint64(&l.lastSequence, 1)
				_, err := fmt.Fprintf(
					l.file,
					"%d\t%d\t%s\t%s\n",
					sequence,
					e.EventType,
					e.Key,
					e.Value,
				)
				if err != nil {
					select {
					case l.errors <- err:
					default:
					}
				}
			case <-l.done:
				return
			}
		}
	}()
}

func (l *FileTransactionLogger) Close() error {
	close(l.done)
	close(l.events)
	return l.file.Close()
}

func InitFileTransactionLogger(cache *Cache) error {
	logger, err := NewFileTransactionLogger("transaction.log")
	if err != nil {
		return fmt.Errorf("failed to create event logger: %w", err)
	}

	events, errors := logger.ReadEvents()
	for {
		select {
		case err := <-errors:
			if err != nil {
				return fmt.Errorf("failed to read events: %w", err)
			}
		case e, ok := <-events:
			if !ok {
				// Канал закрыт, восстановление завершено
				goto recovered
			}
			switch e.EventType {
			case EventDelete:
				cache.delete(e.Key) // используем внутренний метод без логирования
			case EventAdd:
				cache.add(e.Key, e.Value, time.Minute*2) // используем внутренний метод без логирования
			}
		}
	}

recovered:
	// Подключаем logger к cache
	cache.SetLogger(logger)
	logger.Run()

	// Запускаем обработчик ошибок
	go func() {
		for err := range logger.Err() {
			// Здесь можно логировать ошибки или обрабатывать их
			fmt.Printf("Transaction logger error: %v\n", err)
		}
	}()

	return nil
}
