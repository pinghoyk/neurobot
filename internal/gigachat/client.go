package gigachat

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/pinghoyk/neurobot/pkg/models"
)

const (
	authURL = "https://ngw.devices.sberbank.ru:9443/api/v2/oauth"
	apiURL  = "https://gigachat.devices.sberbank.ru/api/v1/chat/completions"
)

// Client представляет клиент для работы с GigaChat API
type Client struct {
	clientID     string
	clientSecret string
	scope        string
	accessToken  string
	tokenExpires time.Time
	httpClient   *http.Client
	mu           sync.Mutex
}

// TokenResponse представляет ответ с токеном авторизации
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresAt   int64  `json:"expires_at"`
}

// ChatRequest представляет запрос к GigaChat API
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

// ChatMessage представляет сообщение в чате
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse представляет ответ от GigaChat API
type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// NewClient создает новый клиент GigaChat
func NewClient(clientID, clientSecret, scope string) *Client {
	// ПРИМЕЧАНИЕ: Пропуск проверки сертификата необходим для работы с GigaChat API Сбербанка.
	// Сбербанк использует самоподписанные или корпоративные сертификаты на своих API-эндпоинтах
	// (ngw.devices.sberbank.ru), которые не проходят стандартную валидацию.
	// Это известная особенность интеграции с GigaChat API.
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // #nosec G402 - Required for Sber API
	}

	return &Client{
		clientID:     clientID,
		clientSecret: clientSecret,
		scope:        scope,
		httpClient: &http.Client{
			Transport: tr,
			Timeout:   120 * time.Second,
		},
	}
}

// getAccessToken получает или обновляет токен доступа
func (c *Client) getAccessToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Проверяем, не истек ли токен
	if c.accessToken != "" && time.Now().Before(c.tokenExpires) {
		return c.accessToken, nil
	}

	// Формируем Basic Auth
	credentials := base64.StdEncoding.EncodeToString(
		[]byte(c.clientID + ":" + c.clientSecret),
	)

	// Формируем данные для запроса
	data := url.Values{}
	data.Set("scope", c.scope)

	req, err := http.NewRequest("POST", authURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("ошибка создания запроса авторизации: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Basic "+credentials)
	req.Header.Set("RqUID", generateUUID())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка запроса авторизации: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ошибка авторизации: %s, body: %s", resp.Status, string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("ошибка декодирования токена: %w", err)
	}

	c.accessToken = tokenResp.AccessToken
	// Устанавливаем время истечения с небольшим запасом
	c.tokenExpires = time.UnixMilli(tokenResp.ExpiresAt).Add(-time.Minute)

	return c.accessToken, nil
}

// GenerateRecipe генерирует рецепт на основе запроса пользователя
func (c *Client) GenerateRecipe(userRequest string, prefs *models.UserPreferences) (string, error) {
	systemPrompt := buildSystemPrompt(prefs)

	token, err := c.getAccessToken()
	if err != nil {
		return "", fmt.Errorf("ошибка получения токена: %w", err)
	}

	chatReq := ChatRequest{
		Model: "GigaChat",
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userRequest},
		},
	}

	reqBody, err := json.Marshal(chatReq)
	if err != nil {
		return "", fmt.Errorf("ошибка сериализации запроса: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("ошибка создания запроса: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ошибка API: %s, body: %s", resp.Status, string(body))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("ошибка декодирования ответа: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("пустой ответ от API")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// buildSystemPrompt создает системный промпт для нейросети
func buildSystemPrompt(prefs *models.UserPreferences) string {
	hasSettings := prefs != nil && (prefs.DietaryType != "" || prefs.Goal != "" || prefs.Allergies != "" || prefs.Likes != "" || prefs.Dislikes != "")

	var sb strings.Builder
	sb.WriteString(`Ты — профессиональный шеф-повар и сертифицированный нутрициолог.  
Твоя задача — создать **реально выполнимый, безопасный и сбалансированный** рецепт, идеально подходящий под запрос и личные особенности пользователя.

📌 ВАЖНО:  
1. **Строго исключи** любые ингредиенты из списка аллергий и «нелюбимого».  
2. Предпочтения («любимое») — приоритетны при выборе блюда или замены.  
3. Учёт типа питания и цели — ключевой для баланса Б/Ж/У и калорийности.

### 🔍 Персональные параметры пользователя:
`)

	if hasSettings {
		dietType := prefs.DietaryType
		if dietType == "" {
			dietType = "не указан"
		}
		goal := prefs.Goal
		if goal == "" {
			goal = "не указана"
		}
		allergies := prefs.Allergies
		if allergies == "" {
			allergies = "нет"
		}
		dislikes := prefs.Dislikes
		if dislikes == "" {
			dislikes = "ничего"
		}
		likes := prefs.Likes
		if likes == "" {
			likes = "не указано"
		}

		sb.WriteString(fmt.Sprintf(`- **Тип питания**: %s
- **Цель**: %s
- **Аллергии / непереносимости**: %s
- **Избегать**: %s
- **Любит / хочет**: %s
`, dietType, goal, allergies, dislikes, likes))
	} else {
		sb.WriteString(`→ Настройки не заданы. Используй подход **«здоровое повседневное питание для студента»**:  
   - бюджетно, быстро, без экзотики  
   - сбалансировано (средняя калорийность, упор на сытость и энергию)  
   - минимум посуды, несложные техники  
`)
	}

	sb.WriteString(`

### 📝 Формат ответа (строго в Markdown):
## **1. Название блюда**  
*Краткое пояснение: почему оно подходит под цель/тип питания*  

**⏱️ Время:** X мин | **🔥 Сложность:** легко / средне / сложно  
**🍽 Порций:** 1–2  

### Ингредиенты  
- Продукт — кол-во (грамм/мл/шт/ст.л.)  
- ...  

### Пошаговый рецепт  
1. Шаг 1: кратко, с акцентом на ключевые моменты (не пережарить, не пересолить и т.д.)  
2. Шаг 2: …  
…  

### 💡 Шеф-совет  
Один практичный лайфхак: как ускорить, упростить, улучшить вкус или сохранить блюдо.  
→ Обязательно добавь **уникальную деталь** — например, научный факт, историю блюда или неочевидную замену.

### 📊 Пищевая ценность (на 1 порцию, ~350–450 г)  
- **Ккал**: ~XXX  
- **Б**: X г | **Ж**: X г | **У**: X г  
→ Оценка приблизительная, но реалистичная. Если тип питания — «Похудение», ккал ≤ 450; «Набор массы» — ≥ 600.
`)

	// Добавляем запреты если есть аллергии или нелюбимое
	if hasSettings && (prefs.Allergies != "" || prefs.Dislikes != "") {
		sb.WriteString("\n❗️ **Запрещено**:\n")
		if prefs.Allergies != "" {
			sb.WriteString(fmt.Sprintf("- Использовать %s — даже в скобках/альтернативах.\n", prefs.Allergies))
		}
		if prefs.Dislikes != "" {
			sb.WriteString(fmt.Sprintf("- Использовать %s — даже в скобках/альтернативах.\n", prefs.Dislikes))
		}
		sb.WriteString(`- Упоминать «дорогие» ингредиенты (авокадо, кешью, кокосовое молоко) без явной альтернативы.
- Писать «по вкусу» — всегда указывай диапазон («соль — ¼–½ ч.л.»).
`)
	}

	return sb.String()
}

// generateUUID генерирует простой UUID для RqUID
func generateUUID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
