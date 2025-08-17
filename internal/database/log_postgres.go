package ttlcache

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

const tableName = "transactions"

type PostgresTransactionLogger struct {
	events chan<- Event
	errors <-chan error
	db *sql.DB
	done chan struct{}
}

type PostgresDBParams struct {
	dbName string
	host string
	user string
	password string
}


func NewPostgresTransactionLogger(config PostgresDBParams) (TransactionLogger, error) {
	connStr := fmt.Sprintf("host=%s dbname=%s user=%s password=%s",
		config.host, config.dbName, config.user, config.password)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to open db connection: %w", err)
	}

	logger := &PostgresTransactionLogger{db: db, done: make(chan struct{})}

	exists, err := logger.verifyTableExists()
	if err != nil {
		return nil, fmt.Errorf("failed to verify table: %w", err)
	}

	if !exists {
		if err := logger.createTable(); err != nil {
			return nil, fmt.Errorf("failed to create table: %w", err)
		}
	}

	return logger, nil
}

func (l *PostgresTransactionLogger) WriteDelete(key string) {
	select {
	case l.events <- Event{EventType: EventDelete, Key: key}:
	case <-l.done:
	}
}


func (l *PostgresTransactionLogger) WriteAdd(key string, value string) {
	select {
	case l.events <- Event{EventType: EventAdd, Key: key, Value: value}:
	case <-l.done:
	}
}

func (l *PostgresTransactionLogger) Err() <-chan error {
	return l.errors
}

func (l *PostgresTransactionLogger) Close() error {
	close(l.done)
	close(l.events)
	return l.db.Close()
}

func (l *PostgresTransactionLogger) ReadEvents() (<-chan Event, <-chan error) {
	outEvent := make(chan Event)
	outError :=  make(chan error)

	go func() {
		defer close(outEvent)
		defer close(outError)

		query := `
			SELECT sequence, event_type, key, value FROM transactions
			ORDER BY sequence`

		rows, err := l.db.Query(query)
		if err != nil {
			outError <- fmt.Errorf("sql query error: %w", err)
			return
		}

		defer rows.Close()

		e := Event{}

		for rows.Next() {
			err = rows.Scan(&e.Sequence, &e.EventType, &e.Key, &e.Value)

			if err != nil {
				outError <- fmt.Errorf("error reading now: %w", err)
				return
			}

			outEvent <- e
		}

		err = rows.Err()
		if err != nil {
			outError <- fmt.Errorf("transaction log read failure: %w", err)
		}
	}()

	return outEvent, outError
}

func (l *PostgresTransactionLogger) Run() {
	events := make(chan Event, 16)
	l.events = events

	errors := make(chan error, 1)
	l.errors = errors

	go func() {
		query := `
			INSERT INTO transactions
			(event_type, key, value)
			VALUES ($1, $2, $3)`

		for e := range events {
			_, err := l.db.Exec(query, e.EventType, e.Key, e.Value)
			if err != nil {
				errors <- err
			}
		}
	}()
}

func (l *PostgresTransactionLogger) verifyTableExists() (bool, error) {
	query := `
		SELECT EXISTS (
            SELECT 1 FROM information_schema.tables
            WHERE table_schema = 'public'
            AND table_name = $1
        );`

    var exists bool
    err := l.db.QueryRow(query, tableName).Scan(&exists)
    if err != nil {
    	return false, err
    }

    return exists, err
}

func (l *PostgresTransactionLogger) createTable() error {
	query := `
		CREATE TABLE transactions (
			sequence BIGSERIAL PRIMARY KEY,
			event_type SMALLINT,
			key TEXT,
			value TEXT
		)`

	_, err := l.db.Exec(query)

	return err
}

func InitPostgresTransactionLogger(cache *Cache) error {
	config := PostgresDBParams{
		host: "localhost",
		dbName: "kvs",
		user: "test",
		password: "123456",
	}

	logger, err := NewPostgresTransactionLogger(config)
	if err != nil {
		return fmt.Errorf("failed to create postgres logger: %w", err)
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
				goto recovered
			}
			switch e.EventType {
			case EventDelete:
				cache.delete(e.Key)
			case EventAdd:
				cache.add(e.Key, e.Value, time.Minute*2)
			}
		}
	}

recovered:
	cache.SetLogger(logger)
	logger.Run()

	go func() {
		for err := range logger.Err() {
			fmt.Printf("Postgres transaction logger error: %v\n", err)
		}
	}()

	return nil
}
