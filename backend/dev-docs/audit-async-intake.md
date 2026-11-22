# Audit SDK ordered asynchronous handoff

Main-site v0.2.21-tokenfree.dev.2 uses SDK
`273efdd8aed60692a8cf2853f78ab0a2f231f59c`, including ordered capture,
bounded shutdown, resident queue indexes and background snapshot recovery.
The SDK reuses existing Relay connection settings. New memory/index settings
require the matching Relay and Console version; upgrading the main-site image
does not update those services or change their saved policies.

The gateway calls `CaptureRequest`, `Part`, `ObserveUpstream` and `Complete`.
Those calls copy mutable bytes/headers, establish context and assign request
sequence numbers synchronously. SDK channels and their receiver goroutines
then own ordering, storage, sending, retries and confirmations. No gateway
payload queue, disk spool or retry worker is added. A full SDK channel expands;
the default memory policy continues to allow intake. An explicitly configured drop policy can reject new frames above the estimated
memory limit. Existing collection
policy, credentials, connection settings and retention are unchanged.
The compatible `/responses` handler refreshes its saved context after capture
starts, so account selection and forwarding retain the newly created session.

`Complete` defers final response metadata and end until already-started body
reads/closes have submitted their last observations. It returns without waiting
for those operations. Gin writers, contexts and live headers are never accessed
by SDK background workers. Upstream I/O and business errors remain synchronous
at the same transport seam; capture failures cannot replace their result.
Body close frames also wait behind already-started reads, so Relay's assembler
cannot discard a late final fragment after it has processed close.

SDK initialization happens before accepting HTTP traffic. Shutdown first drains
HTTP requests and then calls SDK `Shutdown` using the **same existing five-second
deadline**. SDK stops accepting new capture operations, joins registered reads,
drains its channels and waits for accepted data to be persisted or confirmed.
It need not wait for Relay to confirm data already safely journaled. A timeout
returns a logged error; SDK cleanup continues in the background while the host
finishes its existing cleanup. A stuck filesystem cannot indefinitely hold this
call. Forced exit or expiration can lose memory-only frames, pending response
metadata/end and observations from unfinished requests. Persisted, unconfirmed
frames resume on restart; they must not be deleted or have a second process open
the spool while the first SDK is still closing.

The gitlink and `audit-sdk.lock.json` pin a committed SDK. The build verifier
checks every production Go source plus module files, rejects modified/missing
or extra Go files, and checks the gitlink when building from Git. Both Docker
source builds and GoReleaser verify these inputs; SDK revision is injected into
the executable and shown by `sub2api -version`. The release workflow verifies
the linked SDK revision in the built executable before publishing its image.

This is asynchronous handoff, not a zero-cost promise. Copies, short sequencing
locks, channel allocation, scheduling and garbage collection still share the
host process. Continued backlog can exhaust its memory. The integration tests
exercise controlled stalls and concurrency, not an arbitrary production load.
