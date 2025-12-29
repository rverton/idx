-- name: ListRuns :many
select * from runs order by started_at desc;

-- name: InsertRun :one
insert into runs (id, started_at, status) values (?, ?, 'running') returning id;

-- name: UpdateRun :exec
update runs set status = ?, finished_at = ?, error_message = ? where id = ?;
