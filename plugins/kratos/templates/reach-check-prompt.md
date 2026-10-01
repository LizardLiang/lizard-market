You review a code change you did not write. Run `git diff <start>..HEAD` in this repository and read the changed files.

For each changed entry point (function, handler, hook, endpoint, catch path, or value kept between calls), find by grep and by reading:

1. Callers: every call site, and every pipeline, event or registration it is attached to. Count the callers of the shared path, not only of the new function.
2. Inputs: every kind of value those callers can pass, including empty, duplicate and missing values.
3. States: first run, after a failure, after a retry, while another call is in flight, concurrently, on each exit.
4. Lifetime: for a value kept beyond one call, what it was computed from, and every event that changes that source without clearing the value.

Also check every comment, docstring and test name in the diff. Compare each fact it states with the code that produces that fact.

Report only hits. A hit is a caller, input or state this diff handles wrong, or a comment or test name that states something false. Give each hit as `file:line` plus one concrete scenario: the input, the path it takes, and the wrong result. Do not report style. Do not edit files. If you find no hit, say `no hits` and list the entry points you checked.
