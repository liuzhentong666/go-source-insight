package storage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync"
)

// One fixed lock covers this API database, including recovery and all project
// collections. The dedicated session must never return to a connection pool.
const instanceLockID int64 = 716391402839

var ErrInstanceActive = errors.New("another API instance owns this database")

type Instance struct {
	conn *sql.Conn
	once sync.Once
}

// Recovery is unreachable until this process owns the dedicated session lock.
func StartInstance(ctx context.Context, db *sql.DB, recover func(context.Context) error) (*Instance, error) {
	i, err := AcquireInstance(ctx, db)
	if err != nil {
		return nil, err
	}
	if err = recover(ctx); err != nil {
		i.Close()
		return nil, err
	}
	return i, nil
}

func AcquireInstance(ctx context.Context, db *sql.DB) (*Instance, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, errors.New("instance control unavailable")
	}
	var acquired bool
	if err = conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", instanceLockID).Scan(&acquired); err != nil {
		conn.Close()
		return nil, errors.New("instance control unavailable")
	}
	if !acquired {
		conn.Close()
		return nil, ErrInstanceActive
	}
	return &Instance{conn: conn}, nil
}
func (i *Instance) Check(ctx context.Context) error {
	var value int
	return i.conn.QueryRowContext(ctx, "SELECT 1").Scan(&value)
}
func (i *Instance) Close() {
	i.once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5e9)
		defer cancel()
		_, _ = i.conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", instanceLockID)
		// Close normally returns a healthy session to the pool. Mark it unusable to
		// guarantee a failed unlock can never leave a pooled session holding the lock.
		_ = i.conn.Raw(func(any) error { return driver.ErrBadConn })
		_ = i.conn.Close()
	})
}
