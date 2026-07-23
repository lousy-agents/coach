-- Task 2 (#103): report-assembly fields on jobs, populated only on
-- successful completion (RecordCompletion), so both columns are nullable.
-- commit_sha is the resolved commit the job scanned; report_versions is the
-- Report.versions payload (analyzer + rubric id->version pairs) frozen at
-- completion time so later rubric/analyzer upgrades cannot retroactively
-- change an already-generated report's versions.

ALTER TABLE jobs
    ADD COLUMN commit_sha       TEXT,
    ADD COLUMN report_versions  JSONB;
