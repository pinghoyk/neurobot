CREATE TABLE IF NOT EXISTS user_states (
    user_id INTEGER PRIMARY KEY,
    current_state TEXT NOT NULL,
    last_message_id INTEGER,
    input_data TEXT,
    state_history TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS rate_limits (
    user_id INTEGER PRIMARY KEY,
    last_request_at DATETIME,
    request_count INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS user_preferences (
    user_id INTEGER PRIMARY KEY,
    dietary_type TEXT DEFAULT '',
    goal TEXT DEFAULT '',
    allergies TEXT DEFAULT '',
    likes TEXT DEFAULT '',
    dislikes TEXT DEFAULT '',
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);