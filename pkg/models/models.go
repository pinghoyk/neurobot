package models

// UserState представляет текущее состояние пользователя в разговоре
type UserState struct {
	UserID        int64
	CurrentState  string   // например: "main", "settings"
	LastMessageID int      // ID последнего сообщения - для редактирования
	InputData     string   // временные данные от пользователя (напр., введённые ингредиенты)
	StateHistory  []string // стек состояний - чтобы можно было сделать "назад"
}