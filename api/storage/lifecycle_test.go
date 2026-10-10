package storage

import (
	"context"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"go-ai-study/api/fault"
	"go-ai-study/api/models"
	"testing"
)

func TestInstanceLockPrecedesRecoveryAndRejectsSecondInstance(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).WithArgs(instanceLockID).WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(true))
	first, err := StartInstance(context.Background(), db, func(context.Context) error {
		// The callback observes that the lock statement has already executed.
		return mock.ExpectationsWereMet()
	})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).WithArgs(instanceLockID).WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(false))
	recovered := false
	if _, err = StartInstance(context.Background(), db, func(context.Context) error { recovered = true; return nil }); !errors.Is(err, ErrInstanceActive) || recovered {
		t.Fatalf("second instance recovered live work: recovered=%v err=%v", recovered, err)
	}
	mock.ExpectQuery(`SELECT 1`).WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(1))
	if err = first.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec(`SELECT pg_advisory_unlock\(\$1\)`).WithArgs(instanceLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	first.Close()
	first.Close()
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRecoverOnlyNonterminalJobsWithoutReplay(t *testing.T) {
	store, mock := postgresTestStore(t)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "jobs" SET .* WHERE status IN \(\$8,\$9,\$10\)`).WithArgs("interrupted", "execution interrupted; any index publication must be checked before retry", sqlmock.AnyArg(), "", nil, "failed", sqlmock.AnyArg(), "queued", "running", "canceling").WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec(`UPDATE "analyses" SET .* WHERE status IN \(\$4,\$5,\$6,\$7\)`).WithArgs("execution interrupted", "failed", sqlmock.AnyArg(), "queued", "running", "canceling", "pending").WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()
	if err := store.RecoverInterrupted(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyAnalysisAndJobCreatedAtomically(t *testing.T) {
	store, mock := postgresTestStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "analyses" .* RETURNING "id"`).WithArgs(1, "queued", "", sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
	job := models.Job{ID: "example-job", OwnerID: 7, ProjectID: 1, Kind: models.JobLegacyAnalyze, Target: ".", Status: models.JobQueued, Payload: "package sample"}
	mock.ExpectExec(`INSERT INTO "jobs"`).WithArgs(job.ID, 7, 1, "legacy_analyze", ".", "queued", nil, "", "", sqlmock.AnyArg(), sqlmock.AnyArg(), nil, nil, 42, "package sample").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := store.CreateJob(context.Background(), &job); err != nil || job.LegacyAnalysisID != 42 {
		t.Fatalf("legacy durable admission=%+v %v", job, err)
	}
}
func TestLegacyHistoryWriteFailureRollsBackJobCompletion(t *testing.T) {
	store, mock := postgresTestStore(t)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "jobs" SET .*`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "analyses" SET .*`).WillReturnError(errors.New("private database error"))
	mock.ExpectRollback()
	if err := store.UpdateJob(context.Background(), "example-job", "running", map[string]any{"status": "completed", "result": "{}"}); !errors.Is(err, fault.ErrStorage) {
		t.Fatalf("completion did not rollback: %v", err)
	}
}

func TestReadinessRequiresDatabaseAndInstanceSession(t *testing.T) {
	store, mock := postgresTestStore(t)
	// sqlmock's default connection ping succeeds, without model transports.
	if err := store.Ready(context.Background()); err != nil {
		t.Fatal(err)
	}
	db, _ := store.db.DB()
	mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).WithArgs(instanceLockID).WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(true))
	instance, err := AcquireInstance(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	owned := NewPostgresWithInstance(store.db, instance)
	mock.ExpectQuery(`SELECT 1`).WillReturnError(errors.New("private connection detail"))
	if err := owned.Ready(context.Background()); !errors.Is(err, fault.ErrStorage) {
		t.Fatalf("lost instance session was ready=%v", err)
	}
	mock.ExpectExec(`SELECT pg_advisory_unlock\(\$1\)`).WithArgs(instanceLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	instance.Close()
}
