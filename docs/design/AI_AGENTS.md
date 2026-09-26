# AI_AGENTS.md

## AI Philosophy
AI characters are real persistent world residents.

They use the same fundamental Character model as players:
- attributes
- energies
- knowledge
- understanding
- mastery
- class
- profession
- equipment
- reputation
- hidden Root/Talent
- goals
- personality
- memory

## AI Roles
- master
- merchant
- alchemist
- blacksmith
- mayor
- diplomat
- general
- scholar
- explorer
- farmer
- assassin
- researcher

## AI Progression
AI can:
- learn
- practice
- improve mastery
- evolve profession
- evolve class
- research
- discover
- create
- trade
- teach
- move cities
- establish schools
- sign contracts

## Masters
Masters have:
- specialization
- knowledge
- mastery
- teaching skill
- student capacity
- personality
- goals
- contract status

A master with capacity 30/30 cannot accept unlimited students.

## AI Memory
Use MongoDB for appropriate memory documents:
- short term
- long term
- professional
- knowledge
- personal relationships
- world history

## Goals
Examples:
- reach Alchemy 95%
- discover a new elixir
- find a rare material
- train promising students
- earn gold
- protect a city
- research a formation

## AI Decision Pipeline
LLM/behavior engine
-> proposed action
-> agent-service
-> domain command
-> validation in owning service
-> state mutation
-> event

AI must never directly modify authoritative DB state.

## LLM Boundary
LLM may generate:
- dialogue
- explanation
- narrative
- flavor text
- proposed actions

LLM may not directly decide:
- money
- ownership
- combat result
- mastery
- item creation
- access permission
- death
- class unlock

## AI and Hidden Talent
AI characters also have hidden Roots/Talents.
Players and other agents do not see exact values.

## AI Economy
AI can:
- earn
- spend
- invest
- trade
- pay salaries
- sign contracts
- own businesses

## AI Migration
A master may leave a city and accept a better offer elsewhere.
This makes masters real strategic city assets.

## AI Death
If death exists, it must follow deterministic game rules, never an arbitrary LLM response.
