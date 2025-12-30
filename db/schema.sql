create table if not exists runs (
    id integer primary key autoincrement,
    status text not null,
    error_message text,
    started_at integer not null default (strftime('%s', 'now')),
    finished_at integer
) strict;

create index if not exists idx_runs_started_at on runs(started_at desc);

create table if not exists memories (
    key text primary key,
    target_type text not null,
    target_name text not null,
    analyzed_at integer not null default (strftime('%s', 'now')),
    run_id integer references runs(id)
) strict;

create index if not exists idx_memories_target on memories(target_type, target_name);
create index if not exists idx_memories_run_id on memories(run_id);
