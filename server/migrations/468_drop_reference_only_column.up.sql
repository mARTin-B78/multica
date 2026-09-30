-- Drop reference_only, the contract half of the pair MUL-7072 started. Since that
-- release a bare body mention of an issue key writes no link row at all, and no
-- query reads or writes this column. It may only run once every instance of the
-- previous release is gone: an older instance still names the column in its link
-- INSERT and would fail on every link write against this schema.
--
-- The DELETE repeats as a mop-up. Migration 462 cleared the historical hidden
-- rows, but instances still serving during that rollout could write a few more,
-- and once the column is gone such a row becomes indistinguishable from a real
-- link — visible in the issue's PR list and, while its PR is in flight, blocking
-- the issue from auto-advancing.
DO $migration$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'issue_pull_request'
          AND column_name = 'reference_only'
    ) THEN
        DELETE FROM issue_pull_request WHERE reference_only;
        ALTER TABLE issue_pull_request DROP COLUMN reference_only;
    END IF;

    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'issue_vcs_pull_request'
          AND column_name = 'reference_only'
    ) THEN
        DELETE FROM issue_vcs_pull_request WHERE reference_only;
        ALTER TABLE issue_vcs_pull_request DROP COLUMN reference_only;
    END IF;
END
$migration$;
