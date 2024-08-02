-- migrations/000001_create_users_table_and_trigger.up.sql

CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE OR REPLACE FUNCTION notify_users_changes()
RETURNS trigger AS $$
DECLARE
    payload JSON;
BEGIN
    payload = json_build_object(
        'action', TG_OP,
        'table_name', TG_TABLE_NAME,
        'data', row_to_json(NEW)
    );
    
    PERFORM pg_notify('users_changes', payload::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER users_changes_trigger
AFTER INSERT OR UPDATE OR DELETE ON users
FOR EACH ROW EXECUTE FUNCTION notify_users_changes();
