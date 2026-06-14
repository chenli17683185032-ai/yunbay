# Event Horizon Public Frontend Redesign

Date: 2026-06-15
Project: New API
Scope: public customer-facing frontend only
Chosen direction: Event Horizon Gateway
Implementation depth: CSS + Motion lightweight edition

## 1. Outcome

Redesign the public-facing New API frontend into a dark, minimal, future-tech experience inspired by high-end creative technology sites. The interface should feel like an entry point at the edge of a vast model universe: quiet, precise, cinematic, and inevitable.

The redesign must preserve current New API public functions, routing, authentication behavior, API wiring, i18n behavior, custom content behavior, and project attribution. It must not rewrite the logged-in console product surface.

## 2. Design Read

Reading this as a customer-facing SaaS and infrastructure landing redesign for technical buyers and normal users, with an experimental dark-tech language, leaning toward native CSS, Tailwind utilities, and restrained Motion animation.

Dial values:

- DESIGN_VARIANCE: 8
- MOTION_INTENSITY: 6
- VISUAL_DENSITY: 3

Rationale:

- The user explicitly requested an Active Theory-like frontend, extreme minimal future technology, and a grand cosmic sense of fate.
- The product is still an API gateway, so the design must remain trustworthy and readable, not become a pure art experiment.
- The current app already uses React 19, Tailwind CSS v4, Rsbuild, and Motion, so a lightweight CSS + Motion path gives strong visual change with low implementation risk.

## 3. Scope

### In scope

Public customer-facing pages and shared public shell:

- `/`
- public header and public navigation
- public footer
- `/sign-in`
- `/sign-up`
- forgot password, reset password, OAuth callback, OTP, and related auth pages that use the public auth shell
- `/pricing` and model square public pricing views
- `/rankings`
- `/about`
- `/privacy-policy`
- `/user-agreement`
- setup or public error pages only where they share the same public shell and can be styled safely

### Out of scope

Logged-in product pages and business logic:

- `/dashboard`
- `/keys`
- `/wallet`
- `/usage-logs`
- `/profile`
- `/channels`
- `/models` inside the authenticated console
- `/system-settings`
- `/users`
- `/playground`
- any backend API, billing, relay, account, token, key, payment, or admin logic

The authenticated app may inherit global theme variables only if that does not materially alter layout or feature behavior. The implementation should avoid large structural changes to authenticated pages.

## 4. Non-negotiable constraints

1. Preserve protected project identity:
   - Do not remove or replace New API project attribution.
   - Do not remove or replace QuantumNous attribution.
   - Do not modify license identity, package metadata, or repository references for branding removal.
2. Preserve routes and URLs.
3. Preserve form field names, validation, OAuth behavior, OTP behavior, legal consent behavior, and redirect behavior.
4. Preserve i18n usage patterns with `useTranslation()` and `t()` in React components.
5. Preserve existing API hooks and request behavior.
6. Do not introduce heavyweight WebGL or Three.js for this pass.
7. Do not make logged-in control pages part of this visual overhaul.
8. Keep mobile fallback explicit for multi-column public layouts.
9. Respect `prefers-reduced-motion` for any continuous or entrance animation.
10. Maintain accessible contrast for text, forms, and CTAs.

## 5. Visual language

### 5.1 Mood

The final public frontend should feel like:

- a dark cosmic aperture,
- a silent model gateway,
- a precise infrastructure control surface,
- a future-facing brand experience with restraint,
- a monumental threshold rather than a colorful SaaS template.

It should not feel like:

- generic AI purple gradient marketing,
- ordinary Launch UI SaaS template,
- playful neon dashboard,
- overdecorated glassmorphism,
- a fake terminal screenshot page,
- a dense admin console.

### 5.2 Palette

Theme: dark-first for public pages.

Base tokens:

- near-black space background: `#050609` or token equivalent
- elevated surface: near-black with low-opacity white border
- primary text: off-white
- secondary text: desaturated translucent white
- accent: cold spectral white/blue, low saturation
- supporting glow: muted blue-violet only as atmospheric depth, never as generic CTA color

Accent rule:

- One accent family across the public pages: cold blue-white spectral light.
- CTAs should use high-contrast off-white on black or black on off-white.
- Purple should not become the main brand treatment.

### 5.3 Typography

