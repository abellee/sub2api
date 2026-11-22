export default {
  logCollection: {
    title: 'Log collection', timing: 'Latency diagnostics',
    description: 'Collect requests matching all three IDs. Each collection gets a unique ID. Only timings and sizes are stored; no prompts, images, or credentials.',
    user_id: 'User ID', group_id: 'Group ID', account_id: 'Account ID',
    start: 'Start collection', stop: 'Stop collection', active: 'Collecting', stopped: 'Stopped',
    limits: 'Up to 10 active collections and 10,000 records per collection, then automatic stop. Stopping discards pending records. Starts synchronize across instances in about 2 seconds without a restart.',
    confirm: 'Collect latency for user {user}, group {group}, account {account}? Send new requests after starting.',
    collectionId: 'Collection ID (blank for history)', requestId: 'Request ID (client: prefix supported)',
    scope: 'User {user} · Group {group} · Account {account}', count: '{count} records saved',
    empty: 'No collections', noRecords: 'No records yet. Records are saved after requests finish. Refresh to check.', more: 'Load older records',
    records: 'Timing records', limitReached: '10,000 record limit reached',
    ready: 'Collection configuration ready on this instance', unready: 'Configuration unavailable on this instance; collection suspended',
    health: 'This instance: {queue} queued · {dropped} dropped · {failed} write failures · {refresh} refresh failures (counters reset on restart)',
    reading: 'Times start at application ingress. Stages may overlap and cannot be summed. A dash means unobserved, not zero. HTTP tracks connection, upload and first byte; OpenAI Responses also tracks first event and content. WS records are saved when the connection closes; turns appear as attempts. First writes may be heartbeats. Upstream internal queueing and client rendering are not observable.',
    bytes: 'Request bytes', attempt: 'Attempt', stage: 'Stage', elapsed: 'Since request start', duration: 'Stage duration',
    reused: 'Reused connection', newConnection: 'New connection', failed: 'Transport failed',
    truncated: 'Per-record limit reached (32 attempts / 256 stages); later details omitted.',
    invalid: 'Enter three valid positive integer IDs', error: 'Operation failed. Please retry.',
    stages: {
      auth_complete: 'Authentication complete', body_read_start: 'Request body read started', body_read_complete: 'Request body read complete', body_read_failed: 'Request body read failed',
      account_selected: 'Account selected', forward_prepare_start: 'Preparing upstream request', upstream_start: 'Upstream attempt started',
      connection_wait_start: 'Waiting for connection', connection_ready: 'Connection ready (includes setup)',
      dns_start: 'DNS started', dns_done: 'DNS finished', connect_start: 'TCP connection started', connect_done: 'TCP connection finished',
      tls_start: 'TLS handshake started', tls_done: 'TLS handshake finished', request_headers_sent: 'Request headers sent',
      upload_complete: 'Request sent (since connection ready)', upload_failed: 'Request upload failed',
      first_response_byte: 'First upstream byte (wait after upload)', upstream_headers: 'Upstream headers received', upstream_error: 'Upstream transport failed',
      first_body_byte: 'First upstream body byte read', upstream_body_complete: 'Upstream body read complete',
      first_stream_event: 'First stream event', first_semantic_output: 'First content (wait after upload)',
      downstream_write_start: 'First downstream write started', downstream_first_write: 'First downstream write returned', downstream_first_flush: 'First downstream flush returned',
      request_complete: 'Request complete', user_slot_start: 'User concurrency check started', user_slot_done: 'User concurrency check finished',
      account_slot_start: 'Account concurrency check started', account_slot_done: 'Account concurrency check finished',
      ops_auth_latency_ms: 'Existing authentication metric', ops_routing_latency_ms: 'Routing and queueing metric',
      ops_upstream_latency_ms: 'Upstream headers latency metric', ops_response_latency_ms: 'Response processing metric',
      ops_time_to_first_token_ms: 'Existing TTFT metric', ops_openai_ws_queue_wait_ms: 'WS queue wait', ops_openai_ws_conn_pick_ms: 'WS connection acquisition'
    }
  }
}
