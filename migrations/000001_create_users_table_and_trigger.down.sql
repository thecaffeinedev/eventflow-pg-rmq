-- migrations/000001_create_users_table_and_trigger.down.sql

DROP TRIGGER IF EXISTS users_changes_trigger ON users;
DROP FUNCTION IF EXISTS notify_users_changes();
DROP TABLE IF EXISTS users;