Use the existing font infrastructure where possible, but style public pages with:

- very large, tight display headlines,
- mono microtext only for functional system labels or endpoint strings,
- short body copy with generous space,
- no excessive section eyebrows.

Chinese and English text must remain readable and not clip at display sizes.

### 5.4 Geometry

Use a consistent radius system:

- public navigation: pill or soft capsule
- CTAs: pill
- glass panels: large soft radius, approximately 24-32px
- forms: pill or soft rounded inputs, consistent with the auth card

Avoid mixing square cards with pill CTAs unless the rule is explicit.

### 5.5 Cosmic motifs

Allowed motifs:

- faint star fields,
- one large event-horizon glow,
- orbital rings,
- subtle scanline/grain layer,
- cold glass panels,
- sparse ASCII or endpoint code fragments,
- model constellation layout.

Not allowed as defaults:

- custom cursor,
- heavy neon glow everywhere,
- multiple marquees,
- fake screenshots made from div-only dashboards,
- section-number eyebrows,
- scroll cue labels,
- decorative city/time/weather strips.

## 6. Architecture plan

### 6.1 Public visual shell

Create a small public-only visual system under the existing frontend, likely in:

- `web/default/src/components/layout/components/public-cosmic-background.tsx`
- `web/default/src/components/layout/components/public-page-shell.tsx`
- `web/default/src/components/layout/components/public-header.tsx`
- `web/default/src/components/layout/components/footer.tsx`
- `web/default/src/styles/index.css` or a new imported public style file

The shell should provide:

- dark cosmic background,
- fixed atmospheric layers,
- reduced-motion fallback,
- reusable public content container,
- public surface panel classes,
- public CTA styles where safe.

The public shell must not force logged-in console pages into the same layout.

### 6.2 Homepage composition

Replace the current generic SaaS hero with Event Horizon Gateway composition:

- dark full-viewport hero using `min-h-[100dvh]`, not `h-screen`,
- left or asymmetric content placement,
- giant two-line max headline,
- endpoint capsule preserving the current base URL and `/v1/responses` display behavior,
- primary CTA preserving unauthenticated `Get Started` or authenticated `Go to Dashboard`,
- docs CTA preserving status-driven docs URL,
- model/provider constellation below or beside the hero,
- restrained public stats if the current stats remain useful.

Current custom home page behavior must remain:

- If system settings provide home content, keep iframe/Markdown custom home page logic.
- If no custom content exists, render the new Event Horizon default home.

### 6.3 Header and navigation

Public header should become:

- fixed top, transparent or low-opacity black,
- compact single-line desktop navigation,
- pill-shaped shell after scroll,
- logo and site name preserved from system config,
- existing dynamic top-nav links preserved,
- notifications, language switcher, theme switch, and profile/sign-in behavior preserved.

The existing public header already has scroll-state behavior. The redesign should adapt it rather than rewrite auth prompt logic.

### 6.4 Footer

Footer should become:

- minimal dark footer with subtle top border,
- New API and QuantumNous-related attribution preserved,
- configured custom footer HTML preserved,
- legal links preserved,
- fallback columns preserved or restyled.

### 6.5 Auth pages

Auth shell should become:

- dark cosmic background,
- centered or asymmetrically placed glass auth card,
- logo link preserved,
- form components unchanged internally where possible,
- legal terms footer preserved,
- OAuth provider layout preserved,
- mobile layout explicit and readable.

No changes to auth API calls, redirect search params, validation schemas, or token handling.

### 6.6 Pricing and model square

Pricing/model square should keep all current behavior:

- data loading from current hooks,
- search,
- filters,
- sidebar,
- toolbar,
- card/table view switch,
- model details drawer,
- empty and loading states.

Visual changes should focus on:

- dark cosmic public page background,
- top hero/title area,
- search bar surface treatment,
- sidebar and toolbar surfaces,
- card/table container styling through existing component class names where safe.

Avoid deep rewrites of pricing calculation and model metadata code.

### 6.7 Rankings, about, legal, and public content pages

Apply the same public shell:

- dark cosmic page background,
- readable content panels,
- restrained heading scale,
- same CTA and link treatment,
- Markdown/HTML rendering preserved.

For administrator-provided HTML/Markdown content, do not alter raw content semantics. Only wrap with a safe public container and prose styling.

## 7. Motion plan

