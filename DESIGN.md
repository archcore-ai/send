# DESIGN.md — Archcore Website

## Purpose

This file defines the visual, product, and interaction rules for the Archcore website.

AI agents must use this file as the source of truth when creating or modifying UI, copy, landing pages, components, sections, documentation previews, pricing pages, onboarding flows, or product marketing screens.

Archcore should feel like a serious developer infrastructure product: calm, precise, structured, and trustworthy.

Do not make the website look like a generic AI SaaS landing page.

---

## Product Positioning

### One-line positioning

Archcore turns a repository into structured, machine-readable context so AI coding agents understand architecture, rules, decisions, and workflows.

### Category

Git-native context layer for AI coding agents.

### Primary audience

- Founders building with AI coding agents
- Staff/principal engineers maintaining architectural consistency
- Engineering teams using Claude Code, Cursor, GitHub Copilot, Gemini CLI, Codex CLI, OpenCode, Roo Code, or Cline
- Teams that already have scattered docs, ADRs, CLAUDE.md, AGENTS.md, or internal conventions

### Core promise

The agent stops guessing and starts following the system.

### Emotional tone

Archcore should create the feeling of:

- “The agent finally understands my repo.”
- “Our architecture is no longer tribal knowledge.”
- “The context survives across agents, sessions, and teammates.”

### What Archcore is not

Do not frame Archcore as:

- a chatbot
- a code generator
- an IDE replacement
- another documentation tool
- “AI magic”
- a generic knowledge base
- a visual design tool

Archcore is infrastructure for repo-aware agents.

---

## Brand Personality

### Voice

Use a voice that is:

- direct
- technical
- confident
- minimal
- calm
- specific

### Avoid

Avoid copy that feels like:

- hype
- buzzword-heavy AI marketing
- vague productivity promises
- “10x developer” language
- enterprise jargon
- overly playful startup language

### Preferred words

Use words like:

- repository
- structured context
- machine-readable
- architecture
- rules
- decisions
- workflows
- MCP
- typed documents
- relation graph
- session hooks
- coding agents
- durable context
- source of truth
- repo-aware

### Avoid words

Avoid overusing:

- magic
- revolutionary
- seamless
- effortless
- unlock
- supercharge
- game-changing
- autonomous
- copilot for X
- AI-powered everything

---

## Visual Direction

Archcore uses a warm, minimal, almost paper-like visual system.

The interface should feel closer to a technical field notebook than a flashy AI landing page.

### Visual principles

1. Warm, quiet, structured.
2. Dense enough for developers, but never cluttered.
3. Use cards to explain systems.
4. Use subtle borders instead of heavy shadows.
5. Use black as the main action color.
6. Use beige surfaces and grid backgrounds to create structure.
7. Use small labels, code snippets, and comparison cards to make abstract concepts concrete.

---

## Design Tokens

Use these values unless there is a strong reason not to.

```yaml
colors:
  background:
    page: "#F8F1E8"
    page_alt: "#F3EDE1"
    surface: "#FBF7EE"
    surface_muted: "#F1EBDD"
    elevated: "#FFFDF7"

  text:
    primary: "#11100E"
    secondary: "#5F5A50"
    muted: "#8A8377"
    inverse: "#FFFFFF"

  border:
    subtle: "#E3D8C8"
    default: "#D5C8B6"
    strong: "#191714"

  action:
    primary: "#11100E"
    primary_hover: "#2A2722"
    secondary: "#F8F1E8"

  status:
    success: "#2E6B45"
    warning: "#8A6426"
    danger: "#8A2E2E"

typography:
  font_family:
    sans: "Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif"
    mono: "JetBrains Mono, SFMono-Regular, Consolas, 'Liberation Mono', monospace"

  desktop:
    hero:
      size: "56px"
      line_height: "0.96"
      weight: 700
      tracking: "-0.045em"

    h2:
      size: "34px"
      line_height: "1.05"
      weight: 700
      tracking: "-0.035em"

    h3:
      size: "20px"
      line_height: "1.2"
      weight: 650

    body:
      size: "15px"
      line_height: "1.55"
      weight: 400

    small:
      size: "12px"
      line_height: "1.4"
      weight: 400

  mobile:
    hero:
      size: "36px"
      line_height: "1.0"
      weight: 700
      tracking: "-0.04em"

    h2:
      size: "28px"
      line_height: "1.08"
      weight: 700

spacing:
  page_x_mobile: "20px"
  page_x_desktop: "32px"
  section_y_mobile: "72px"
  section_y_desktop: "120px"
  container_max: "1040px"
  narrow_max: "760px"

radius:
  xs: "6px"
  sm: "8px"
  md: "12px"
  lg: "16px"
  xl: "22px"
  pill: "999px"

border:
  thin: "1px solid #D5C8B6"
  strong: "1px solid #191714"

shadow:
  card: "0 1px 0 rgba(17, 16, 14, 0.04)"
  raised: "0 12px 32px rgba(17, 16, 14, 0.08)"

grid:
  background_size: "32px 32px"
  background_line: "rgba(17, 16, 14, 0.035)"
```

