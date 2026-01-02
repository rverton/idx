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

create table if not exists findings (
    id integer primary key autoincrement,
    run_id integer not null references runs(id),
    target_type text not null,
    target_name text not null,
    rule_name text not null,
    rule_description text not null,
    content_key text not null,
    location text not null,
    match text not null,
    detected_at integer not null default (strftime('%s', 'now')),
    unique(run_id, content_key, location, rule_name)
) strict;

create index if not exists idx_findings_run_id on findings(run_id);
create index if not exists idx_findings_target on findings(target_type, target_name);
create index if not exists idx_findings_rule_name on findings(rule_name);
