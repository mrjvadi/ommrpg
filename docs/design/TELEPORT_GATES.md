# TELEPORT_GATES.md

## Gate Types
- normal
- ancient
- unstable
- void
- world
- event

## Gate Properties
- origin
- destination
- owner
- stability
- capacity
- energy consumption
- maintenance
- tax
- access policy
- level
- history

## City Gates
Cities may own gates and define:
- open
- paid
- invitation
- restricted
- closed

## Strategic Importance
Gates create:
- trade routes
- taxes
- political power
- military objectives
- exploration

## Sacred Land Return-Bound Rule
Mandatory.

If a player enters Sacred Land from City 2:
- record City 2 as the origin
- allow Sacred Land services
- allow return only to City 2
- do not allow City 3

City 2 -> Sacred Land -> City 2 = valid.
City 2 -> Sacred Land -> City 3 = invalid.

The client must never choose the final Sacred Land return destination.
The server stores the origin session.

## Gate Security
Validate:
- origin
- destination
- access
- player progression
- gate state
- session binding
