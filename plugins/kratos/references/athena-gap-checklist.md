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
