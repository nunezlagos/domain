



CREATE TABLE IF NOT EXISTS intake_attachments (
    id          uuid PRIMARY KEY,
    intake_id   uuid NOT NULL,
    filename    varchar(255) NOT NULL,
    mime_type   varchar(127) NOT NULL,
    size_bytes  bigint NOT NULL,
    s3_key      text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    status      text NOT NULL DEFAULT 'active'
);


CREATE TABLE IF NOT EXISTS project_links (
    id                uuid PRIMARY KEY,
    organization_id   uuid NOT NULL,
    project_id        uuid NOT NULL,
    linked_project_id uuid NOT NULL,
    access_level      varchar(32) NOT NULL DEFAULT 'read',
    created_by        uuid NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    status            text NOT NULL DEFAULT 'active'
);


CREATE TABLE IF NOT EXISTS event_log (
    id              uuid PRIMARY KEY,
    type            varchar(64) NOT NULL,
    organization_id uuid,
    project_id      uuid,
    payload         jsonb,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    status          text NOT NULL DEFAULT 'active'
);


CREATE TABLE IF NOT EXISTS llm_semantic_cache (
    id               varchar(64) PRIMARY KEY,
    organization_id  uuid NOT NULL,
    provider         varchar(32) NOT NULL,
    model            varchar(64) NOT NULL,
    params_hash      varchar(64) NOT NULL,
    prompt_hash      varchar(64) NOT NULL,
    prompt_preview   text,
    response         jsonb NOT NULL,
    tokens           integer NOT NULL DEFAULT 0,
    hit_count        integer NOT NULL DEFAULT 0,
    prompt_embedding vector(1536),
    created_at       timestamptz NOT NULL DEFAULT now(),
    last_used_at     timestamptz,
    updated_at       timestamptz NOT NULL DEFAULT now(),
    status           text NOT NULL DEFAULT 'active'
);


CREATE TABLE IF NOT EXISTS domain_query_stats_history (
    id               bigserial PRIMARY KEY,
    captured_at      timestamptz NOT NULL DEFAULT now(),
    query_text       text NOT NULL,
    queryid          bigint NOT NULL,
    calls            bigint NOT NULL,
    total_exec_time  double precision NOT NULL,
    mean_exec_time   double precision NOT NULL,
    rows_returned    bigint NOT NULL,
    shared_blks_hit  bigint NOT NULL,
    shared_blks_read bigint NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    status           text NOT NULL DEFAULT 'active'
);
