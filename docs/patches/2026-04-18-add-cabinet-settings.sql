alter table cabinets
    add column settings jsonb default '{}'::jsonb not null;
