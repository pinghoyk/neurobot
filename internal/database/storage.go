package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pinghoyk/neurobot/pkg/models"
)

// Все методы Save/Get/etc. остаются как есть, но без migrate/initTables
// (они теперь в sqlite.go → applySchema)

func (db *DB) SaveUserState(state *models.UserState) error { /* ... */ }
func (db *DB) GetUserState(userID int64) (*models.UserState, error) { /* ... */ }
func (db *DB) CheckRateLimit(userID int64) (bool, error) { /* ... */ }
func (db *DB) updateRateLimit(userID int64) error { /* ... */ }
func (db *DB) SaveUserPreferences(prefs *models.UserPreferences) error { /* ... */ }
func (db *DB) GetUserPreferences(userID int64) (*models.UserPreferences, error) { /* ... */ }