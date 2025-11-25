package bot

import (
	"context"
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/pinghoyk/neurobot/internal/database"
	"github.com/pinghoyk/neurobot/internal/gigachat"
	"github.com/pinghoyk/neurobot/pkg/locales"
	"github.com/pinghoyk/neurobot/pkg/models"
)

// Bot представляет Telegram бота
type Bot struct {
	api      *tgbotapi.BotAPI
	db       *database.DB
	gigachat *gigachat.Client
}

// New создает нового бота
func New(token string, db *database.DB, gigachatClient *gigachat.Client) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания бота: %w", err)
	}

	log.Printf("Авторизован как @%s", api.Self.UserName)

	return &Bot{
		api:      api,
		db:       db,
		gigachat: gigachatClient,
	}, nil
}

// Start запускает обработку обновлений
func (b *Bot) Start(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return nil
		case update := <-updates:
			go b.handleUpdate(update)
		}
	}
}

// handleUpdate обрабатывает входящее обновление
func (b *Bot) handleUpdate(update tgbotapi.Update) {
	if update.CallbackQuery != nil {
		b.handleCallback(update.CallbackQuery)
		return
	}

	if update.Message != nil {
		b.handleMessage(update.Message)
	}
}

// handleMessage обрабатывает текстовые сообщения
func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	userID := msg.From.ID

	// Удаляем сообщение пользователя
	deleteMsg := tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID)
	b.api.Send(deleteMsg)

	// Получаем текущее состояние
	state, err := b.db.GetUserState(userID)
	if err != nil {
		log.Printf("Ошибка получения состояния: %v", err)
		return
	}

	// Обработка команд
	if msg.IsCommand() {
		switch msg.Command() {
		case "start":
			b.showMainMenu(msg.Chat.ID, userID, state.LastMessageID)
		case "settings":
			b.showSettings(msg.Chat.ID, userID, state.LastMessageID)
		case "help":
			b.showHelp(msg.Chat.ID, userID, state.LastMessageID)
		default:
			b.handleRecipeRequest(msg.Chat.ID, userID, msg.Text, state.LastMessageID)
		}
		return
	}

	// Обработка ввода в зависимости от состояния
	switch state.CurrentState {
	case models.StateSettingsGoal:
		b.handleGoalInput(msg.Chat.ID, userID, msg.Text, state.LastMessageID)
	case models.StateSettingsAllerg:
		b.handleAllergiesInput(msg.Chat.ID, userID, msg.Text, state.LastMessageID)
	case models.StateSettingsHabitsLikes:
		b.handleLikesInput(msg.Chat.ID, userID, msg.Text, state.LastMessageID)
	case models.StateSettingsHabitsDislikes:
		b.handleDislikesInput(msg.Chat.ID, userID, msg.Text, state.LastMessageID)
	default:
		// Генерация рецепта
		b.handleRecipeRequest(msg.Chat.ID, userID, msg.Text, state.LastMessageID)
	}
}

// handleCallback обрабатывает нажатия на inline-кнопки
func (b *Bot) handleCallback(callback *tgbotapi.CallbackQuery) {
	userID := callback.From.ID
	chatID := callback.Message.Chat.ID
	msgID := callback.Message.MessageID

	// Отвечаем на callback чтобы убрать "часики"
	b.api.Send(tgbotapi.NewCallback(callback.ID, ""))

	// Получаем состояние
	state, err := b.db.GetUserState(userID)
	if err != nil {
		log.Printf("Ошибка получения состояния: %v", err)
		return
	}

	// Сохраняем ID сообщения
	state.LastMessageID = msgID

	switch callback.Data {
	case "menu:main":
		b.showMainMenu(chatID, userID, msgID)
	case "menu:settings":
		b.showSettings(chatID, userID, msgID)
	case "menu:diet":
		b.showDietMenu(chatID, userID, msgID)
	case "menu:goal":
		b.showGoalInput(chatID, userID, msgID)
	case "menu:allergies":
		b.showAllergiesInput(chatID, userID, msgID)
	case "menu:habits":
		b.showHabitsMenu(chatID, userID, msgID)
	case "menu:likes":
		b.showLikesInput(chatID, userID, msgID)
	case "menu:dislikes":
		b.showDislikesInput(chatID, userID, msgID)
	case "menu:clear":
		b.showClearConfirm(chatID, userID, msgID)
	case "menu:help":
		b.showHelp(chatID, userID, msgID)

	// Выбор типа питания
	case "diet:none":
		b.saveDietType(chatID, userID, msgID, "Обычное")
	case "diet:lose":
		b.saveDietType(chatID, userID, msgID, "Похудение")
	case "diet:gain":
		b.saveDietType(chatID, userID, msgID, "Набор массы")

	// Подтверждение сброса
	case "clear:yes":
		b.clearAllSettings(chatID, userID, msgID)
	case "clear:no":
		b.showSettings(chatID, userID, msgID)
	}
}

