ALTER TABLE delete_cart ADD COLUMN group_id TEXT NOT NULL DEFAULT '';
ALTER TABLE delete_cart ADD COLUMN group_label TEXT NOT NULL DEFAULT '';
UPDATE delete_cart SET group_id = object_hash WHERE group_id = '';
CREATE INDEX delete_cart_group_id ON delete_cart (group_id);
