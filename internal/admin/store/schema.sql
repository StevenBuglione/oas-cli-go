-- Admin control-plane persistence schema

-- Sources represent external API sources (OpenAPI specs, etc.)
CREATE TABLE IF NOT EXISTS admin_sources (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    display_name TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

-- Bundles are access packages that can be assigned to principals (users, groups)
-- Full bundle and principal management will be added in future tasks
CREATE TABLE IF NOT EXISTS admin_bundles (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);
