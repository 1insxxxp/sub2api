CREATE TABLE IF NOT EXISTS checkin_reward_campaigns (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(120) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    reward_tiers JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_by BIGINT NULL,
    updated_by BIGINT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT checkin_reward_campaigns_status_check
        CHECK (status IN ('draft', 'enabled', 'disabled')),
    CONSTRAINT checkin_reward_campaigns_date_order_check
        CHECK (start_date <= end_date)
);

CREATE INDEX IF NOT EXISTS checkin_reward_campaigns_status_dates_idx
    ON checkin_reward_campaigns (status, start_date, end_date);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'checkin_reward_campaigns_enabled_dates_excl'
          AND conrelid = 'checkin_reward_campaigns'::regclass
    ) THEN
        ALTER TABLE checkin_reward_campaigns
            ADD CONSTRAINT checkin_reward_campaigns_enabled_dates_excl
            EXCLUDE USING gist (
                daterange(start_date, end_date, '[]') WITH &&
            ) WHERE (status = 'enabled');
    END IF;
END $$;

ALTER TABLE user_checkins
    ADD COLUMN IF NOT EXISTS reward_campaign_id BIGINT NULL,
    ADD COLUMN IF NOT EXISTS reward_campaign_name VARCHAR(120) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS reward_campaign_tiers_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb;

CREATE INDEX IF NOT EXISTS user_checkins_reward_campaign_id_idx
    ON user_checkins (reward_campaign_id);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'user_checkins_reward_campaign_id_fkey'
          AND conrelid = 'user_checkins'::regclass
    ) THEN
        ALTER TABLE user_checkins
            ADD CONSTRAINT user_checkins_reward_campaign_id_fkey
            FOREIGN KEY (reward_campaign_id)
            REFERENCES checkin_reward_campaigns(id)
            ON DELETE RESTRICT;
    END IF;
END $$;
