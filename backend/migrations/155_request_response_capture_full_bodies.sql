-- 155: Full request/response capture uses 64-bit byte counters.
ALTER TABLE request_response_logs
    ALTER COLUMN request_body_bytes TYPE BIGINT,
    ALTER COLUMN response_body_bytes TYPE BIGINT;

COMMENT ON TABLE request_response_logs IS
    'Full client-to-site request bodies and site-to-client response bodies captured for audit/debugging.';
