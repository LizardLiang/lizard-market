# Kratos Code Review — React Rules

> Extends `default.md`. These rules apply to any file using React (`.jsx`, `.tsx`, components with JSX).
> Project-specific overrides live in `.claude/.Arena/review-rules/react.md`.

Based on `eslint-plugin-react` and `eslint-plugin-react-hooks` recommendations.

---

## Hooks Rules (`eslint-plugin-react-hooks`)

These are correctness issues — Tier 1 violations. Treat as `[BLOCKER]`.

### rules-of-hooks

Hooks must only be called:
- At the top level of a React function component or custom hook
- Never inside loops, conditions, or nested functions

```tsx
// BLOCKER
function Bad() {
  if (condition) {
    const [state, setState] = useState(false); // conditional hook call
  }
}

// Correct
function Good() {
  const [state, setState] = useState(false);
  if (condition) { /* use state here */ }
}
```

### exhaustive-deps

Every value from the component scope used inside `useEffect`, `useCallback`, `useMemo` must appear in the dependency array. Missing deps cause stale closure bugs.

```tsx
// BLOCKER — userId is used but not in deps
useEffect(() => {
  fetchUser(userId);
}, []); // missing userId

// Correct
useEffect(() => {
  fetchUser(userId);
}, [userId]);
```

**Exception**: Stable refs (`useRef`, `setState` from `useState`, `dispatch` from `useReducer`) do not need to be in deps — they are guaranteed stable.

**Common patterns to flag**:
- Empty `[]` deps with values used inside the callback
- `useCallback` with no deps that captures changing values
- `useMemo` dependencies that include object/array literals (recreated every render)

---

## Component Rules (`eslint-plugin-react`)

### react/jsx-key — `[BLOCKER]`

Every element in a list render must have a unique, stable `key` prop.

```tsx
// BLOCKER
items.map(item => <Item data={item} />)

// Correct
items.map(item => <Item key={item.id} data={item} />)
```

**Flag**: `key={index}` is acceptable only when the list is static (never reordered, never filtered). If items can be reordered or filtered, `key={index}` is a `[WARNING]`.

### react/no-array-index-key — `[WARNING]`

Using array index as key in dynamic lists causes reconciliation bugs (wrong component state after reorder).

### react/no-direct-mutation-state — `[BLOCKER]`

Never mutate state directly. Always use the setter.

```tsx
// BLOCKER
this.state.count = 5;

// Correct
this.setState({ count: 5 });
// or with hooks:
setCount(5);
```

### react/no-unstable-nested-components — `[WARNING]`

Do not define components inside other components' render. Creates a new component type on every render, causing full unmount/remount.

```tsx
// WARNING
function Parent() {
  function Child() { return <div />; } // redefined every render
  return <Child />;
}

// Correct — define outside
function Child() { return <div />; }
function Parent() { return <Child />; }
```

### react/display-name — `[WARNING]`

Components should have a display name for React DevTools and error boundaries. Anonymous arrow functions assigned to variables usually get it automatically; watch for memoized components.

```tsx
// WARNING — no display name in DevTools
const MyComponent = React.memo(() => <div />);

// Correct
const MyComponent = React.memo(function MyComponent() { return <div />; });
```

### react/no-deprecated — `[WARNING]`

Flag use of deprecated React APIs:
- `componentWillMount` → `componentDidMount`
- `componentWillReceiveProps` → `getDerivedStateFromProps` or `useEffect`
- `componentWillUpdate` → `getSnapshotBeforeUpdate` or `useEffect`
- `ReactDOM.render` → `ReactDOM.createRoot` (React 18+)
- `findDOMNode` → `ref` callbacks

### react/self-closing-comp — `[SUGGESTION]`

Components and elements with no children should self-close.

```tsx
// SUGGESTION
<MyComponent></MyComponent>

// Correct
<MyComponent />
```

---

## State Management Rules

### Stale closure trap — `[BLOCKER]`

Using event handlers or callbacks that capture outdated state/props:

```tsx
// BLOCKER — onClick always sees initial count value
const [count, setCount] = useState(0);
useEffect(() => {
  const id = setInterval(() => {
    setCount(count + 1); // stale count
  }, 1000);
  return () => clearInterval(id);
}, []); // missing count dep

// Correct — use functional updater
setCount(prev => prev + 1);
```

### Object/array state mutation — `[BLOCKER]`

Never mutate state objects or arrays directly.

```tsx
// BLOCKER
const [items, setItems] = useState([]);
items.push(newItem); // mutation
setItems(items);

// Correct
setItems(prev => [...prev, newItem]);
```

---

## Performance Rules

### Unnecessary re-renders — `[WARNING]`

- Inline object/array/function literals as props trigger re-renders on every parent render
- Wrap with `useMemo` / `useCallback` when passed to memoized children

```tsx
// WARNING — new object reference every render
<Child style={{ color: 'red' }} />

// Correct if Child is memoized
const style = useMemo(() => ({ color: 'red' }), []);
<Child style={style} />
```

**Note**: Don't flag this if the child is not memoized — the fix would have no effect.

### useEffect with no cleanup for subscriptions — `[WARNING]`

Effects that set up subscriptions, event listeners, or timers must return a cleanup function.

```tsx
// WARNING — memory leak
useEffect(() => {
  window.addEventListener('resize', handler);
}, []);

// Correct
useEffect(() => {
  window.addEventListener('resize', handler);
  return () => window.removeEventListener('resize', handler);
}, []);
```

---

## Accessibility (a11y) — `[WARNING]`

Flag these common issues:

- `<img>` without `alt` attribute
- Interactive elements (`div`, `span`) with `onClick` but no `role` or keyboard handler
- Form inputs without associated `<label>` or `aria-label`
- Empty `href` on `<a>` tags (`href="#"` is acceptable for SPAs with proper `onClick`)
- `<button>` with no accessible text (no text content, no `aria-label`)

---

## TypeScript + React — `[WARNING]`

When TypeScript is used:

- Component props should have explicit types (interface or type alias), not `any`
- Event handler types should use React's built-in types: `React.MouseEvent`, `React.ChangeEvent<HTMLInputElement>`, etc.
- `useRef` should be typed: `useRef<HTMLDivElement>(null)`, not `useRef(null)`
- Context values should be typed with generics: `createContext<MyType>(defaultValue)`

---

## What Does NOT Count as a Violation

Do not flag:
- Choosing `useState` vs `useReducer` (both are valid depending on complexity)
- Choosing class components vs function components (both still valid)
- Preferring Tailwind vs CSS Modules vs styled-components (project style choice)
- Component file organization (one-file vs directory) unless project has a convention
