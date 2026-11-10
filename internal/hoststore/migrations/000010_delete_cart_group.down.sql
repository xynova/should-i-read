DROP INDEX IF EXISTS delete_cart_group_id;
ALTER TABLE delete_cart DROP COLUMN group_label;
ALTER TABLE delete_cart DROP COLUMN group_id;
