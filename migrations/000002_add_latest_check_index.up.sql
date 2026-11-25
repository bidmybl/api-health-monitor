CREATE INDEX idx_check_results_endpoint_completed_at
ON check_results (endpoint_id, completed_at DESC, id DESC);