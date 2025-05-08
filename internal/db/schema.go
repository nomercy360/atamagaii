package db

// UpdateSchema adds the flashcard tables to the database
func (s *Storage) UpdateSchema() error {
	// Flashcard schema
	schema := `
	CREATE TABLE IF NOT EXISTS users (
	   id TEXT PRIMARY KEY,
	   telegram_id INTEGER UNIQUE,
	   deleted_at TIMESTAMP,
	   created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	   updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	   username TEXT,
	   avatar_url TEXT,
	   name TEXT,
	   level TEXT DEFAULT 'N5',
	   points REAL DEFAULT 0
    );
	-- Decks table
	CREATE TABLE IF NOT EXISTS decks (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		new_cards_per_day INTEGER DEFAULT 7,
		level TEXT,
		user_id TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		deleted_at TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);
	-- Cards table
	CREATE TABLE IF NOT EXISTS cards (
		id TEXT PRIMARY KEY,
		deck_id TEXT NOT NULL,
		front TEXT NOT NULL,
		back TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		deleted_at TIMESTAMP,
		FOREIGN KEY (deck_id) REFERENCES decks(id)
	);
	-- Card progress table (user-specific progress for each card)
	CREATE TABLE IF NOT EXISTS card_progress (
		user_id TEXT NOT NULL,
		card_id TEXT NOT NULL,
		next_review TIMESTAMP,
		interval INTEGER NOT NULL DEFAULT 0,
		ease REAL NOT NULL DEFAULT 2.5,
		review_count INTEGER NOT NULL DEFAULT 0,
		laps_count INTEGER NOT NULL DEFAULT 0,
		last_reviewed_at TIMESTAMP,
		PRIMARY KEY (user_id, card_id),
		FOREIGN KEY (user_id) REFERENCES users(id),
		FOREIGN KEY (card_id) REFERENCES cards(id)
	);
	-- Reviews history table
	CREATE TABLE IF NOT EXISTS reviews (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		card_id TEXT NOT NULL,
		rating INTEGER NOT NULL,
		reviewed_at TIMESTAMP NOT NULL,
		time_spent_ms INTEGER NOT NULL,
		prev_interval INTEGER NOT NULL,
		new_interval INTEGER NOT NULL,
		prev_ease REAL NOT NULL,
		new_ease REAL NOT NULL,
		FOREIGN KEY (user_id) REFERENCES users(id),
		FOREIGN KEY (card_id) REFERENCES cards(id)
	);

	CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		type TEXT,
		content TEXT,
		answer TEXT,
		difficulty TEXT
	);
	
	CREATE TABLE IF NOT EXISTS user_tasks (
		id TEXT PRIMARY KEY,
		user_id INTEGER,
		task_id INTEGER,
		completion_at TIMESTAMP,
		time_spent_ms INTEGER,
		is_correct BOOLEAN,
		FOREIGN KEY (user_id) REFERENCES users(id),
		FOREIGN KEY (task_id) REFERENCES tasks(id)
	);

	-- Create index on next_review to speed up due card queries
	CREATE INDEX IF NOT EXISTS idx_card_progress_next_review ON card_progress(next_review);
	
	-- Create index on deck_id to speed up deck queries
	CREATE INDEX IF NOT EXISTS idx_cards_deck_id ON cards(deck_id);
	
	-- Create index on user_id for deck queries
	CREATE INDEX IF NOT EXISTS idx_decks_user_id ON decks(user_id);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return err
	}

	return nil
}
