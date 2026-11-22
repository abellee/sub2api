# Cost precheck cache history

This module contains only the bounded, process-local previous-request cache history used by the host cost precheck. It uses the standard library and skips optional history on contention. It does not reserve balances or scan usage logs.
