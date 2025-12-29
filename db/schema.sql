create table if not exists runs (
    id integer primary key autoincrement,
    status text not null,
    error_message text,
    started_at integer not null default (strftime('%s', 'now')),
    finished_at integer
) strict;