// showMainMenu отображает главное меню
func (b *Bot) showMainMenu(chatID, userID int64, editMsgID int) {
	l := locales.Get()
	text := l.MainMenu.Text

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(l.MainMenu.Buttons.Settings, "menu:settings"),
		),
	)

	b.sendOrEditMessage(chatID, userID, editMsgID, text, keyboard, models.StateMain)
}

// showSettings отображает меню настроек
func (b *Bot) showSettings(chatID, userID int64, editMsgID int) {
	l := locales.Get()

	prefs, _ := b.db.GetUserPreferences(userID)
	settingsText := b.formatSettingsText(prefs)
	text := fmt.Sprintf(l.SettingsMenu.Text, settingsText)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(l.SettingsMenu.Buttons.Diet, "menu:diet"),
			tgbotapi.NewInlineKeyboardButtonData(l.SettingsMenu.Buttons.Goal, "menu:goal"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(l.SettingsMenu.Buttons.Allergies, "menu:allergies"),
			tgbotapi.NewInlineKeyboardButtonData(l.SettingsMenu.Buttons.Habits, "menu:habits"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(l.SettingsMenu.Buttons.Clear, "menu:clear"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(l.SettingsMenu.Buttons.Back, "menu:main"),
		),
	)

	b.sendOrEditMessage(chatID, userID, editMsgID, text, keyboard, models.StateSettings)
}

// showDietMenu отображает меню выбора типа питания
func (b *Bot) showDietMenu(chatID, userID int64, editMsgID int) {
	l := locales.Get()

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(l.DietMenu.Options.None, "diet:none"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(l.DietMenu.Options.Lose, "diet:lose"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(l.DietMenu.Options.Gain, "diet:gain"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(l.DietMenu.Buttons.BackToSettings, "menu:settings"),
		),
	)

	b.sendOrEditMessage(chatID, userID, editMsgID, l.DietMenu.Text, keyboard, models.StateSettingsDiet)
}

// showGoalInput запрашивает ввод цели
func (b *Bot) showGoalInput(chatID, userID int64, editMsgID int) {
	l := locales.Get()

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(l.GoalMenu.Buttons.BackToSettings, "menu:settings"),
		),
	)

	b.sendOrEditMessage(chatID, userID, editMsgID, l.GoalMenu.Text, keyboard, models.StateSettingsGoal)
}

// showAllergiesInput запрашивает ввод аллергий
func (b *Bot) showAllergiesInput(chatID, userID int64, editMsgID int) {
	l := locales.Get()

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(l.AllergiesMenu.Buttons.BackToSettings, "menu:settings"),
		),
	)

	b.sendOrEditMessage(chatID, userID, editMsgID, l.AllergiesMenu.Text, keyboard, models.StateSettingsAllerg)
}

// showHabitsMenu отображает меню привычек
func (b *Bot) showHabitsMenu(chatID, userID int64, editMsgID int) {
	l := locales.Get()

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(l.HabitsMenu.Buttons.Dislikes, "menu:dislikes"),
			tgbotapi.NewInlineKeyboardButtonData(l.HabitsMenu.Buttons.Likes, "menu:likes"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(l.HabitsMenu.Buttons.BackToSettings, "menu:settings"),
		),
	)

	b.sendOrEditMessage(chatID, userID, editMsgID, l.HabitsMenu.Text, keyboard, models.StateSettingsHabits)
}

