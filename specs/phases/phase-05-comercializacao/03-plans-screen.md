# Phase 5.3 — Plans Screen

## Goal

Clear, conversion-optimized screen showing plan options, pricing, and a path to purchase.

## Pricing

| Plan | Monthly | Annual |
|---|---|---|
| Essencial | R$ 19 | R$ 152 (33% off) |
| Pro | R$ 39 | R$ 312 (33% off) |
| Expert | R$ 69 | R$ 552 (33% off) |

## Tasks

- [ ] Comparison table: Essencial / Pro / Expert with feature checklist per column
- [ ] Monthly / Annual toggle; annual price shown with discount badge
- [ ] CTA button per plan: "Choose Essencial", "Choose Pro", "Choose Expert"
- [ ] Current plan highlighted if user already has a subscription
- [ ] Screen shown automatically when trial expires (blocking modal)
- [ ] Accessible from settings at any time

## Acceptance Criteria

- Toggling to Annual updates all prices without page reload
- The user's current plan column is visually highlighted
- Clicking a CTA opens the correct Paddle checkout for that plan + billing period
- Blocking modal cannot be dismissed; other screens are inaccessible until a plan is chosen or the window is closed
