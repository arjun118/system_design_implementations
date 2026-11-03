ALTER TABLE table_name DROP COLUMN column_name;

alter table jobs drop column lease_owner;
alter table jobs drop column lease_until;
alter table jobs drop column lease_token;
alter table jobs drop column last_error;

CREATE INDEX idx_jobs_lease_expired ON jobs (lease_until)
WHERE status = 'running';