// showLikesInput запрашивает ввод любимых продуктов
func (b *Bot) showLikesInput(chatID, userID int64, editMsgID int) {
	l := locales.Get()

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(l.LikesMenu.Buttons.BackToHabits, "menu:habits"),
		),
	)

	b.sendOrEditMessage(chatID, userID, editMsgID, l.LikesMenu.Text, keyboard, models.StateSettingsHabitsLikes)
}

// showDislikesInput запрашивает ввод нелюбимых продуктов
func (b *Bot) showDislikesInput(chatID, userID int64, editMsgID int) {
	l := locales.Get()

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(l.DislikesMenu.Buttons.BackToHabits, "menu:habits"),
		),
	)

	b.sendOrEditMessage(chatID, userID, editMsgID, l.DislikesMenu.Text, keyboard, models.StateSettingsHabitsDislikes)
}

// showClearConfirm показывает подтверждение сброса
func (b *Bot) showClearConfirm(chatID, userID int64, editMsgID int) {
	l := locales.Get()

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(l.ClearConfirm.Buttons.Yes, "clear:yes"),
			tgbotapi.NewInlineKeyboardButtonData(l.ClearConfirm.Buttons.No, "clear:no"),
		),
	)

	b.sendOrEditMessage(chatID, userID, editMsgID, l.ClearConfirm.Text, keyboard, models.StateSettingsClearConfirm)
}

// showHelp отображает справку
func (b *Bot) showHelp(chatID, userID int64, editMsgID int) {
	text := `❓ *Помощь*

Я — бот для генерации персонализированных рецептов.

Настройте тип питания, цели, аллергии и предпочтения — и я учту всё при подборе блюд.

Чтобы начать — откройте *Настройки*.`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "menu:main"),
		),
	)

	b.sendOrEditMessage(chatID, userID, editMsgID, text, keyboard, models.StateHelp)
}

// saveDietType сохраняет тип питания
func (b *Bot) saveDietType(chatID, userID int64, editMsgID int, dietType string) {
	l := locales.Get()

	prefs, _ := b.db.GetUserPreferences(userID)
	prefs.UserID = userID
	prefs.DietaryType = dietType

	if err := b.db.SaveUserPreferences(prefs); err != nil {
		log.Printf("Ошибка сохранения предпочтений: %v", err)
	}

	text := fmt.Sprintf(l.DietMenu.Success, dietType)
	keyboard := b.getSuccessKeyboard()

	b.sendOrEditMessage(chatID, userID, editMsgID, text, keyboard, models.StateSettings)
}

// handleGoalInput обрабатывает ввод цели
func (b *Bot) handleGoalInput(chatID, userID int64, text string, editMsgID int) {
	l := locales.Get()

	prefs, _ := b.db.GetUserPreferences(userID)
	prefs.UserID = userID

	if strings.ToLower(strings.TrimSpace(text)) == "нет" {
		prefs.Goal = ""
	} else {
		prefs.Goal = text
	}

	if err := b.db.SaveUserPreferences(prefs); err != nil {
		log.Printf("Ошибка сохранения предпочтений: %v", err)
	}

	keyboard := b.getSuccessKeyboard()
	b.sendOrEditMessage(chatID, userID, editMsgID, l.GoalMenu.Success, keyboard, models.StateSettings)
}

// handleAllergiesInput обрабатывает ввод аллергий
func (b *Bot) handleAllergiesInput(chatID, userID int64, text string, editMsgID int) {
	l := locales.Get()

	prefs, _ := b.db.GetUserPreferences(userID)
	prefs.UserID = userID

	if strings.ToLower(strings.TrimSpace(text)) == "нет" {
		prefs.Allergies = ""
	} else {
		prefs.Allergies = text
	}

	if err := b.db.SaveUserPreferences(prefs); err != nil {
		log.Printf("Ошибка сохранения предпочтений: %v", err)
	}

	keyboard := b.getSuccessKeyboard()
	b.sendOrEditMessage(chatID, userID, editMsgID, l.AllergiesMenu.Success, keyboard, models.StateSettings)
}

