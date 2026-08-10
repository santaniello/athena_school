# Phase 5.2 — Paddle Integration

## Goal

Users can purchase a plan via Paddle; payment events update the account plan automatically.

## Tasks

- [ ] Paddle account configured with 6 products:
  - Essencial Monthly, Essencial Annual
  - Pro Monthly, Pro Annual
  - Expert Monthly, Expert Annual
- [ ] Auth server: `POST /webhooks/paddle` → validates signature → updates `Account.Plan`
- [ ] UI: "Upgrade" button opens Paddle checkout (external browser or Paddle overlay)
- [ ] After successful payment: plan badge in app updates on next plan check

## Paddle Webhook Events Handled

- `subscription.activated` → set plan
- `subscription.cancelled` → revert to free/trial expired
- `subscription.updated` → update plan tier

## Acceptance Criteria

- Clicking "Upgrade to Pro" opens the Paddle checkout
- After a test purchase, the account plan in the database is updated
- Cancellation reverts the plan to the appropriate downgraded state
- Invalid webhook signatures are rejected with 401
