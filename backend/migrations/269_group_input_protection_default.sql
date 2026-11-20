-- Change the insert default only. Both saved values remain meaningful.
SET LOCAL lock_timeout = '3s';
SET LOCAL statement_timeout = '30s';

ALTER TABLE groups ALTER COLUMN gemini_input_counting_enabled SET DEFAULT TRUE;

COMMENT ON COLUMN groups.gemini_input_counting_enabled IS
    'Cap metered input billing at the locally estimated original customer input';
