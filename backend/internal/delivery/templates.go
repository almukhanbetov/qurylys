package delivery

import "fmt"

// Message kinds.
const (
	KindRegister          = "register"
	KindAlreadyRegistered = "already_registered"
	KindResetPassword     = "reset_password"
	KindAddContact        = "add_contact"
	KindTest              = "test"
)

type template struct{ subject, text string }

var templates = map[string]map[string]template{
	"kk": {
		KindRegister: {"QURYLYS B2B: тіркелуді растау",
			"Сәлеметсіз бе!\n\nТіркелуді растау коды: %s\n\nКод 15 минут жарамды. Оны ешкімге айтпаңыз.\nЕгер сіз тіркелмеген болсаңыз, бұл хатты елемеңіз.\n\nQURYLYS B2B\n"},
		KindAlreadyRegistered: {"QURYLYS B2B: аккаунт бар",
			"Сәлеметсіз бе!\n\nБұл email-мен тіркелуге әрекет жасалды, бірақ аккаунт бұрыннан бар. Кабинетке кіріңіз немесе құпиясөзді қалпына келтіріңіз.\nЕгер бұл сіз болмасаңыз, ештеңе істеудің қажеті жоқ.\n\nQURYLYS B2B\n"},
		KindResetPassword: {"QURYLYS B2B: қолжетімділікті қалпына келтіру",
			"Сәлеметсіз бе!\n\nҚұпиясөзді қалпына келтіру коды: %s\n\nКод 15 минут жарамды. Оны ешкімге айтпаңыз.\nЕгер сіз сұратпаған болсаңыз, бұл хатты елемеңіз.\n\nQURYLYS B2B\n"},
		KindAddContact: {"QURYLYS B2B: email растау",
			"Сәлеметсіз бе!\n\nEmail растау коды: %s\n\nКод 15 минут жарамды. Оны ешкімге айтпаңыз.\nЕгер сіз сұратпаған болсаңыз, бұл хатты елемеңіз.\n\nQURYLYS B2B\n"},
		KindTest: {"QURYLYS B2B: тексеру хаты", "Хат жіберу жұмыс істейді.\n\nQURYLYS B2B\n"},
	},
	"ru": {
		KindRegister: {"QURYLYS B2B: подтверждение регистрации",
			"Здравствуйте!\n\nКод подтверждения регистрации: %s\n\nКод действует 15 минут. Никому его не сообщайте.\nЕсли вы не регистрировались, просто проигнорируйте это письмо.\n\nQURYLYS B2B\n"},
		KindAlreadyRegistered: {"QURYLYS B2B: аккаунт уже есть",
			"Здравствуйте!\n\nС этим email попытались зарегистрироваться, но аккаунт уже существует. Войдите в кабинет или восстановите пароль.\nЕсли это были не вы, ничего делать не нужно.\n\nQURYLYS B2B\n"},
		KindResetPassword: {"QURYLYS B2B: восстановление доступа",
			"Здравствуйте!\n\nКод для восстановления пароля: %s\n\nКод действует 15 минут. Никому его не сообщайте.\nЕсли вы его не запрашивали, просто проигнорируйте это письмо.\n\nQURYLYS B2B\n"},
		KindAddContact: {"QURYLYS B2B: подтверждение email",
			"Здравствуйте!\n\nКод подтверждения email: %s\n\nКод действует 15 минут. Никому его не сообщайте.\nЕсли вы его не запрашивали, просто проигнорируйте это письмо.\n\nQURYLYS B2B\n"},
		KindTest: {"QURYLYS B2B: проверочное письмо", "Отправка писем работает.\n\nQURYLYS B2B\n"},
	},
}

// mailText falls back to Kazakh, the primary language. No user-supplied text is
// included, so the mail cannot be abused to send someone else's content.
func mailText(m Message) (subject, text string) {
	set, ok := templates[m.Locale]
	if !ok {
		set = templates["kk"]
	}
	t := set[m.Kind]
	if m.Code != "" {
		return t.subject, fmt.Sprintf(t.text, m.Code)
	}
	return t.subject, t.text
}
