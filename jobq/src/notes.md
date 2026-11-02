One distinction: if the status update itself fails—for example, the handler succeeded but Ack failed—don’t treat that as an ordinary handler retry. Log and surface it as an infrastructure/reporting error; the server may eventually redeliver the job, so handlers should be designed for possible duplicate execution.

## on LeaseDuration and HeartbeatInterval for WorkerConfig

1. HeartbeatInterval is better when = LeaseDuration/3

## migrations

ALTER TABLE jobs ADD COLUMN lease_owner TEXT;
ALTER TABLE jobs ADD COLUMN lease_until TIMESTAMPTZ;
ALTER TABLE jobs ADD COLUMN lease_token INT NOT NULL DEFAULT 0;
ALTER TABLE jobs ADD COLUMN last_error TEXT;

CREATE INDEX idx_jobs_lease_expired ON jobs (lease_until)
WHERE status = 'running';

add the index later
