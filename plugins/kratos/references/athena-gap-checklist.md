# Gap Analysis Checklist

Check coverage across these areas. Each unchecked item is a gap.

**Restrictions & Constraints**

- [ ] Performance requirements (speed, scale, volume limits)
- [ ] Security requirements (authentication, authorization, encryption, compliance)
- [ ] Platform/browser/device constraints
- [ ] Integration constraints (what systems it must work with)
- [ ] Budget/timeline/resource constraints

**Use Cases & Edge Cases**

- [ ] Primary happy path clearly defined
- [ ] Error scenarios covered (what happens when X fails?)
- [ ] Edge cases identified (empty state, max limits, concurrent users, timeouts)
- [ ] User roles and permissions considered
- [ ] State transitions defined (what happens before/during/after)

**Behavioral Lifecycle** (for any stateful/CRUD-ish capability — enumerate per verb, don't collapse to one)

- [ ] Create / grant — how does the thing come into being? (e.g. for a permission feature: *how is permission granted*, by whom, through what surface)
- [ ] Read / list / inspect — how is current state viewed or queried?
- [ ] Update / change — can it be modified after creation, and how?
- [ ] Revoke / delete — how is it removed or turned off, and what cascades?
- [ ] Enforce / check — where and when is the rule applied at runtime?
- [ ] Defaults / initial state — what holds before anything is configured?
- [ ] Roles / scopes — does behavior differ by actor, tier, or scope?

> Covering enforcement while ignoring the grant/revoke paths is the classic gap. If the feature is stateful, every verb above is a gap until explicitly answered or marked out of scope.

**Data & Integration**

- [ ] What data is involved and where does it come from?
- [ ] What data is created, modified, or deleted?
- [ ] How does this interact with existing features?
- [ ] External dependencies identified?

**Users & Measurement**

- [ ] Who are ALL the users affected (not just primary)?
- [ ] How will success be measured with specific metrics?
- [ ] What is explicitly OUT of scope?
- [ ] What happens to existing functionality?

**Rollout & Migration**

- [ ] Backwards compatibility — do existing clients/data/configs keep working?
- [ ] Migration path for existing data or users (and rollback if it fails)
- [ ] Rollout strategy — feature flag, staged rollout, or all-at-once?

**Operability**

- [ ] Observability — what logging/metrics/alerts tell you it works (or broke) in production?
- [ ] Concurrency & idempotency — simultaneous actions on the same resource, retries, double-submits
