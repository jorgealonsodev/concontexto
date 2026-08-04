# Delta for source-ingestion-eurostat

Slice **2** (prerequisite). Delta against `openspec/specs/source-ingestion-eurostat/spec.md`.

JSON-stat keys `status` by the same linear index the decoder already computes as `pos`, so reading it is
an addition to the existing decode loop. Eurostat's flags are not a P/D binary — `p e b d f u c n :` —
and there is **no "definitive" flag**: absence means definitive.

## ADDED Requirements

### Requirement: JSON-stat status is read at the computed position

The decoder MUST read the JSON-stat `status` map at the same linear `pos` it already computes for each
value, and MUST carry the verbatim flag through to the observation's `source_status`.

#### Scenario: A flagged observation carries its verbatim flag

- GIVEN a recorded JSON-stat response whose `status` map carries `"p"` at the position of one observation
- WHEN it is decoded
- THEN that observation's `source_status` is `"p"`
- AND its domain status is provisional
- AND no other observation is affected

#### Scenario: Status is aligned with the value it belongs to

- GIVEN a response with flags at several non-contiguous positions
- WHEN it is decoded
- THEN each flag lands on the observation at the same linear position as the value it annotates

### Requirement: Absence of a flag means definitive

Eurostat publishes no definitive flag. An observation with no entry in the `status` map MUST be recorded
as definitive, with a null `source_status`. The absence MUST NOT be treated as unknown, missing or a
schema drift.

#### Scenario: An unflagged observation is definitive

- GIVEN a recorded response whose `status` map has no entry for an observation
- WHEN it is decoded
- THEN that observation's domain status is definitive
- AND its `source_status` is null

#### Scenario: An empty status map is valid

- GIVEN a recorded response carrying no `status` map at all
- WHEN it is decoded
- THEN every observation is definitive and ingestion proceeds

### Requirement: Break and definition flags are metadata, not statuses

The flags `b` (break in time series) and `d` (definition differs) MUST be routed to break and definition
metadata beside `series_break`, and MUST NOT be mapped into the observation status enum. An unrecognised
flag MUST fail the run as `sourceerr.SchemaDrift`.

#### Scenario: A break flag becomes break metadata

- GIVEN a recorded response with `"b"` at an observation's position
- WHEN it is decoded
- THEN a break annotation is recorded for that series and period beside `series_break`
- AND the observation's domain status is not set from the `b` flag

#### Scenario: A definition flag becomes definition metadata

- GIVEN a recorded response with `"d"` at an observation's position
- WHEN it is decoded
- THEN a definition-differs annotation is recorded for that series and period
- AND the observation's domain status is not set from the `d` flag

#### Scenario: An unrecognised flag fails closed

- GIVEN a recorded response carrying a flag outside the documented set
- WHEN it is decoded
- THEN the run fails with `sourceerr.SchemaDrift` naming the dataset and the unrecognised flag
- AND nothing is written
