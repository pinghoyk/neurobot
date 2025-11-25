package database

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/pinghoyk/neurobot/pkg/models"
)

// SaveUserState сохраняет состояние пользователя
func (db *DB) SaveUserState(state *models.UserState) error {
	historyJSON, err := json.Marshal(state.StateHistory)
	if err != nil {
		historyJSON = []byte("[]")
	}

	_, err = db.conn.Exec(`
		INSERT INTO user_states (user_id, current_state, last_message_id, input_data, state_history, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			current_state = excluded.current_state,
			last_message_id = excluded.last_message_id,
			input_data = excluded.input_data,
			state_history = excluded.state_history,
			updated_at = excluded.updated_at
	`, state.UserID, state.CurrentState, state.LastMessageID, state.InputData, string(historyJSON), time.Now())

	return err
}

// GetUserState получает состояние пользователя
func (db *DB) GetUserState(userID int64) (*models.UserState, error) {
	state := &models.UserState{UserID: userID}
	var historyJSON string

	err := db.conn.QueryRow(`
		SELECT current_state, last_message_id, input_data, state_history
		FROM user_states WHERE user_id = ?
	`, userID).Scan(&state.CurrentState, &state.LastMessageID, &state.InputData, &historyJSON)

	if err == sql.ErrNoRows {
		// Возвращаем начальное состояние для нового пользователя
		return &models.UserState{
			UserID:       userID,
			CurrentState: models.StateMain,
			StateHistory: []string{},
		}, nil
	}

	if err != nil {
		return nil, err
	}

	// Парсим историю состояний
	if historyJSON != "" {
		if err := json.Unmarshal([]byte(historyJSON), &state.StateHistory); err != nil {
			state.StateHistory = []string{}
		}
	}

	return state, nil
}

// CheckRateLimit проверяет, не превышен ли лимит запросов
func (db *DB) CheckRateLimit(userID int64) (bool, error) {
	var lastRequest sql.NullTime
	var requestCount int

	err := db.conn.QueryRow(`
		SELECT last_request_at, request_count FROM rate_limits WHERE user_id = ?
	`, userID).Scan(&lastRequest, &requestCount)

	if err == sql.ErrNoRows {
		// Первый запрос пользователя
		return true, db.updateRateLimit(userID)
	}

	if err != nil {
		return false, err
	}

	// Сбрасываем счетчик если прошла минута
	if lastRequest.Valid && time.Since(lastRequest.Time) > time.Minute {
		return true, db.updateRateLimit(userID)
	}

	// Проверяем лимит (например, 5 запросов в минуту)
	if requestCount >= 5 {
		return false, nil
	}

	// Увеличиваем счетчик
	_, err = db.conn.Exec(`
		UPDATE rate_limits SET request_count = request_count + 1 WHERE user_id = ?
	`, userID)

	return true, err
}

// updateRateLimit обновляет или создает запись о лимитах
func (db *DB) updateRateLimit(userID int64) error {
	_, err := db.conn.Exec(`
		INSERT INTO rate_limits (user_id, last_request_at, request_count)
		VALUES (?, ?, 1)
		ON CONFLICT(user_id) DO UPDATE SET
			last_request_at = excluded.last_request_at,
			request_count = 1
	`, userID, time.Now())
	return err
}

// SaveUserPreferences сохраняет предпочтения пользователя
func (db *DB) SaveUserPreferences(prefs *models.UserPreferences) error {
	_, err := db.conn.Exec(`
		INSERT INTO user_preferences (user_id, dietary_type, goal, allergies, likes, dislikes, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			dietary_type = excluded.dietary_type,
			goal = excluded.goal,
			allergies = excluded.allergies,
			likes = excluded.likes,
			dislikes = excluded.dislikes,
			updated_at = excluded.updated_at
	`, prefs.UserID, prefs.DietaryType, prefs.Goal, prefs.Allergies, prefs.Likes, prefs.Dislikes, time.Now())

	return err
}

// GetUserPreferences получает предпочтения пользователя
func (db *DB) GetUserPreferences(userID int64) (*models.UserPreferences, error) {
	prefs := &models.UserPreferences{UserID: userID}

	err := db.conn.QueryRow(`
		SELECT dietary_type, goal, allergies, likes, dislikes
		FROM user_preferences WHERE user_id = ?
	`, userID).Scan(&prefs.DietaryType, &prefs.Goal, &prefs.Allergies, &prefs.Likes, &prefs.Dislikes)

	if err == sql.ErrNoRows {
		// Возвращаем пустые предпочтения для нового пользователя
		return prefs, nil
	}

	return prefs, err
}

// ClearUserPreferences очищает все предпочтения пользователя
func (db *DB) ClearUserPreferences(userID int64) error {
	_, err := db.conn.Exec(`
		UPDATE user_preferences SET
			dietary_type = '',
			goal = '',
			allergies = '',
			likes = '',
			dislikes = '',
			updated_at = ?
		WHERE user_id = ?
	`, time.Now(), userID)

	if err != nil {
		return err
	}

	// Если записи не было, создаем пустую
	_, err = db.conn.Exec(`
		INSERT OR IGNORE INTO user_preferences (user_id, dietary_type, goal, allergies, likes, dislikes, updated_at)
		VALUES (?, '', '', '', '', '', ?)
	`, userID, time.Now())

	return err
}
