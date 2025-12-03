-- Add availability type column
ALTER TABLE availability_rules ADD COLUMN IF NOT EXISTS type TEXT;

-- Add is_recurring column (default true for backward compatibility)
ALTER TABLE availability_rules ADD COLUMN IF NOT EXISTS is_recurring BOOLEAN DEFAULT TRUE;

-- Add new days_of_week array column
ALTER TABLE availability_rules ADD COLUMN IF NOT EXISTS days_of_week INTEGER[];

-- Migrate existing day_of_week to days_of_week array (only if day_of_week column exists)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns 
                WHERE table_name = 'availability_rules' AND column_name = 'day_of_week') THEN
        UPDATE availability_rules 
        SET days_of_week = ARRAY[day_of_week]::INTEGER[]
        WHERE days_of_week IS NULL;
    END IF;
END $$;

-- Make days_of_week NOT NULL after migration
ALTER TABLE availability_rules ALTER COLUMN days_of_week SET NOT NULL;

-- Create a function to validate days_of_week array
CREATE OR REPLACE FUNCTION validate_days_of_week(days_arr INTEGER[])
RETURNS BOOLEAN AS $$
BEGIN
    -- Check array length (1-7 days)
    IF array_length(days_arr, 1) IS NULL OR array_length(days_arr, 1) < 1 OR array_length(days_arr, 1) > 7 THEN
        RETURN FALSE;
    END IF;
    
    -- Check each day is between 0-6
    RETURN (
        SELECT bool_and(d >= 0 AND d <= 6)
        FROM unnest(days_arr) AS d
    );
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Add constraint using the validation function
ALTER TABLE availability_rules ADD CONSTRAINT check_days_of_week_range 
CHECK (validate_days_of_week(days_of_week));

-- Drop old day_of_week column (after ensuring data is migrated)
ALTER TABLE availability_rules DROP COLUMN IF EXISTS day_of_week;

