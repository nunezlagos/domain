








CREATE TABLE project_tickets (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  client_id       UUID REFERENCES clients(id) ON DELETE SET NULL,


  key             VARCHAR(40) NOT NULL,

  number          INTEGER NOT NULL,
  title           VARCHAR(255) NOT NULL,
  description_md  TEXT,
  description_tsv TSVECTOR GENERATED ALWAYS AS (
    to_tsvector('spanish', coalesce(title,'') || ' ' || coalesce(description_md,''))
  ) STORED,



  issue_type      VARCHAR(20) NOT NULL DEFAULT 'task'
    CHECK (issue_type IN ('bug','feature','requirement','task','epic','improvement','spike')),
  status          VARCHAR(20) NOT NULL DEFAULT 'backlog'
    CHECK (status IN ('backlog','todo','in_progress','in_review','blocked','done','cancelled')),
  priority        VARCHAR(20) NOT NULL DEFAULT 'medium'
    CHECK (priority IN ('trivial','low','medium','high','critical')),
  assignee_id     UUID REFERENCES users(id) ON DELETE SET NULL,
  reporter_id     UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  labels          TEXT[] NOT NULL DEFAULT '{}',

  external_provider VARCHAR(20)
    CHECK (external_provider IN ('jira','github','gitlab','linear','azure_devops')),
  external_id       VARCHAR(80),
  external_url      VARCHAR(500),
  external_synced_at TIMESTAMPTZ,

  parent_id       UUID REFERENCES project_tickets(id) ON DELETE SET NULL,

  estimated_hours NUMERIC(6,2),
  actual_hours    NUMERIC(6,2),

  due_date        DATE,
  started_at      TIMESTAMPTZ,
  completed_at    TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at      TIMESTAMPTZ,

  UNIQUE (organization_id, project_id, number),
  UNIQUE (organization_id, project_id, key)
);

CREATE INDEX project_tickets_org_project_status_idx
  ON project_tickets (organization_id, project_id, status, updated_at DESC)
  WHERE deleted_at IS NULL;
CREATE INDEX project_tickets_assignee_idx
  ON project_tickets (organization_id, assignee_id, status)
  WHERE deleted_at IS NULL AND assignee_id IS NOT NULL;
CREATE INDEX project_tickets_external_idx
  ON project_tickets (external_provider, external_id)
  WHERE external_id IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX project_tickets_parent_idx
  ON project_tickets (parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX project_tickets_tsv_idx ON project_tickets USING gin(description_tsv);
CREATE INDEX project_tickets_labels_idx ON project_tickets USING gin(labels);

CREATE TRIGGER set_updated_at_project_tickets
  BEFORE UPDATE ON project_tickets
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();


CREATE TABLE project_ticket_comments (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  ticket_id    UUID NOT NULL REFERENCES project_tickets(id) ON DELETE CASCADE,
  author_id   UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  body_md     TEXT NOT NULL,

  external_id VARCHAR(80),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at  TIMESTAMPTZ
);
CREATE INDEX project_ticket_comments_ticket_idx
  ON project_ticket_comments (ticket_id, created_at)
  WHERE deleted_at IS NULL;

CREATE TRIGGER set_updated_at_project_ticket_comments
  BEFORE UPDATE ON project_ticket_comments
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();


CREATE TABLE project_ticket_status_history (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  ticket_id     UUID NOT NULL REFERENCES project_tickets(id) ON DELETE CASCADE,
  from_status  VARCHAR(20),  -- null en la transición inicial (creación)
  to_status    VARCHAR(20) NOT NULL,
  changed_by   UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  note         TEXT,
  changed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX project_ticket_status_history_ticket_idx
  ON project_ticket_status_history (ticket_id, changed_at DESC);


ALTER TABLE project_tickets ENABLE ROW LEVEL SECURITY;
ALTER TABLE project_tickets FORCE ROW LEVEL SECURITY;
CREATE POLICY project_tickets_org_isolation ON project_tickets
  FOR ALL TO PUBLIC
  USING (organization_id = current_org_id())
  WITH CHECK (organization_id = current_org_id());

ALTER TABLE project_ticket_comments ENABLE ROW LEVEL SECURITY;
ALTER TABLE project_ticket_comments FORCE ROW LEVEL SECURITY;
CREATE POLICY project_ticket_comments_via_issue ON project_ticket_comments
  FOR ALL TO PUBLIC
  USING (
    EXISTS (SELECT 1 FROM project_tickets i
            WHERE i.id = ticket_id AND i.organization_id = current_org_id())
  )
  WITH CHECK (
    EXISTS (SELECT 1 FROM project_tickets i
            WHERE i.id = ticket_id AND i.organization_id = current_org_id())
  );

ALTER TABLE project_ticket_status_history ENABLE ROW LEVEL SECURITY;
ALTER TABLE project_ticket_status_history FORCE ROW LEVEL SECURITY;
CREATE POLICY project_ticket_status_history_via_issue ON project_ticket_status_history
  FOR ALL TO PUBLIC
  USING (
    EXISTS (SELECT 1 FROM project_tickets i
            WHERE i.id = ticket_id AND i.organization_id = current_org_id())
  )
  WITH CHECK (
    EXISTS (SELECT 1 FROM project_tickets i
            WHERE i.id = ticket_id AND i.organization_id = current_org_id())
  );

GRANT SELECT, INSERT, UPDATE, DELETE ON project_tickets TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON project_ticket_comments TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON project_ticket_status_history TO app_user;
GRANT ALL ON project_tickets, project_ticket_comments, project_ticket_status_history TO app_admin;