---

## CSS Variables

Use CSS variables so agents do not invent new colors.

```css
:root {
  --color-page: #f8f1e8;
  --color-page-alt: #f3ede1;
  --color-surface: #fbf7ee;
  --color-surface-muted: #f1ebdd;
  --color-elevated: #fffdf7;

  --color-text: #11100e;
  --color-text-secondary: #5f5a50;
  --color-text-muted: #8a8377;
  --color-text-inverse: #ffffff;

  --color-border-subtle: #e3d8c8;
  --color-border: #d5c8b6;
  --color-border-strong: #191714;

  --color-action: #11100e;
  --color-action-hover: #2a2722;

  --radius-card: 16px;
  --radius-control: 10px;
  --radius-pill: 999px;

  --container-max: 1040px;
  --container-narrow: 760px;
}
```

---

## Page Background

Use a warm cream background with a very subtle grid.

The grid should feel structural, not decorative.

```css
body {
  background-color: var(--color-page);
  background-image:
    linear-gradient(rgba(17, 16, 14, 0.035) 1px, transparent 1px),
    linear-gradient(90deg, rgba(17, 16, 14, 0.035) 1px, transparent 1px);
  background-size: 32px 32px;
  color: var(--color-text);
}
```

Do not use:

- blue AI gradients
- purple neon glows
- glassmorphism
- heavy drop shadows
- large stock illustrations
- generic AI orb visuals

---

## Layout System

### Container

Default desktop container:

```css
.container {
  max-width: 1040px;
  margin-inline: auto;
  padding-inline: 32px;
}
```

Narrow content container:

```css
.container-narrow {
  max-width: 760px;
  margin-inline: auto;
  padding-inline: 24px;
}
```

### Section spacing

Desktop sections should breathe.

```css
.section {
  padding-block: 112px;
}
```

Mobile:

```css
.section {
  padding-block: 72px;
}
```

### Alignment

Use centered alignment for hero and major section introductions.

Use left alignment inside cards, docs, code blocks, comparison examples, install steps, and FAQ rows.

---

## Navigation

The top nav should be minimal and quiet.

### Structure

Left:

- Archcore logo
- wordmark

Center:

- Plugin
- CLI
- Compare
- Docs
- EN dropdown

Right:

- GitHub icon
- primary CTA: Install Plugin

### Nav rules

- Height: 56–64px
- Background: transparent or page color
- Border: none by default
- Keep nav compact
- CTA is black pill button
- Do not add large nav shadows
- Do not use colorful active states

### Mobile nav

Use:

- logo left
- primary CTA or menu button right
- collapsed menu
- preserve the same calm visual tone

---

## Buttons

### Primary button

Use for the main action.

Examples:

- Use the Plugin
- Install Plugin
- Start with CLI

```css
.button-primary {
  background: var(--color-action);
  color: var(--color-text-inverse);
  border: 1px solid var(--color-action);
  border-radius: var(--radius-control);
  height: 38px;
  padding-inline: 18px;
  font-size: 13px;
  font-weight: 600;
}
```

### Secondary button

Use for lower-intent actions.

Examples:

- Start with CLI
- Read docs
- Compare paths

```css
.button-secondary {
  background: transparent;
  color: var(--color-text);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-control);
  height: 38px;
  padding-inline: 18px;
  font-size: 13px;
  font-weight: 600;
}
```

### Button copy rules

Use short verbs.

Good:

- Install Plugin
- Use the Plugin
- Start with CLI
- Read docs
- View GitHub

Bad:

- Unlock your AI workflow
- Start your transformation
- Supercharge development
- Get started now for free

---

## Cards

Cards are the main explanation primitive.

### Default card

```css
.card {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-card);
  box-shadow: var(--shadow-card);
}
```

