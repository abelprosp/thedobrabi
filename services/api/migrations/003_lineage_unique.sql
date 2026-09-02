CREATE UNIQUE INDEX IF NOT EXISTS idx_lineage_edges_uniq
    ON lineage_edges (from_id, to_id, relation);
