-- Add model market recommendation display controls to groups.
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS recommendation_label VARCHAR(50) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS recommendation_stars INTEGER NOT NULL DEFAULT 3;

UPDATE groups
SET recommendation_stars = 3
WHERE recommendation_stars IS NULL OR recommendation_stars < 3;

UPDATE groups
SET recommendation_stars = 5
WHERE recommendation_stars > 5;