### Emphasized card

Use for “With Archcore”, selected path, recommended mode, or primary examples.

```css
.card-emphasis {
  background: var(--color-elevated);
  border: 1px solid var(--color-border-strong);
  border-radius: var(--radius-card);
  box-shadow: 0 8px 24px rgba(17, 16, 14, 0.06);
}
```

### Card content

Cards should usually contain:

- small icon or label
- title
- short body text
- bullets or checks
- optional code snippet
- optional badge

Do not fill cards with long paragraphs.

---

## Badges

Badges are used for labels like:

- Recommended
- Best for most teams
- Plugin
- CLI
- Claude Code
- Cursor
- MCP
- Session hooks

### Badge style

```css
.badge {
  border-radius: 999px;
  padding: 3px 8px;
  font-size: 10px;
  font-weight: 600;
  letter-spacing: -0.01em;
  line-height: 1;
}
```

### Dark badge

Use sparingly for “Recommended” or selected states.

```css
.badge-dark {
  background: var(--color-text);
  color: var(--color-text-inverse);
}
```

### Light badge

```css
.badge-light {
  background: var(--color-surface-muted);
  color: var(--color-text-secondary);
  border: 1px solid var(--color-border);
}
```

---

## Code Blocks

Code blocks should look like terminal snippets printed on warm paper.

```css
.code-block {
  background: #f5eee1;
  border: 1px solid var(--color-border);
  border-radius: 12px;
  padding: 14px 16px;
  font-family: var(--font-mono);
  font-size: 13px;
  line-height: 1.5;
  color: var(--color-text);
}
```

### Rules

- Always include copy buttons for commands.
- Keep commands short.
- Do not overload landing pages with huge config examples.
- For long JSON/MCP config, show only the relevant excerpt and link to docs.
- Use monospace only for commands, filenames, config, and document types.

---

## Icons

Use simple monochrome line icons.

Preferred icon style:

- 16px or 18px
- 1.75px stroke
- rounded caps
- black or muted gray
- no filled colorful icons

Good icon themes:

- file
- folder
- graph
- check
- x
- terminal
- plug
- branch
- shield
- list
- code
- workflow

Do not use:

- robots as mascots
- sparkles as primary icons
- colorful 3D icons
- generic AI brain icons

---

## Homepage Structure

The homepage should follow this order.

### 1. Hero

Purpose: explain the product immediately.

Recommended headline:

> Turn your repository into structured, machine-readable context.

Recommended subheading:

> So AI agents can follow your architecture, rules, and decisions — instead of guessing.

CTA cards:

- Plugin
- CLI

Plugin card should be visually primary and marked recommended.

CLI card should feel equal in capability but less guided.

Hero should also include small proof/support text:

> Works with Claude Code, Cursor, GitHub Copilot, Gemini CLI, Codex CLI, and more.

### 2. The moment the agent understands your repo

Purpose: show the before/after.

Use a prompt example:

> Add a new user notifications service.

Then show two cards:

- Without Archcore
- With Archcore Plugin

Without Archcore:

- guesses folder structure
- ignores conventions
- misses ADRs
- re-creates old patterns

With Archcore:

- places files where architecture says they belong
- follows team rules
- respects ADRs and specs
- reuses known patterns

### 3. Install Archcore

Purpose: reduce friction.

Use tabs:

- Plugin
- CLI

Plugin tab should be default and recommended.

Show two steps:

1. Install the plugin in your agent host
2. Run the first intent command

Use short code blocks and copy buttons.

### 4. Why not just CLAUDE.md, AGENTS.md, or repo instructions?

Purpose: position against flat instruction files.

Message:

> Instruction files are flat memory. Archcore is structured system context.

Show 4 small cards:

- Archcore describes what happened, not only what to do
- Relations connect decisions, rules, specs, and guides
- Agents can query typed documents through MCP
- Context survives across tools and sessions

### 5. How to choose between Plugin and CLI

Purpose: help users self-select.

Two cards:

- Choose the Plugin
- Choose the CLI

Plugin is recommended for Claude Code and Cursor users.

CLI is recommended for direct integrations, scripting, CI, and unsupported hosts.

### 6. What gets better once your agent knows your system

Purpose: convert abstract system value into concrete use cases.

Use a 2x2 feature grid:

- Add a new service
- Follow team rules
- Reuse prior decisions
- Run multi-step workflows

### 7. FAQ

