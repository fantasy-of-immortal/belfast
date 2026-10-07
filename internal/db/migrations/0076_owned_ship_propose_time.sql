-- Add propose_time so the client can render a real marriage date.
--
-- owned_ships.propose is a boolean, but the client's SHIPINFO.propose field carries the
-- marriage timestamp itself:
--
--   model/vo/ship.lua
--     slot0.propose     = slot1.propose and slot1.propose > 0
--     slot0.proposeTime = slot1.propose
--
-- proposeTime feeds the marriage-date UI (view/ship/proposeui.lua:240) and the "days since
-- marriage" summary (model/vo/summary.lua:30). Sending the previous boolToUint32 value (1)
-- made every ship report 1970-01-01 as its marriage date.
ALTER TABLE owned_ships
  ADD COLUMN IF NOT EXISTS propose_time timestamptz NOT NULL DEFAULT '1970-01-01 00:00:00+00';

-- Backfill: any ship already proposed keeps a usable timestamp.
UPDATE owned_ships
SET propose_time = COALESCE(create_time, NOW())
WHERE propose = TRUE
  AND propose_time = '1970-01-01 00:00:00+00';
