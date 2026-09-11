DROP TRIGGER IF EXISTS trg_spec_items_set_updated_at ON spec_items;
DROP TRIGGER IF EXISTS trg_spec_tables_set_updated_at ON spec_tables;
DROP TRIGGER IF EXISTS trg_users_set_updated_at ON users;
DROP FUNCTION IF EXISTS dspecs_set_updated_at();

DROP TABLE IF EXISTS share_links;
DROP TABLE IF EXISTS spec_revisions;
DROP TABLE IF EXISTS spec_items;
DROP TABLE IF EXISTS spec_tables;
DROP TABLE IF EXISTS users;
