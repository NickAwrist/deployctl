package store

import (
	"context"
	"database/sql"
	"time"
)

const (
	JobStatusQueued    = "queued"
	JobStatusRunning   = "running"
	JobStatusSucceeded = "succeeded"
	JobStatusFailed    = "failed"
	JobStatusCancelled = "cancelled"
)

type Job struct {
	ID             string
	Type           string
	DeploymentName string
	Status         string
	Error          string
	CreatedAt      time.Time
	StartedAt      time.Time
	FinishedAt     time.Time
}

type JobLog struct {
	JobID     string
	Sequence  int64
	Message   string
	CreatedAt time.Time
}

type JobStore struct {
	storage storage
}

func NewJobStore() *JobStore {
	return &JobStore{storage: newStorage()}
}

func (s *JobStore) openDatabase() (*sql.DB, error) {
	db, err := s.storage.open()
	if err != nil {
		return nil, err
	}
	if err := migrateJobs(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func (s *JobStore) Insert(ctx context.Context, job Job) error {
	db, err := s.openDatabase()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.ExecContext(ctx, `
		INSERT INTO jobs (id, type, deployment_name, status, error, created_at_unix, started_at_unix, finished_at_unix)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, job.ID, job.Type, job.DeploymentName, job.Status, job.Error, unix(job.CreatedAt), unix(job.StartedAt), unix(job.FinishedAt))
	return err
}

func (s *JobStore) Update(ctx context.Context, job Job) error {
	db, err := s.openDatabase()
	if err != nil {
		return err
	}
	defer db.Close()

	result, err := db.ExecContext(ctx, `
		UPDATE jobs
		SET status = ?, error = ?, started_at_unix = ?, finished_at_unix = ?
		WHERE id = ?
	`, job.Status, job.Error, unix(job.StartedAt), unix(job.FinishedAt), job.ID)
	if err != nil {
		return err
	}
	return requireRowsAffected(result)
}

func (s *JobStore) Get(ctx context.Context, id string) (Job, error) {
	db, err := s.openDatabase()
	if err != nil {
		return Job{}, err
	}
	defer db.Close()

	var job Job
	var createdAt, startedAt, finishedAt int64
	err = db.QueryRowContext(ctx, `
		SELECT id, type, deployment_name, status, error, created_at_unix, started_at_unix, finished_at_unix
		FROM jobs
		WHERE id = ?
	`, id).Scan(&job.ID, &job.Type, &job.DeploymentName, &job.Status, &job.Error, &createdAt, &startedAt, &finishedAt)
	if err != nil {
		return Job{}, err
	}
	job.CreatedAt = fromUnix(createdAt)
	job.StartedAt = fromUnix(startedAt)
	job.FinishedAt = fromUnix(finishedAt)
	return job, nil
}

func (s *JobStore) List(ctx context.Context, deploymentName string) ([]Job, error) {
	db, err := s.openDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := `
		SELECT id, type, deployment_name, status, error, created_at_unix, started_at_unix, finished_at_unix
		FROM jobs
	`
	args := []any{}
	if deploymentName != "" {
		query += " WHERE deployment_name = ?"
		args = append(args, deploymentName)
	}
	query += " ORDER BY created_at_unix DESC, id DESC"

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var job Job
		var createdAt, startedAt, finishedAt int64
		if err := rows.Scan(&job.ID, &job.Type, &job.DeploymentName, &job.Status, &job.Error, &createdAt, &startedAt, &finishedAt); err != nil {
			return nil, err
		}
		job.CreatedAt = fromUnix(createdAt)
		job.StartedAt = fromUnix(startedAt)
		job.FinishedAt = fromUnix(finishedAt)
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (s *JobStore) AddLog(ctx context.Context, jobID string, message string) (JobLog, error) {
	db, err := s.openDatabase()
	if err != nil {
		return JobLog{}, err
	}
	defer db.Close()

	now := time.Now()
	result, err := db.ExecContext(ctx, `
		INSERT INTO job_logs (job_id, message, created_at_unix)
		VALUES (?, ?, ?)
	`, jobID, message, unix(now))
	if err != nil {
		return JobLog{}, err
	}

	sequence, err := result.LastInsertId()
	if err != nil {
		return JobLog{}, err
	}
	return JobLog{JobID: jobID, Sequence: sequence, Message: message, CreatedAt: now}, nil
}

func (s *JobStore) LogsAfter(ctx context.Context, jobID string, afterSequence int64) ([]JobLog, error) {
	db, err := s.openDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, `
		SELECT job_id, sequence, message, created_at_unix
		FROM job_logs
		WHERE job_id = ? AND sequence > ?
		ORDER BY sequence
	`, jobID, afterSequence)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []JobLog
	for rows.Next() {
		var log JobLog
		var createdAt int64
		if err := rows.Scan(&log.JobID, &log.Sequence, &log.Message, &createdAt); err != nil {
			return nil, err
		}
		log.CreatedAt = fromUnix(createdAt)
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return logs, nil
}

func migrateJobs(db *sql.DB) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			deployment_name TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			error TEXT NOT NULL DEFAULT '',
			created_at_unix INTEGER NOT NULL,
			started_at_unix INTEGER NOT NULL DEFAULT 0,
			finished_at_unix INTEGER NOT NULL DEFAULT 0
		)
	`); err != nil {
		return err
	}

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS job_logs (
			sequence INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id TEXT NOT NULL,
			message TEXT NOT NULL,
			created_at_unix INTEGER NOT NULL
		)
	`)
	return err
}

func unix(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

func fromUnix(seconds int64) time.Time {
	if seconds == 0 {
		return time.Time{}
	}
	return time.Unix(seconds, 0)
}
