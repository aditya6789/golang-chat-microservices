CREATE TABLE IF NOT EXISTS friendships (
    user_1 UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_2 UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_1, user_2),
    CHECK (user_1 < user_2)
);

CREATE INDEX IF NOT EXISTS idx_friendships_u1 ON friendships(user_1);
CREATE INDEX IF NOT EXISTS idx_friendships_u2 ON friendships(user_2);
