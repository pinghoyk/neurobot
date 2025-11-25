package models

// UserState представляет текущее состояние пользователя в разговоре
type UserState struct {
	UserID        int64
	CurrentState  string   // например: "main", "settings"
	LastMessageID int      // ID последнего сообщения - для редактирования
	InputData     string   // временные данные от пользователя (напр., введённые ингредиенты)
	StateHistory  []string // стек состояний - чтобы можно было сделать "назад"
}

// UserPreferences представляет кулинарные предпочтения пользователя
type UserPreferences struct {
	UserID      int64
	DietaryType string // обычное, похудение, набор веса
	Goal        string // цель: (напр., похудеть на 3 кг)
	Allergies   string // данные, на что аллергия
	Likes       string // данные, что нравится в еде
	Dislikes    string // данные, что не нравится в еде
}

// Константы состояний для конечного автомата (FSM)
const (
	StateMain                   = "main"
	StateHelp                   = "help"
	StateSettings               = "settings"
	StateSettingsDiet           = "settings_diet"
	StateSettingsAllerg         = "settings_allergies"
	StateSettingsGoal           = "settings_goal"
	StateSettingsHabits         = "settings_habits"
	StateSettingsHabitsLikes    = "settings_habits_likes"
	StateSettingsHabitsDislikes = "settings_habits_dislikes"
	StateSettingsClearConfirm   = "settings_clear_confirm"
	StateGenerating             = "generating"
)
