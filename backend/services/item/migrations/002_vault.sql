-- NFT support: an item is either in a character's bag ('inventory') or
-- tokenised and held in the owner's vault ('vault', managed by asset-service).
ALTER TABLE items ADD COLUMN state TEXT NOT NULL DEFAULT 'inventory';
ALTER TABLE items ADD COLUMN vault_op TEXT;
ALTER TABLE items ADD COLUMN claim_op TEXT;
CREATE INDEX items_state ON items (state);
