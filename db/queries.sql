-- name: ListRuns :many
select * from runs order by started_at desc;

-- name: InsertRun :one
insert into runs (started_at, status) values (?, 'running') returning id;

-- name: UpdateRun :exec
update runs set status = ?, finished_at = ?, error_message = ? where id = ?;

-- name: HasMemoryKey :one
select exists(select 1 from memories where key = ?) as has_key;

-- name: SetMemoryKey :exec
insert or ignore into memories (key, target_type, target_name, run_id) values (?, ?, ?, ?);
