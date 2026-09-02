-- Flow materialization and gateway token index

ALTER TABLE flows ADD COLUMN IF NOT EXISTS output_dataset_id UUID REFERENCES datasets(id) ON DELETE SET NULL;
ALTER TABLE flows ADD COLUMN IF NOT EXISTS layout_json JSONB NOT NULL DEFAULT '{}';

CREATE INDEX IF NOT EXISTS idx_flows_output_dataset ON flows(output_dataset_id);
