# Chapter 8 — Credentials

> **In plain terms.** Chapter 1 called this the trust wall: to do real work, a
> worker needs keys — and the naive way to give keys (hand it your own
> password) makes every task carry your whole identity. This chapter explains
> the mechanism the organization uses instead: every worker gets its own key,
> made for its one task, scoped to the one place it needs to write, and
> destroyed the moment the task ends — with the destruction *proven*.

## 8.1 The principle: identity per task, not per person

The design rule (Chapter 2, P5) is short: **least privilege, provably brief.**
Expanded into mechanics, a credential in this system must be:

1. **Issued per task** — minted at spawn time, for this task only;
2. **Scoped** — permission to touch exactly the resources the task touches,
   and nothing else;
3. **Short-lived** — the credential self-expires within at most an hour,
   regardless of what else happens;
4. **Revoked at completion** — the reaper's cleanup includes killing the
   credential, not just the container;
5. **Proven dead** — after revocation, the system *demonstrates* the
   credential no longer works. A revocation that isn't verified is a hope.

## 8.2 The mechanism: an organization-owned app identity

The build implemented this with the credential issuer the platform ecosystem
already provides for machine identity: an **organization-owned application**
registered with the source-control provider, holding a private key that
lives on the machine that issues credentials — and nowhere else.

The flow for every task:

```
work order accepted
   → the lifecycle signs a short-lived assertion with the app's private key
   → exchanges it for an installation token
   → injects ONLY that token into the worker's environment
   → worker clones, branches, edits, pushes — with this token alone
   → task ends
   → the token is revoked
   → a verification call with the dead token returns DENIED (proven)
```

Properties worth pausing on:

- **The worker never sees a long-lived secret.** Not the app's private key,
  not a personal token — only the task token, which cannot outlive the task
  by more than minutes.
- **The private key lives on the issuer machine only.** Workers never touch
  it; they receive its *product*, not the key itself. In the target
  architecture the issuer machine is also the vault boundary (Chapter 13).
- **Blast radius is one task, one repository.** A compromised worker yields
  an attacker a token that can write to one repo and dies within the hour.
  Compare: a personal credential yields everything, forever.

## 8.3 The token's life, measured

The proof-of-concept measured the properties rather than asserting them:

- **Cap:** the assertion lifetime was pinned below the provider's maximum
  (a provider-side cap on token validity forced the design below an hour —
  a constraint that *improved* the design; Chapter 12 tells the story of
  discovering it).
- **Revocation lag:** from task end to verified-dead: **under 75 seconds**,
  in repeated runs.
- **Post-revocation proof:** a call using the dead token returns an explicit
  denial — recorded as test evidence, not as a log line.

These three numbers are the chapter's payload. "The token is dead" is a
claim until it is a test result.

## 8.4 What each component may hold

The credential architecture doubles as a map of trust in the estate:

| Component | Holds | Never holds |
|---|---|---|
| Credential issuer (execution host) | App private key (0600), issuer config | Personal tokens |
| Worker containers | One task-scoped token, in env | Any key, any long-lived secret |
| Front-of-house | A read-only catalog token | Any write credential |
| Human operator | Personal account (outside the system) | Nothing injected into workers |

The rule that fell out of the build, and is now a design invariant: **no
personal identity is ever injected into a worker.** If a component is about
to hand a worker a human's token, the architecture is wrong, not the
component.

## 8.5 Design decisions and alternatives

**Decision: the app-identity mechanism over a shared long-lived token.**
The build evaluated both honestly. The shared token was *faster* — zero
setup, work immediately. It was rejected on four grounds: it placed a
person's full-scope identity into the automation path (violating the
architecture's own credential rule), made the revocation test meaningless
(you cannot revoke a credential the whole system depends on), weakened the
secret-hygiene story, and would need replacing in the next version anyway.
The build's own gates — revocation *proof* and secret *scans* — only pass
under the app mechanism. A decision that makes your own tests pass is the
right decision; one that requires weakening the tests is not.

**Decision: assertion lifetime 9 minutes, not the maximum 10.** The provider
caps assertion validity at 10 minutes. Designing to the cap's edge invites
clock-skew failures at the worst moment; the margin costs nothing.

**Decision: revocation is verified, revocation lag is measured.** Both came
from the same instinct: the security property must appear in the evidence
pack as a *result* ("HTTP 401, T+75 s"), never as a configuration line
("revocation: enabled").

**Decision: secrets live in gitignored, permission-restricted files; scans
enforce the boundary.** Every built image and every tracked repository file
is scanned for secret patterns as a hard gate — zero findings to pass. The
scan is what makes "the key is only on the issuer machine" a maintained
invariant instead of a launch-day memory.

## 8.6 The trust story in one paragraph

A skeptical auditor asks: "Your AI worker edited a repository. What could it
touch?" The complete answer, with this architecture: *one repository, one
branch it created itself, for under an hour, with access verified dead
immediately after — and a scan that proves no stronger credential ever
entered its environment.* No policy document says this; the mechanism says
it. That is the difference between trusting a system and verifying one,
and it is the property the organization's credential layer exists to
deliver.
