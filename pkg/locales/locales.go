package locales

import (
	_ "embed"
	"encoding/json"
	"log"
)

//go:embed locales.json
var localesJSON []byte

// Locales содержит все текстовые строки из locales.json
type Locales struct {
	MainMenu      MainMenu      `json:"main_menu"`
	SettingsMenu  SettingsMenu  `json:"settings_menu"`
	DietMenu      DietMenu      `json:"diet_menu"`
	GoalMenu      GoalMenu      `json:"goal_menu"`
	AllergiesMenu AllergiesMenu `json:"allergies_menu"`
	HabitsMenu    HabitsMenu    `json:"habits_menu"`
	DislikesMenu  DislikesMenu  `json:"dislikes_menu"`
	LikesMenu     LikesMenu     `json:"likes_menu"`
	ClearConfirm  ClearConfirm  `json:"clear_confirm"`
	ClearSuccess  ClearSuccess  `json:"clear_success"`
}

type MainMenu struct {
	Text    string `json:"text"`
	Buttons struct {
		Settings string `json:"settings"`
	} `json:"buttons"`
}

type SettingsMenu struct {
	Text   string `json:"text"`
	Fields struct {
		Diet      string `json:"diet"`
		Goal      string `json:"goal"`
		Allergies string `json:"allergies"`
		Habits    string `json:"habits"`
	} `json:"fields"`
	Buttons struct {
		Diet      string `json:"diet"`
		Goal      string `json:"goal"`
		Allergies string `json:"allergies"`
		Habits    string `json:"habits"`
		Clear     string `json:"clear"`
		Back      string `json:"back"`
	} `json:"buttons"`
}

type DietMenu struct {
	Text    string `json:"text"`
	Options struct {
		None string `json:"none"`
		Lose string `json:"lose"`
		Gain string `json:"gain"`
	} `json:"options"`
	Success string `json:"success"`
	Buttons struct {
		BackToSettings string `json:"back_to_settings"`
		BackToMain     string `json:"back_to_main"`
	} `json:"buttons"`
}

type GoalMenu struct {
	Text    string `json:"text"`
	Success string `json:"success"`
	Buttons struct {
		BackToSettings string `json:"back_to_settings"`
		BackToMain     string `json:"back_to_main"`
	} `json:"buttons"`
}

type AllergiesMenu struct {
	Text    string `json:"text"`
	Success string `json:"success"`
	Buttons struct {
		BackToSettings string `json:"back_to_settings"`
		BackToMain     string `json:"back_to_main"`
	} `json:"buttons"`
}

type HabitsMenu struct {
	Text    string `json:"text"`
	Buttons struct {
		Dislikes       string `json:"dislikes"`
		Likes          string `json:"likes"`
		BackToSettings string `json:"back_to_settings"`
	} `json:"buttons"`
}

type DislikesMenu struct {
	Text    string `json:"text"`
	Success string `json:"success"`
	Buttons struct {
		BackToHabits   string `json:"back_to_habits"`
		BackToSettings string `json:"back_to_settings"`
		BackToMain     string `json:"back_to_main"`
	} `json:"buttons"`
}

type LikesMenu struct {
	Text    string `json:"text"`
	Success string `json:"success"`
	Buttons struct {
		BackToHabits   string `json:"back_to_habits"`
		BackToSettings string `json:"back_to_settings"`
		BackToMain     string `json:"back_to_main"`
	} `json:"buttons"`
}

type ClearConfirm struct {
	Text    string `json:"text"`
	Buttons struct {
		Yes string `json:"yes"`
		No  string `json:"no"`
	} `json:"buttons"`
}

type ClearSuccess struct {
	Text    string `json:"text"`
	Buttons struct {
		ToSettings string `json:"to_settings"`
		ToMain     string `json:"to_main"`
	} `json:"buttons"`
}

var L *Locales

func init() {
	L = &Locales{}
	if err := json.Unmarshal(localesJSON, L); err != nil {
		log.Fatalf("Не удалось распарсить locales.json: %v", err)
	}
}

// Get возвращает указатель на локали
func Get() *Locales {
	return L
}
