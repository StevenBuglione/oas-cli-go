-- Admin control-plane persistence schema

-- Sources represent external API sources (OpenAPI specs, etc.)
CREATE TABLE IF NOT EXISTS admin_sources (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    display_name TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('draft', 'validated', 'publishable', 'archived')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- Bundles represent access packages that can be assigned to principals
CREATE TABLE IF NOT EXISTS admin_bundles (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

-- Bundle assignments link bundles to principals (users or groups)
CREATE TABLE IF NOT EXISTS admin_bundle_assignments (
    id TEXT PRIMARY KEY,
    bundle_id TEXT NOT NULL REFERENCES admin_bundles(id) ON DELETE CASCADE,
    principal_type TEXT NOT NULL CHECK (principal_type IN ('user', 'group')),
    principal_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE(bundle_id, principal_type, principal_id)
);
