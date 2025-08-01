-- Remove genesis change tracking fields
-- Note: SQLite doesn't support DROP COLUMN, so we need to recreate the table
-- This is a simplified down migration - in practice, you'd need to recreate the table

-- For now, we'll just document what needs to be done
-- In a real implementation, you would:
-- 1. Create a new table with the old schema
-- 2. Copy data from the current table to the new table
-- 3. Drop the current table
-- 4. Rename the new table to the original name

-- This is left as a placeholder since SQLite doesn't support DROP COLUMN 