// handleLikesInput обрабатывает ввод любимых продуктов
func (b *Bot) handleLikesInput(chatID, userID int64, text string, editMsgID int) {
	l := locales.Get()

	prefs, _ := b.db.GetUserPreferences(userID)
	prefs.UserID = userID

	if strings.ToLower(strings.TrimSpace(text)) == "нет" {
		prefs.Likes = ""
	} else {
		prefs.Likes = text
	}

	if err := b.db.SaveUserPreferences(prefs); err != nil {
		log.Printf("Ошибка сохранения предпочтений: %v", err)
	}

	keyboard := b.getSuccessKeyboardWithHabits()
	b.sendOrEditMessage(chatID, userID, editMsgID, l.LikesMenu.Success, keyboard, models.StateSettingsHabits)
}

// handleDislikesInput обрабатывает ввод нелюбимых продуктов
func (b *Bot) handleDislikesInput(chatID, userID int64, text string, editMsgID int) {
	l := locales.Get()

	prefs, _ := b.db.GetUserPreferences(userID)
	prefs.UserID = userID

	if strings.ToLower(strings.TrimSpace(text)) == "нет" {
		prefs.Dislikes = ""
	} else {
		prefs.Dislikes = text
	}

	if err := b.db.SaveUserPreferences(prefs); err != nil {
		log.Printf("Ошибка сохранения предпочтений: %v", err)
	}

	keyboard := b.getSuccessKeyboardWithHabits()
	b.sendOrEditMessage(chatID, userID, editMsgID, l.DislikesMenu.Success, keyboard, models.StateSettingsHabits)
}

// clearAllSettings сбрасывает все настройки
func (b *Bot) clearAllSettings(chatID, userID int64, editMsgID int) {
	l := locales.Get()

	if err := b.db.ClearUserPreferences(userID); err != nil {
		log.Printf("Ошибка сброса настроек: %v", err)
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(l.ClearSuccess.Buttons.ToSettings, "menu:settings"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(l.ClearSuccess.Buttons.ToMain, "menu:main"),
		),
	)

	b.sendOrEditMessage(chatID, userID, editMsgID, l.ClearSuccess.Text, keyboard, models.StateSettings)
}

// handleRecipeRequest обрабатывает запрос на генерацию рецепта
func (b *Bot) handleRecipeRequest(chatID, userID int64, request string, editMsgID int) {
	// Проверяем rate limit
	allowed, err := b.db.CheckRateLimit(userID)
	if err != nil {
		log.Printf("Ошибка проверки лимита: %v", err)
	}

	if !allowed {
		text := "⏳ *Подождите немного*\n\nСлишком много запросов. Попробуйте через минуту."
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "Markdown"
		b.api.Send(msg)
		return
	}

	// Показываем сообщение о генерации
	waitMsg := tgbotapi.NewMessage(chatID, "🍳 *Готовлю рецепт...*\n\nЭто займёт несколько секунд.")
	waitMsg.ParseMode = "Markdown"
	sentMsg, err := b.api.Send(waitMsg)
	if err != nil {
		log.Printf("Ошибка отправки сообщения: %v", err)
		return
	}

	// Получаем предпочтения пользователя
	prefs, _ := b.db.GetUserPreferences(userID)

	// Генерируем рецепт
	recipe, err := b.gigachat.GenerateRecipe(request, prefs)
	if err != nil {
		log.Printf("Ошибка генерации: %v", err)
		errorText := "❌ *Ошибка генерации*\n\nПопробуйте ещё раз или переформулируйте запрос."
		editMsg := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, errorText)
		editMsg.ParseMode = "Markdown"
		b.api.Send(editMsg)
		return
	}

	// Редактируем сообщение с результатом
	editMsg := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, recipe)
	editMsg.ParseMode = "Markdown"
	b.api.Send(editMsg)

	// Обновляем состояние
	state := &models.UserState{
		UserID:        userID,
		CurrentState:  models.StateMain,
		LastMessageID: sentMsg.MessageID,
	}
	b.db.SaveUserState(state)
}

