-- Task 2 (#103): report generation timestamp, populated only on successful
-- completion (RecordCompletion), so nullable like commit_sha/report_versions
-- (0003_job_report_fields.sql). Distinct from finished_at: finished_at marks
-- when the job attempt stopped running, generated_at marks when the Report
-- payload itself was assembled, which JobStore.RecordCompletion's Completion
-- struct already models as two independent timestamps.

ALTER TABLE jobs
    ADD COLUMN generated_at TIMESTAMPTZ;
