package storage

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"go-ai-study/api/fault"
	"go-ai-study/api/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func postgresTestStore(t *testing.T) (*Postgres, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
	})
	orm, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	return NewPostgres(orm), mock
}

func TestPostgresOwnershipQueriesIncludeProjectAndJobOwners(t *testing.T) {
	store, mock := postgresTestStore(t)
	mock.ExpectQuery(`SELECT .* FROM "projects" WHERE id = \$1 AND owner_id = \$2 .*`).WithArgs(1, 7).
		WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id", "name"}).AddRow(1, 7, "example"))
	p, err := store.OwnedProject(context.Background(), 1, 7)
	if err != nil || p.OwnerID != 7 {
		t.Fatalf("project query: %+v %v", p, err)
	}
	const id = "00000000-0000-0000-0000-000000000001"
	mock.ExpectQuery(`SELECT jobs\.\* FROM "jobs" JOIN projects ON projects.id = jobs.project_id WHERE jobs.id = \$1 AND jobs.owner_id = \$2 AND projects.owner_id = \$3 .*`).WithArgs(id, 7, 7).
		WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id", "project_id", "kind", "target", "status", "result", "failure_code", "failure_reason"}).AddRow(id, 7, 1, "analyze", ".", "completed", []byte(`{"files_analyzed":1}`), "", ""))
	j, err := store.OwnedJob(context.Background(), id, 7)
	if err != nil || j.ProjectID != 1 || string(j.Result) != `{"files_analyzed":1}` {
		t.Fatalf("job JSONB query: %+v %v", j, err)
	}
	mock.ExpectQuery(`SELECT .* FROM "projects" WHERE id = \$1 AND owner_id = \$2 .*`).WithArgs(1, 8).WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id"}))
	if _, err := store.OwnedProject(context.Background(), 1, 8); !errors.Is(err, fault.ErrNotFound) {
		t.Fatalf("missing/foreign project=%v", err)
	}
}

func TestPostgresCreatesDurableJobAndUpdatesExpectedStates(t *testing.T) {
	store, mock := postgresTestStore(t)
	const id = "00000000-0000-0000-0000-000000000001"
	j := models.Job{ID: id, OwnerID: 7, ProjectID: 1, Kind: models.JobAnalyze, Target: ".", Status: models.JobQueued, Result: json.RawMessage(`{"files_analyzed":1}`)}
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO "jobs"`).WithArgs(id, 7, 1, "analyze", ".", "queued", `{"files_analyzed":1}`, "", "", sqlmock.AnyArg(), sqlmock.AnyArg(), nil, nil, 0, "").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := store.CreateJob(context.Background(), &j); err != nil {
		t.Fatal(err)
	}
	if j.CreatedAt.IsZero() {
		t.Fatal("durable job has no creation timestamp")
	}
	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "jobs" SET .* WHERE id = \$4 AND status = \$5`).WithArgs(now, "running", sqlmock.AnyArg(), id, "queued").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "analyses" SET .* WHERE id = \(SELECT legacy_analysis_id`).WithArgs("running", sqlmock.AnyArg(), id).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	if err := store.UpdateJob(context.Background(), id, models.JobQueued, map[string]any{"status": models.JobRunning, "started_at": now}); err != nil {
		t.Fatal(err)
	}
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "jobs" SET .* WHERE id = \$6 AND status = \$7`).WithArgs(now, "", `{"files_analyzed":1}`, "completed", sqlmock.AnyArg(), id, "running").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "analyses" SET .* WHERE id = \(SELECT legacy_analysis_id`).WithArgs(`{"files_analyzed":1}`, "completed", sqlmock.AnyArg(), id).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	if err := store.UpdateJob(context.Background(), id, models.JobRunning, map[string]any{"status": models.JobCompleted, "result": `{"files_analyzed":1}`, "finished_at": now}); err != nil {
		t.Fatal(err)
	}
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "jobs" SET .* WHERE id = \$3 AND status = \$4`).WithArgs("running", sqlmock.AnyArg(), id, "queued").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	if err := store.UpdateJob(context.Background(), id, models.JobQueued, map[string]any{"status": models.JobRunning}); !errors.Is(err, fault.ErrConflict) {
		t.Fatalf("duplicate transition=%v", err)
	}
}

func TestPostgresErrorsAreClassifiedForSafeResponses(t *testing.T) {
	store, mock := postgresTestStore(t)
	mock.ExpectQuery(`SELECT .* FROM "projects"`).WillReturnError(errors.New("internal database connection details"))
	_, err := store.OwnedProject(context.Background(), 1, 7)
	code, message := fault.Public(err)
	if !errors.Is(err, fault.ErrStorage) || code != "storage_unavailable" || message != fault.ErrStorage.Error() {
		t.Fatalf("unsafe storage error: %s %s", code, message)
	}
}
