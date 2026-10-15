CREATE TABLE IF NOT EXISTS monitored_endpoints (
  id bigserial PRIMARY KEY,
  url text UNIQUE NOT NULL,
  check_interval_seconds integer NOT NULL DEFAULT 60,
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now (),
  updated_at timestamptz NOT NULL DEFAULT now (),
  CONSTRAINT chk_interval_positive CHECK (check_interval_seconds > 0)
);

CREATE TABLE IF NOT EXISTS check_results (
  id bigserial PRIMARY KEY,
  endpoint_id bigint NOT NULL,
  started_at timestamptz NOT NULL DEFAULT now (),
  completed_at timestamptz NOT NULL DEFAULT now (),
  http_status_code smallint,
  response_time_ms integer NOT NULL,
  is_success BOOLEAN GENERATED ALWAYS AS (
    COALESCE(http_status_code BETWEEN 200 AND 299, FALSE)
  ) STORED,
  error_message text,
  CONSTRAINT fk_endpoint FOREIGN KEY (endpoint_id) REFERENCES monitored_endpoints (id) ON DELETE CASCADE,
  CHECK (
    http_status_code IS NULL
    OR (
      http_status_code >= 100
      AND http_status_code <= 599
    )
  ),
  CHECK (response_time_ms >= 0),
  CHECK (completed_at >= started_at),
  CHECK (
    (
      http_status_code IS NULL
      AND error_message IS NOT NULL
    )
    OR (
      http_status_code IS NOT NULL
      AND error_message IS NULL
    )
  )
);

CREATE INDEX IF NOT EXISTS idx_check_results_endpoint_started_at ON check_results (endpoint_id, started_at);