Motion intensity: 6.

Allowed motion:

- entrance fade/blur/translate for hero elements,
- very slow orbital ring drift using CSS keyframes,
- subtle starfield drift using transform/opacity,
- CTA hover transform and border highlight,
- glass panel hover elevation where meaningful,
- model constellation gentle reveal.

Reduced motion behavior:

- disable continuous orbital/star animations,
- keep content visible immediately,
- preserve hover/focus states without movement-heavy effects.

Implementation rules:

- Animate only transform and opacity.
- Do not attach `window.addEventListener('scroll')` for new motion. The existing header scroll listener may remain unless replaced safely.
- Prefer CSS animations gated by media queries or Motion with `useReducedMotion()`.
- Do not use Three.js or GSAP in this pass.

## 8. Accessibility plan

- Use semantic HTML for header, nav, main, section, footer.
- Keep existing accessible button/link components.
- Preserve focus rings and keyboard navigation.
- Ensure input labels remain above or associated with inputs.
- Ensure button contrast meets WCAG AA.
- Ensure body text on dark surfaces has sufficient contrast.
- Keep decorative cosmic layers `aria-hidden`.
- Do not rely on color alone for important state.

## 9. Responsiveness plan

Public pages must have explicit mobile fallbacks:

- hero collapses to single column under `md`,
- headline scale reduces on mobile,
- CTAs wrap safely without awkward two-line labels,
- constellation/logos move below hero text,
- auth card width becomes full minus safe padding,
- pricing sidebar remains hidden or drawer-like as currently implemented,
- public header uses existing mobile menu behavior.

Use `min-h-[100dvh]` for hero sections and avoid viewport jump issues.

## 10. Testing and verification

Before implementation completion, verify:

1. Frontend typecheck: `bun run typecheck` from `web/default`.
2. Frontend build: `bun run build` from `web/default`.
3. Public pages manually in the browser:
   - `/`
   - `/sign-in`
   - `/sign-up`
   - `/pricing`
   - `/rankings`
   - `/about`
4. Auth flow smoke behavior:
   - sign-in form renders,
   - sign-up form renders when enabled,
   - protected nav click still opens sign-in prompt or redirect behavior as before.
5. Custom home page behavior still renders when `home_page_content` is configured.
6. Mobile viewport check for homepage and auth page.
7. Reduced motion check by confirming animations are disabled or minimized under reduced motion CSS.

## 11. Implementation approach

Preferred sequence:

1. Add public cosmic style primitives and shared background.
2. Restyle public header and footer without changing their logic.
3. Replace default home sections with Event Horizon components while preserving custom content branch.
4. Restyle auth layout only, leaving forms untouched where possible.
5. Restyle pricing/model square wrapper and major surfaces only.
6. Apply public shell to about/rankings/legal pages.
7. Run typecheck, build, and browser verification.
8. Rebuild Docker image only after frontend build passes, if the user wants the running container updated.

## 12. Risks and mitigations

### Risk: public theme leaks into authenticated console

Mitigation:

- Scope cosmic classes to public layout components.
- Avoid globally changing `:root` and `.dark` tokens in ways that unexpectedly overhaul logged-in pages.

### Risk: auth flow regression

Mitigation:

- Change auth outer layout only.
- Avoid editing form schemas, submit handlers, redirect logic, OAuth hooks, and API functions.

### Risk: pricing page behavior regression

Mitigation:

- Keep pricing hooks and business components intact.
- Prefer wrapper and class changes instead of data model changes.

### Risk: visual performance issues

Mitigation:

- Use CSS gradients and transforms, not WebGL.
- Gate continuous animation behind `prefers-reduced-motion`.
- Keep decorative layers few and fixed.

### Risk: i18n gaps

Mitigation:

- Reuse existing translation keys where possible.
- If new visible text is necessary, add it consistently to locale files or use existing stable keys.

## 13. Acceptance criteria

The design is accepted when:

- The default public homepage no longer looks like a generic light SaaS template.
- It clearly communicates a dark Event Horizon Gateway atmosphere.
- Public routes keep their existing functionality and navigation.
- Login and registration forms still work structurally and visually.
- Pricing/model square remains searchable and filterable.
- Project attribution remains intact.
- Typecheck and production build pass.
- Browser verification confirms desktop and mobile visual quality.

