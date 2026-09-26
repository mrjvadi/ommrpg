# SECURITY.md

## Trust Boundary
The game client and all user input are untrusted. The client is only a view: every rule is enforced by the server.

## Validate
- identity
- kingdom membership
- role permissions
- city access
- Kingdom permissions
- item ownership
- currency
- resource quantities
- energy
- cooldowns
- world rules
- teleport destination
- master capacity

## Client Requests
Never trust client payloads (HTTP bodies, realtime RPC data).
Reload and validate actual state.

## Idempotency
Required for:
- purchase
- sale
- claim
- craft
- attack
- trade
- build
- learn
- teleport

## Economic Attacks
Protect against:
- double spend
- duplication
- negative balance
- replay
- race conditions
- refund abuse
- contract duplication
- reward duplication

## Discovery Attacks
Protect:
- first discoverer
- unique discovery
- seed manipulation
- fake prerequisites
- duplicate result

## AI Security
AI output is never trusted as game state.

## Secrets
Never log secrets, tokens, keys or private credentials.

## Audit
Audit:
- admin changes
- economy changes
- ownership changes
- unique discoveries
- rare asset creation

## Rate Limits
Use Redis/Dragonfly for:
- realtime actions (move, attack, interact)
- exploration spam
- crafting spam
- market spam
- AI action scheduling
