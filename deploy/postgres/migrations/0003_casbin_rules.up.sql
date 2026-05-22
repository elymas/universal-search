-- Casbin RBAC policy storage.
-- SPEC-AUTH-002: casbin_rule table for policy persistence.
-- NFR-AUTH2-004: Isolated from hot-path pgxpool.

CREATE TABLE IF NOT EXISTS casbin_rule (
    id SERIAL PRIMARY KEY,
    ptype VARCHAR(12) DEFAULT '',
    v0 VARCHAR(128) DEFAULT '',
    v1 VARCHAR(128) DEFAULT '',
    v2 VARCHAR(128) DEFAULT '',
    v3 VARCHAR(128) DEFAULT '',
    v4 VARCHAR(128) DEFAULT '',
    v5 VARCHAR(128) DEFAULT ''
);
