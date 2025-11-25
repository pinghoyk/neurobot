// Package gigachat предоставляет клиент для GigaChat API через OAuth 2.0 (client_credentials).
package gigachat

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
	"strings"

	"github.com/pinghoyk/neurobot/pkg/models"
)

// ⚠️ ИСПРАВЛЕНО: убраны пробелы в конце URL
const (
	oauthURL = "https://ngw.devices.sberbank.ru:9443/api/v2/oauth"
	apiURL   = "https://gigachat.devices.sberbank.ru/api/v1/chat/completions"
)

// Client — клиент для GigaChat с OAuth-авторизацией.
type Client struct {
	clientID     string
	clientSecret string
	scope        string
	accessToken  string
	tokenExpires time.Time
	httpClient   *http.Client
	mu           sync.Mutex
}

// TokenResponse — ответ /oauth.
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"` // секунды
}

// ChatRequest — запрос к чату.
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

// ChatMessage — сообщение.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse — ответ модели.
type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// NewClient создаёт клиент с OAuth-данными.
func NewClient(clientID, clientSecret, scope string) *Client {
	if scope == "" {
		scope = "GIGACHAT_API_PERS" // ✅ обязательный scope
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // ✅ ОСТАВЬТЕ ЭТО для Sber API
			MinVersion:         tls.VersionTLS12,
		},
}
	return &Client{
		clientID:     clientID,
		clientSecret: clientSecret,
		scope:        scope,
		httpClient: &http.Client{
			Transport: tr,
			Timeout:   30 * time.Second,
		},
	}
}

// getAccessToken — получает или обновляет access_token.
func (c *Client) getAccessToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.tokenExpires.IsZero() && time.Now().Before(c.tokenExpires) && c.accessToken != "" {
		return c.accessToken, nil
	}

	auth := base64.StdEncoding.EncodeToString([]byte(c.clientID + ":" + c.clientSecret))

	data := url.Values{}
	data.Set("scope", c.scope)

	req, err := http.NewRequest("POST", oauthURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("ошибка создания /oauth запроса: %w", err)
	}

	rqUID := generateUUID()
	log.Printf("🔑 Запрос токена: RqUID=%s", rqUID)

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("RqUID", rqUID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка HTTP /oauth: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("/oauth error %d: %s", resp.StatusCode, string(body))
	}

	var tr TokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("неверный JSON /oauth: %w (body: %s)", err, string(body))
	}

	if tr.AccessToken == "" {
		return "", fmt.Errorf("пустой access_token в ответе /oauth")
	}

	c.accessToken = tr.AccessToken
	c.tokenExpires = time.Now().Add(time.Duration(tr.ExpiresIn-60) * time.Second)

	log.Printf("✅ Токен получен, действует %d сек", tr.ExpiresIn)
	return tr.AccessToken, nil
}

// GenerateRecipe генерирует рецепт.
func (c *Client) GenerateRecipe(userRequest string, prefs *models.UserPreferences) (string, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return "", fmt.Errorf("не удалось получить токен: %w", err)
	}

	systemPrompt := buildSystemPrompt(prefs)

	chatReq := ChatRequest{
		Model: "GigaChat", // ✅ Или "GigaChat-Pro", если у вас есть доступ
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userRequest},
		},
	}

	reqBody, err := json.Marshal(chatReq)
	if err != nil {
		return "", fmt.Errorf("ошибка сериализации: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("ошибка создания запроса: %w", err)
	}

	rqUID := generateUUID()
	log.Printf("📩 Запрос к /chat/completions: RqUID=%s", rqUID)

	// ✅ ОБЯЗАТЕЛЬНЫЕ заголовки (все!)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("RqUID", rqUID)                       // 🔑 ДОБАВЛЕНО: обязательно для SynGX
	req.Header.Set("X-Client-ID", c.clientID)           // 🔑 ИСПРАВЛЕНО: должен быть ваш clientID
	req.Header.Set("X-Request-ID", generateUUID())
	req.Header.Set("X-Session-ID", "sess-"+time.Now().UTC().Format("20060102T150405Z"))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка вызова /chat/completions: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Логируем статус и начало тела для отладки
	log.Printf("📡 Ответ API: %d, body[:200]=%q", resp.StatusCode, string(body)[:min(len(body), 200)])

	if resp.StatusCode == http.StatusUnauthorized {
		c.mu.Lock()
		c.accessToken = ""
		c.tokenExpires = time.Time{}
		c.mu.Unlock()
		return c.GenerateRecipe(userRequest, prefs) // один раз
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("ошибка парсинга ответа: %w (raw: %s)", err, string(body))
	}

	// Проверка на ошибку в теле ответа (иногда 200 + error)
	if chatResp.Error.Message != "" {
		return "", fmt.Errorf("модель вернула ошибку: %s (type: %s)", chatResp.Error.Message, chatResp.Error.Type)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("нет вариантов в ответе")
	}

	content := chatResp.Choices[0].Message.Content
	if content == "" {
		return "", fmt.Errorf("пустой content в ответе")
	}

	log.Printf("✅ Получен ответ длиной %d символов", len(content))
	return content, nil
}

// Вспомогательная функция
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// buildSystemPrompt — как раньше (не менялся)
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

📝 Формат ответа:
*1. Название блюда*

_Краткое пояснение: почему оно подходит под цель/тип питания_

*⏱️ Время:* X мин 
*🔥 Сложность:* легко / средне / сложно  
*🍽 Порций:* 1–2  

*Ингредиенты*  
1. Продукт — кол-во (грамм/мл/шт/ст.л.)  
2. ...  

*Пошаговый рецепт*  
1. Шаг 1: кратко, с акцентом на ключевые моменты (не пережарить, не пересолить и т.д.)  
2. Шаг 2: …  
…  

*💡 Шеф-совет*  
Один практичный лайфхак: как ускорить, упростить, улучшить вкус или сохранить блюдо.  
→ Обязательно добавь **уникальную деталь** — например, научный факт, историю блюда или неочевидную замену.

📊 Пищевая ценность (на 1 порцию, ~350–450 г)  
- *Ккал*: ~XXX  
- *Белки*: X г  
- *Жиры*: X г  
- *Углеводы*: X г  
→ Оценка приблизительная, но реалистичная. Если тип питания — «Похудение», ккал ≤ 450; «Набор массы» — ≥ 600.
`)

	if hasSettings && (prefs.Allergies != "" || prefs.Dislikes != "") {
		sb.WriteString("\n❗️ *Запрещено*:\n")
		if prefs.Allergies != "" {
			sb.WriteString(fmt.Sprintf("- Использовать %s — даже в скобках/альтернативах.\n", prefs.Allergies))
		}
		if prefs.Dislikes != "" {
			sb.WriteString(fmt.Sprintf("- Использовать %s — даже в скобках/альтернативах.\n", prefs.Dislikes))
		}
		sb.WriteString(`- Упоминать «дорогие» ингредиенты (авокадо, кешью, кокосовое молоко) без явной бюджетной альтернативы.  
- Писать «по вкусу» — всегда указывай диапазон (например: «соль — ¼–½ ч.л.»).  
`)
	}

	return sb.String()
}

// generateUUID — как раньше
func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}