// sendOrEditMessage отправляет новое или редактирует существующее сообщение
func (b *Bot) sendOrEditMessage(chatID, userID int64, editMsgID int, text string, keyboard tgbotapi.InlineKeyboardMarkup, newState string) {
	var msgID int

	if editMsgID > 0 {
		// Пытаемся отредактировать существующее сообщение
		editMsg := tgbotapi.NewEditMessageText(chatID, editMsgID, text)
		editMsg.ParseMode = "Markdown"
		editMsg.ReplyMarkup = &keyboard

		_, err := b.api.Send(editMsg)
		if err == nil {
			msgID = editMsgID
		} else {
			// Если редактирование не удалось, отправляем новое
			log.Printf("Не удалось отредактировать сообщение: %v", err)
			newMsg := tgbotapi.NewMessage(chatID, text)
			newMsg.ParseMode = "Markdown"
			newMsg.ReplyMarkup = keyboard
			sentMsg, err := b.api.Send(newMsg)
			if err == nil {
				msgID = sentMsg.MessageID
			}
		}
	} else {
		// Отправляем новое сообщение
		newMsg := tgbotapi.NewMessage(chatID, text)
		newMsg.ParseMode = "Markdown"
		newMsg.ReplyMarkup = keyboard
		sentMsg, err := b.api.Send(newMsg)
		if err == nil {
			msgID = sentMsg.MessageID
		}
	}

	// Сохраняем состояние
	state := &models.UserState{
		UserID:        userID,
		CurrentState:  newState,
		LastMessageID: msgID,
	}
	b.db.SaveUserState(state)
}

// formatSettingsText форматирует текст с текущими настройками
func (b *Bot) formatSettingsText(prefs *models.UserPreferences) string {
	l := locales.Get()
	var parts []string

	diet := prefs.DietaryType
	if diet == "" {
		diet = "_не указано_"
	}
	parts = append(parts, fmt.Sprintf("• %s: %s", l.SettingsMenu.Fields.Diet, diet))

	goal := prefs.Goal
	if goal == "" {
		goal = "_не указано_"
	}
	parts = append(parts, fmt.Sprintf("• %s: %s", l.SettingsMenu.Fields.Goal, goal))

	allergies := prefs.Allergies
	if allergies == "" {
		allergies = "_не указано_"
	}
	parts = append(parts, fmt.Sprintf("• %s: %s", l.SettingsMenu.Fields.Allergies, allergies))

	// Привычки - объединяем likes и dislikes
	var habitsInfo []string
	if prefs.Likes != "" {
		habitsInfo = append(habitsInfo, fmt.Sprintf("❤️ %s", prefs.Likes))
	}
	if prefs.Dislikes != "" {
		habitsInfo = append(habitsInfo, fmt.Sprintf("👎 %s", prefs.Dislikes))
	}

	habits := "_не указано_"
	if len(habitsInfo) > 0 {
		habits = strings.Join(habitsInfo, " | ")
	}
	parts = append(parts, fmt.Sprintf("• %s: %s", l.SettingsMenu.Fields.Habits, habits))

	return strings.Join(parts, "\n")
}

// getSuccessKeyboard возвращает клавиатуру успешного сохранения
func (b *Bot) getSuccessKeyboard() tgbotapi.InlineKeyboardMarkup {
	l := locales.Get()
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(l.DietMenu.Buttons.BackToSettings, "menu:settings"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(l.DietMenu.Buttons.BackToMain, "menu:main"),
		),
	)
}

// getSuccessKeyboardWithHabits возвращает клавиатуру с возвратом в привычки
func (b *Bot) getSuccessKeyboardWithHabits() tgbotapi.InlineKeyboardMarkup {
	l := locales.Get()
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(l.DislikesMenu.Buttons.BackToHabits, "menu:habits"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(l.DislikesMenu.Buttons.BackToSettings, "menu:settings"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(l.DislikesMenu.Buttons.BackToMain, "menu:main"),
		),
	)
}
