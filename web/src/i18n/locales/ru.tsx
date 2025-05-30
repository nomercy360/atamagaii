export const dict = {
	stats: {
		title: 'Статистика',
		summary: 'Сводка обучения',
		loading: 'Загрузка статистики...',
		loadingActivity: 'Загрузка данных активности...',
		totalCards: 'Всего карточек',
		today: 'Сегодня',
		studyDays: 'Дней обучения',
		totalReviews: 'Всего повторений',
		currentStreak: 'Текущая серия',
		days: 'дней',
		totalStudyTime: 'Общее время',
		activityHistory: 'История активности',
		noActivity: 'Нет активности',
		less: 'Меньше',
		more: 'Больше',
	},
	home: {
		deck: 'Выберите колоду',
		add_deck: 'Добавить колоду',
		add_your_first_deck: 'Добавьте свою первую колоду',
		no_decks: 'Тут пустовато',
		new_cards_per_day: 'Новых карточек в день',
	},
	common: {
		loading: 'Загрузка...',
		error: 'Произошла ошибка',
		success: 'Успешно',
		save: 'Сохранить',
		saved: 'Сохранено!',
	},
	navigation: {
		home: 'Главная',
		cards: 'Карточки',
		tasks: 'Задания',
		stats: 'Статистика',
		profile: 'Профиль',
		importDeck: 'Импорт колоды',
	},
	deck: {
		settings: 'Настройки колоды',
		cards: 'Карточки',
		progress: 'Прогресс',
	},
	card: {
		front: 'Передняя сторона',
		back: 'Задняя сторона',
		edit: 'Редактировать карточку',
		delete: 'Удалить карточку',
		new: 'Новая карточка',
		save: 'Сохранить карточку',
	},
	task: {
		complete: 'Завершить',
		skip: 'Пропустить',
		next: 'Далее',
		previous: 'Назад',
		submit: 'Отправить ответ',
		nextTask: 'Следующее задание',
		tryAgain: 'Попробовать снова',
		selectAnswer: 'Выберите ответ',
		enterTranslation: 'Введите перевод',
		correct: 'Верно!',
		incorrect: 'Неверно!',
		translateToJapanese: 'Переведите на японский',
		yourTranslation: 'Ваш перевод',
		feedback: 'Обратная связь:',
		listenToAudio: 'Прослушайте аудио и ответьте на вопрос',
		loadingTasks: 'Загрузка заданий...',
		noTasksAvailable: 'Нет доступных заданий!',
		practiceToGenerateTasks: 'Чтобы появились задания, изучайте карточки в этой колоде. Задания появляются, когда карточки переходят в режим повторения.',
		checkAgain: 'Проверить снова',
		backToTasks: 'Вернуться к заданиям',
		noTasksAvailableDecks: 'Нет доступных заданий!',
		practiceToGenerateTasksDecks: 'Чтобы появились задания, изучайте карточки в ваших колодах. Задания появляются, когда карточки переходят в режим повторения.',
		checkAgainDecks: 'Проверить снова',
		backToDecks: 'Вернуться к колодам',
	},
	profile: {
		language: 'Язык',
		darkMode: 'Темная тема',
		logout: 'Выйти',
		username: 'Имя пользователя',
		displayName: 'Отображаемое имя',
		level: 'Уровень',
		points: 'Очки',
		enterName: 'Введите ваше имя',
		maxTasksPerDay: 'Максимум заданий в день',
		maxTasksHelp: 'Максимальное количество заданий, которое вы получите в день',
		taskTypes: 'Типы заданий',
		taskTypesHelp: 'Выберите типы заданий, которые вы хотите практиковать',
		taskTypeOptions: {
			vocabRecall: 'Запоминание слов',
			sentenceTranslation: 'Перевод предложений',
			audio: 'Аудирование',
		},
	},
} as const
