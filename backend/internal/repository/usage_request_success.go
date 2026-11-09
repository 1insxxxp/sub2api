package repository

// usageRequestSuccessFilterUL classifies the final request, not its bill. A
// completed response may be free; a failed response may still have billable
// usage. Outcome evidence takes precedence over the legacy usage fallback.
// Match final errors by API key AND request-ID namespace, never by timestamp or
// account alone. Successful failover diagnostics (downstream status < 400) do
// not turn a completed request into a failure. The equality predicates keep
// the existing error request-ID indexes usable.
//
// All callers alias usage_logs as ul. This is only for request/latency metrics;
// financial and consumed-token totals must retain billed failures.
const usageRequestSuccessFilterUL = `(
  COALESCE(ul.request_type, 0) NOT IN (4, 6)
  AND COALESCE((
    SELECT uro.stream_completed
      AND uro.http_status >= 200 AND uro.http_status < 400
      AND uro.upstream_status >= 200 AND uro.upstream_status < 400
      AND uro.disconnect_source IN ('', 'none')
      AND (uro.has_text OR uro.has_tool_call OR uro.has_reasoning OR uro.has_media)
      AND LOWER(TRIM(uro.finish_reason)) NOT IN (
        'content_filter', 'safety', 'recitation', 'language', 'blocklist',
        'prohibited_content', 'spii', 'image_safety', 'image_prohibited_content', 'image_recitation'
      )
    FROM usage_response_outcomes uro
    WHERE uro.usage_log_id = ul.id AND uro.collector_version > 0
  ), ul.actual_cost > 0 OR ul.output_tokens > 0 OR ul.image_count > 0 OR ul.image_output_tokens > 0)
  AND NOT EXISTS (
    SELECT 1 FROM ops_error_logs final_error
    WHERE final_error.api_key_id = ul.api_key_id
      AND final_error.status_code >= 400 AND final_error.is_count_tokens = FALSE
      AND (
        (LEFT(ul.request_id, 7) = 'client:' AND final_error.client_request_id = SUBSTRING(ul.request_id FROM 8))
        OR (LEFT(ul.request_id, 7) <> 'client:' AND COALESCE(final_error.client_request_id, '') = ''
          AND final_error.request_id = CASE WHEN LEFT(ul.request_id, 6) = 'local:'
            THEN SUBSTRING(ul.request_id FROM 7) ELSE ul.request_id END)
      )
  )
)`
