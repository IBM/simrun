DROP INDEX IF EXISTS idx_connectors_default_per_type;
CREATE UNIQUE INDEX idx_connectors_default_per_type
ON connectors(type)
WHERE is_default = true AND type IN ('aws', 'gcp', 'azure');
