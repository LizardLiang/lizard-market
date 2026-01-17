---
name: code-taste
description: Reference guide for Linus Torvalds' "Good Taste" principles - the standard against which all code is judged
---

# THE GOOD TASTE STANDARD

This skill defines what "good taste" means in code, based on Linus Torvalds' famous TED 2016 example and decades of kernel development wisdom.

## THE CORE INSIGHT

> "Sometimes you can see a problem in a different way and rewrite it so that a special case goes away and becomes the normal case, and that's good code."

Good taste is NOT about:
- Following style guides
- Using the latest patterns
- Writing "clean" code by some metric

Good taste IS about:
- Seeing problems from angles that eliminate complexity
- Choosing data structures that make logic obvious
- Writing code where the happy path is self-evident

## THE CANONICAL EXAMPLE

### Bad Taste (CS101 Approach)

```c
void remove_list_entry(List* list, Entry* entry) {
    Entry* prev = NULL;
    Entry* walk = list->head;

    while (walk != entry) {
        prev = walk;
        walk = walk->next;
    }

    // THE SPECIAL CASE - THIS IS BAD TASTE
    if (prev == NULL) {
        list->head = entry->next;  // Removing first element
    } else {
        prev->next = entry->next;  // Removing middle/end element
    }
}
```

**Why it's bad:** The if-statement exists because we didn't think about the problem correctly. We're treating the first element as "special" when it doesn't need to be.

### Good Taste (Linus Approach)

```c
void remove_list_entry(List* list, Entry* entry) {
    Entry** indirect = &list->head;

    while (*indirect != entry) {
        indirect = &(*indirect)->next;
    }

    // NO SPECIAL CASE - SAME CODE HANDLES ALL CASES
    *indirect = entry->next;
}
```

**Why it's good:** By using an indirect pointer (pointer to pointer), we eliminate the special case entirely. The first element and middle elements are handled identically.

## TASTE PRINCIPLES

### Principle 1: Eliminate Edge Cases Through Design

**Bad:**
```javascript
function getFirst(arr) {
    if (arr.length === 0) return null;
    return arr[0];
}
```

**Better:** Design your system so empty arrays don't reach this function, or use a data structure that can't be empty.

**Best:** Question why you need this function at all.

### Principle 2: Reduce Indentation Ruthlessly

**Bad:**
```javascript
function process(data) {
    if (data) {
        if (data.items) {
            if (data.items.length > 0) {
                for (const item of data.items) {
                    if (item.valid) {
                        // Logic buried 5 levels deep
                    }
                }
            }
        }
    }
}
```

**Good:**
```javascript
function process(data) {
    if (!data?.items?.length) return;

    data.items
        .filter(item => item.valid)
        .forEach(processItem);
}
```

### Principle 3: One Function, One Purpose

**Bad:**
```javascript
function handleUser(action, user, data, options) {
    if (action === 'create') { /* 50 lines */ }
    else if (action === 'update') { /* 50 lines */ }
    else if (action === 'delete') { /* 30 lines */ }
    else if (action === 'fetch') { /* 40 lines */ }
}
```

**Good:**
```javascript
function createUser(data) { /* focused */ }
function updateUser(user, data) { /* focused */ }
function deleteUser(user) { /* focused */ }
function fetchUser(id) { /* focused */ }
```

### Principle 4: Data Structures Dictate Logic

Choose data structures that make operations trivial:

| Operation | Bad Choice | Good Choice |
|-----------|------------|-------------|
| Lookup by ID | Array + find() | Map/Object |
| Check membership | Array + includes() | Set |
| Ordered unique items | Manual dedup | Array + Set |
| Hierarchical data | Flat array + parentId | Tree structure |

### Principle 5: Names Reveal Intent

**Bad:**
```javascript
const d = getData();
const p = process(d);
const r = format(p);
```

**Good:**
```javascript
const userProfile = fetchUserProfile(userId);
const validatedProfile = validateProfileFields(userProfile);
const displayHtml = renderProfileCard(validatedProfile);
```

### Principle 6: Comments Explain "Why", Code Explains "What"

**Bad:**
```javascript
// Increment counter
i++;
```

**Good:**
```javascript
// Skip header row containing column labels
i++;
```

**Best:** (no comment needed)
```javascript
const dataRowIndex = headerRowCount;
```

### Principle 7: Error Handling Clarifies, Not Obscures

**Bad:**
```javascript
try {
    const a = await getA();
    try {
        const b = await getB(a);
        try {
            return await getC(b);
        } catch (e) { handleCError(e); }
    } catch (e) { handleBError(e); }
} catch (e) { handleAError(e); }
```

**Good:**
```javascript
const a = await getA().catch(handleAError);
if (!a) return;

const b = await getB(a).catch(handleBError);
if (!b) return;

return getC(b).catch(handleCError);
```

## THE TASTE CHECKLIST

When reviewing code, evaluate each item:

| Check | Question | Pass/Fail |
|-------|----------|-----------|
| Edge Cases | Could better design eliminate if-statements? | |
| Indentation | Deeper than 3 levels? | |
| Function Focus | Does it do exactly ONE thing? | |
| Naming | Can you understand without comments? | |
| Data Structure | Is the right structure used? | |
| Happy Path | Is normal flow obvious at a glance? | |
| Complexity | Essential or accidental? | |

## SCORING GUIDE

| Score | Description |
|-------|-------------|
| 10/10 | Elegant. Special cases eliminated. Would merge immediately. |
| 8-9 | Good taste evident. Minor improvements possible. |
| 6-7 | Functional but lacks elegance. Refactoring recommended. |
| 4-5 | Works but painful. Technical debt accumulating. |
| 2-3 | Poor taste. Major refactoring required. |
| 0-1 | Delete and start over. Question how this passed review. |

## FINAL WORD

> "Any programmer can write code the computer understands. Good ones write code humans can understand."

Good taste isn't about cleverness—it's about clarity.
It's not about following rules—it's about understanding problems deeply enough to find simple solutions.

The best code looks obvious in hindsight. That's the mark of good taste.