Purpose: handle objections.

Suggested FAQ items:

- Do I need both the plugin and the CLI?
- Which path should I start with?
- Which AI agents are supported?
- What does Archcore write into my repo?
- How do hooks and MCP work together?
- Do I need any external services?
- How is this different from CLAUDE.md or AGENTS.md?
- How do I keep the CLI updated?

### 8. Final CTA

Purpose: repeat the two paths.

Use compact cards:

- Plugin repository
- CLI repository
- Documentation
- GitHub organization

### 9. Footer

Footer should include:

- logo
- short description
- newsletter input
- links: Plugin, CLI, Docs, GitHub, Discord, X, Telegram, Privacy
- copyright

---

## Section Component Rules

### Hero component

Hero must be centered and compact.

Do:

- large headline
- short subheading
- two action cards
- proof text below

Do not:

- add screenshots above the headline
- add decorative AI illustrations
- add a long paragraph
- add more than two primary CTAs

### Choice card component

Use for Plugin vs CLI.

Each card must contain:

- path label
- small badge
- one-sentence description
- primary action
- optional supporting line

Plugin card should usually be emphasized.

### Before/after component

Use the “Without Archcore / With Archcore” pattern.

Without card:

- muted
- x icons
- lighter border

With card:

- stronger border
- check icons
- recommended badge

### Install steps component

Use numbered steps.

Each step should contain:

- number circle
- title
- short description
- code block or instruction
- tiny helper text

### FAQ component

FAQ rows should be simple accordions.

Use:

- 1px top border
- compact row height
- chevron on the right
- no heavy card container unless the section needs emphasis

---

## Responsive Rules

### Desktop

- Max content width: 1040px
- Hero width: 720–780px
- Cards: 2-column or 4-column grid
- Section vertical spacing: 96–128px

### Tablet

- Keep 2-column cards when readable
- Reduce hero headline to 44–48px
- Keep section spacing around 88px

### Mobile

- Single-column layout
- Hero headline: 34–38px
- Section spacing: 64–80px
- Cards full width
- Buttons full width only when inside CTA cards
- Navigation collapses

Mobile must not feel like a compressed desktop page.

---

## Copywriting Rules

### Headlines

Headlines should be direct and concrete.

Good:

- Turn your repository into structured, machine-readable context.
- The moment the agent starts understanding your repo.
- Why not just CLAUDE.md, AGENTS.md, or repository instructions?
- How to choose between Plugin and CLI.
- What gets better once your agent knows your system.

Bad:

- The future of AI-native development is here.
- Unlock the next generation of coding agents.
- Supercharge your repo with intelligence.
- Build faster with magical context.

### Subheadings

Subheadings should explain the mechanism.

Good:

> Typed documents — decisions, rules, guides, plans — that every AI coding agent can discover through MCP.

Bad:

> Our advanced AI platform helps teams collaborate better and ship faster.

### CTA labels

Preferred:

- Use the Plugin
- Start with CLI
- Install Plugin
- Read docs
- View GitHub
- Compare paths

Avoid:

- Learn more
- Get started
- Sign up
- Unlock now

Use “Get started” only if no stronger action exists.

---

## Product Vocabulary

Use these exact product terms consistently.

```yaml
product:
  name: "Archcore"
  directory: ".archcore/"
  core_terms:
    - "typed documents"
    - "relation graph"
    - "MCP"
    - "session hooks"
    - "Plugin"
    - "CLI"
    - "architecture"
    - "rules"
    - "decisions"
    - "guides"
    - "specs"
    - "workflows"

paths:
  plugin:
    label: "Plugin"
    best_for: "Claude Code and Cursor"
    description: "Higher-level experience with skills, tracks, built-in agents, and guardrails."

  cli:
    label: "CLI"
    best_for: "direct integrations, scripting, CI, and unsupported hosts"
    description: "Core context layer through the CLI, MCP server, and session hooks."
```

---

## Document Type References

When showing examples, use these document types correctly.

```yaml
knowledge:
  adr: "Architecture Decision Record — use for final technical decisions."
  rfc: "Request for Comments — use for proposed changes."
  rule: "Team standard or required behavior."
  guide: "Step-by-step instructions."
  spec: "Normative contract for a boundary."
  doc: "Reference documentation."

vision:
  prd: "Product requirements."
  idea: "Early concept."
  plan: "Implementation plan."

experience:
  task_type: "Recurring task pattern."
  cpat: "Code pattern change."
```

