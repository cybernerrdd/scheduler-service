-- Add event column to availability_rules table
ALTER TABLE availability_rules ADD COLUMN IF NOT EXISTS event TEXT;

-- Create index on event for faster filtering
CREATE INDEX IF NOT EXISTS idx_availability_rules_event ON availability_rules(event);

-- Create composite index for user_id and event for efficient queries
CREATE INDEX IF NOT EXISTS idx_availability_rules_user_event ON availability_rules(user_id, event);

