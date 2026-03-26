-- Admin control-plane persistence schema

-- Sources represent external API sources (OpenAPI specs, etc.)
CREATE TABLE IF NOT EXISTS admin_sources (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    display_name TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('draft', 'validated', 'publishable', 'archived')),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
