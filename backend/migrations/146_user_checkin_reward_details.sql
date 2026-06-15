ALTER TABLE user_checkins
    ADD COLUMN IF NOT EXISTS streak_day INTEGER NOT NULL DEFAULT 1;

ALTER TABLE user_checkins
    ADD COLUMN IF NOT EXISTS base_reward_amount DECIMAL(20,8) NOT NULL DEFAULT 0;

ALTER TABLE user_checkins
    ADD COLUMN IF NOT EXISTS bonus_reward_amount DECIMAL(20,8) NOT NULL DEFAULT 0;

ALTER TABLE user_checkins
    ADD COLUMN IF NOT EXISTS total_reward_amount DECIMAL(20,8) NOT NULL DEFAULT 0;

UPDATE user_checkins
SET base_reward_amount = reward_amount,
    total_reward_amount = reward_amount
WHERE base_reward_amount = 0
  AND total_reward_amount = 0
  AND reward_amount > 0;
