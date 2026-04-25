# MaxClaw UI Refactor Implementation Plan

> **Goal:** Migrate the Electron desktop UI from warm earth-tone glassmorphism to X.com-inspired minimalism (black/white/gray, high density, clean).

**Architecture:** Replace CSS variable system and Tailwind mapping, switch theme mode from `class` to `data-theme`, tighten layout spacing, update component styles to use the new token set. No functional changes.

---

## Task 1: Design Token Foundation

**Files:**
- Modify: `electron/src/renderer/styles/globals.css`
- Modify: `electron/tailwind.config.js`
- Modify: `electron/src/renderer/store/index.ts`
- Modify: `electron/src/renderer/App.tsx`

**Steps:**

### Step 1.1: Rewrite globals.css with new token system
- Replace warm gradient backgrounds with flat `#fff` (light) / `#000` (dark)
- Replace `--primary` terracotta with neutral accent `#1d9bf0` (info blue) or keep `#0f1419` for light / `#e7e9ea` for dark
- Add status color tokens: `--success`, `--info`, `--warning`, `--danger`
- Change font from "Instrument Sans" to "Inter" (system stack with Inter fallback)
- Reduce border-radius values: 4px base, 8px buttons, 12px cards
- Remove glassmorphism overlays, heavy shadows, gradient backgrounds
- Update scrollbar colors to neutral grays
- Keep `.draggable` / `.no-drag` / `.mermaid-chart` / `* { transition }` / `::selection`
- Theme selectors: `[data-theme="light"]` and `[data-theme="dark"]` instead of `.dark`

### Step 1.2: Expand tailwind.config.js
- Keep existing color mappings to CSS vars
- Add new token mappings: `success`, `info`, `warning`, `danger`
- Change `darkMode` from `'class'` to `'selector'` (or remove since we use data-theme + CSS)
- Add `borderRadius` scale: `sm: 4px`, `DEFAULT: 8px`, `md: 8px`, `lg: 12px`, `xl: 16px`
- Add `fontFamily` for `sans` (Inter stack) and `mono` (JetBrains Mono stack)

### Step 1.3: Update theme application logic
- In `store/index.ts`: no change needed (theme values stay `'light' | 'dark' | 'system'`)
- In `App.tsx`: change `document.documentElement.classList.remove('light', 'dark')` to `document.documentElement.removeAttribute('data-theme')`, and `classList.add(theme)` to `setAttribute('data-theme', theme)`

### Step 1.4: Update App.tsx shell layout
- Remove `rounded-[30px]` glassmorphism desktop-panel styling
- Use flat background `bg-background`, remove `backdrop-blur-2xl`
- Keep 3-column layout intention but simplify: sidebar + main flex layout
- Control buttons: simpler borders, remove heavy shadows

## Task 2: Sidebar Refactor

**Files:**
- Modify: `electron/src/renderer/components/Sidebar.tsx`

**Steps:**
- Replace glassmorphism cards (`rounded-[30px]`, gradient bg, heavy shadows) with flat cards (`rounded-xl`, `bg-secondary`, `border-border`)
- Replace `status-dot` colored shadows with simple flat colors using new status tokens
- "New Task" button: remove gradient, use `bg-primary text-primary-foreground rounded-lg`
- Menu items: remove `rounded-2xl` → `rounded-lg`, simplify active state to `bg-secondary` + `border-l-2 border-primary`
- Session items: `rounded-[22px]` → `rounded-lg`, flatter hover states
- Stats cards: `rounded-2xl` → `rounded-lg`, remove gradients
- Settings footer: remove gradient bg, use `border-t border-border`
- Channel selector / custom select: flatter styling

## Task 3: Views Refactor (Settings + Others)

**Files:**
- Modify: `electron/src/renderer/views/SettingsView.tsx`
- Modify: `electron/src/renderer/views/ChatView.tsx`
- Modify: `electron/src/renderer/views/SessionsView.tsx`
- Modify: `electron/src/renderer/views/ScheduledTasksView.tsx`
- Modify: `electron/src/renderer/views/SkillsView.tsx`
- Modify: `electron/src/renderer/views/MCPView.tsx`

**Steps:**
- In SettingsView: replace `rounded-2xl`/`rounded-xl` cards with `rounded-lg` or `rounded-xl`
- Remove warm-toned hover/active states, use neutral `--hover` / `--active`
- Buttons: remove gradient bg, use solid `bg-secondary hover:bg-border`
- Primary buttons: `bg-primary text-primary-foreground`
- In ChatView: simplify input area, message bubbles use flat colors
- In other views: similar token swap

## Task 4: Component Refactor

**Files:**
- Modify: `electron/src/renderer/components/MarkdownRenderer.tsx`
- Modify: `electron/src/renderer/components/ExecutionHistory.tsx`
- Modify: `electron/src/renderer/components/CustomSelect.tsx`
- Modify: `electron/src/renderer/components/ConfirmDialog.tsx`
- Modify: `electron/src/renderer/components/ProviderEditor.tsx`
- Modify: `electron/src/renderer/components/CronBuilder.css`

**Steps:**
- CodeBlockCard: remove gradient backgrounds, use flat `bg-[#0f172a]` (dark) / `bg-[#f8fafc]` (light)
- ExecutionHistory modal: remove heavy shadows, flatter borders
- CustomSelect: flatter dropdown styling
- ConfirmDialog: flat styling
- ProviderEditor: flat card styling
- CronBuilder.css: update color variables to match new token names

## Task 5: Build & Verify

**Steps:**
1. `cd electron && npm run build`
2. Check for TypeScript / Tailwind class errors
3. Fix any broken class references
4. `make build` (Go backend)
5. Visual smoke test (if possible)
6. Commit all changes

---

## Verification Commands

```bash
cd /Users/lua/git/nanobot-go/electron && npm run build
# Check for compilation errors
```

```bash
cd /Users/lua/git/nanobot-go && make build
# Ensure Go backend still compiles
```
