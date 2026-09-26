# EVENTS_AND_HISTORY.md

## Domain Events
Examples:
- CharacterCreated
- KnowledgeLearned
- MasteryChanged
- ClassAwakened
- DiscoveryCreated
- ItemCreated
- UnitCreated
- CityFounded
- BuildingCompleted
- MasterHired
- TradeCompleted
- TreatySigned
- WarDeclared
- GateDiscovered
- PlayerAscended
- WorldEventStarted

## Event Envelope
Fields:
- event id
- type
- version
- actor
- timestamp
- origin service
- entity id
- correlation id
- payload

## History
Record important:
- first discoveries
- first crafts
- first cities
- first gates
- major wars
- treaties
- world bosses
- unique creations
- class/path discoveries

## Ownership vs History
Current owner and first discoverer are separate.
Ownership can change without rewriting historical credit.

## Offline Reports
Players may receive summarized reports when they return after being offline.

## Event Sourcing
Do not use full event sourcing everywhere.
Use authoritative relational state plus durable event/history records.