Do not invent new document types in product examples unless the product itself adds them.

---

## Example UI Copy

### Hero

```txt
Turn your repository into structured, machine-readable context.

So AI agents can follow your architecture, rules, and decisions — instead of guessing.
```

### Plugin card

```txt
Plugin
For Claude Code and Cursor. Higher-level workflows, guardrails, and intent commands inside your agent.
```

CTA:

```txt
Use the Plugin
```

### CLI card

```txt
CLI
For direct integrations, MCP servers, session hooks, scripting, and agent hosts beyond the plugin.
```

CTA:

```txt
Start with CLI
```

### Before/after section

```txt
The moment the agent starts understanding your repo.

Same prompt. Same codebase. The difference is whether the agent has structured project context.
```

Prompt:

```txt
Add a new user notifications service.
```

### CLAUDE.md comparison

```txt
Instruction files are flat memory. Archcore is structured system context.
```

Supporting text:

```txt
Instruction files work for agent-specific prompts and short-lived rules. They are not a graph of decisions, standards, specs, and workflows.
```

---

## Interaction Rules

### Hover states

Keep hover states subtle.

- Cards: slightly stronger border
- Buttons: slightly lighter black
- Links: underline or opacity change
- FAQ rows: muted background tint

Do not use:

- bouncing animations
- large scale transforms
- colorful hover glows
- aggressive motion

### Motion

Motion should be minimal and functional.

Allowed:

- accordion open/close
- small fade-in on page load
- copy button feedback
- tab switch transition

Avoid:

- parallax
- scroll hijacking
- animated gradients
- constantly moving decorative elements

### Copy buttons

Every command or config snippet should have a copy button.

Feedback text:

- Copied
- Copy failed

Keep it tiny and non-intrusive.

---

## Accessibility

Follow these rules for all generated UI.

- Use semantic HTML.
- Buttons must be real `<button>` elements.
- Links must be real `<a>` elements.
- Maintain visible focus states.
- Do not rely on color alone for state.
- Text contrast must remain readable on cream backgrounds.
- FAQ accordions must expose expanded/collapsed state.
- Code blocks must be keyboard-accessible.
- Tap targets should be at least 40px high on mobile.

Focus ring:

```css
:focus-visible {
  outline: 2px solid #11100e;
  outline-offset: 3px;
}
```

---

## Implementation Notes for React / Next.js

When generating components:

- Use reusable section components.
- Keep marketing copy separate from layout where reasonable.
- Prefer CSS variables or Tailwind theme tokens.
- Do not hardcode random colors.
- Use `max-w-[1040px]` for the main container.
- Use `max-w-[760px]` for centered text sections.
- Use rounded cards with warm borders.
- Keep icons monochrome.
- Preserve the page order defined in this file.

Suggested component names:

```txt
SiteHeader
HeroSection
PathChoiceCard
BeforeAfterAgentSection
InstallSection
FlatFilesComparisonSection
PluginVsCliSection
UseCasesGrid
FAQSection
FinalCTASection
SiteFooter
CodeBlock
CopyButton
Badge
```

---

## Do / Don't

### Do

- Make the product feel stable and precise.
- Explain abstract context through concrete repo examples.
- Use Plugin vs CLI as the primary decision model.
- Use before/after cards to show the agent behavior change.
- Use code snippets sparingly but concretely.
- Keep the visual system warm, quiet, and structured.
- Preserve the black primary CTA style.

### Don't

- Turn the site into a generic AI SaaS landing page.
- Add blue/purple gradients.
- Use sparkles as the main visual language.
- Overpromise agent autonomy.
- Add huge illustrations that push the product explanation below the fold.
- Replace technical specificity with vague marketing.
- Make Plugin and CLI feel like unrelated products.
- Hide the `.archcore/`, MCP, typed-documents, and relation-graph concepts.

---

## Agent Checklist Before Shipping UI Changes

Before finalizing any generated UI, verify:

1. The page still communicates the core promise within the first screen.
2. Plugin and CLI remain the two primary adoption paths.
3. The design uses warm cream backgrounds, black text, subtle borders, and structured cards.
4. No new random colors were introduced.
5. The copy is direct and technical.
6. Product terms are used consistently.
7. Code snippets include copy buttons.
8. Mobile layout is single-column and readable.
9. The page does not look like a generic AI startup template.
10. The final result feels like developer infrastructure, not AI decoration.
