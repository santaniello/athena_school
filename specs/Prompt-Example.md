# Prompt Example — AI-Driven Development

> The CLAUDE.md already defines the TDD cycle, code standards, and rules.
> Your prompt just needs to say **what** to implement — not how.

---

## Opening a spec

```
Read and implement the spec `phases/phase-01-foundation/02-config-system.md`.
Follow CLAUDE.md. One acceptance criterion at a time — wait for my approval before moving to the next.
```

That's it. The AI reads the spec, writes the test, implements, refactors, and stops for your approval.

---

## Approving and continuing

```
Approved. Next criterion.
```

---

## Closing the spec

```
All criteria done. Run the quality gate and propose the commit message.
```

---

## The only rule for you

**Run `go test ./...` after each criterion and confirm before approving the next.**
The AI follows the cycle — you control the pace.
