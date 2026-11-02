alter table jobs add column lease_owner TEXT;
alter table jobs add column lease_until TIMESTAMPTZ;
alter table jobs add column lease_token INT NOT NULL DEFAULT 0;
alter table jobs add column last_error TEXT;
