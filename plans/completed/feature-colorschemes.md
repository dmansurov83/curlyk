# Фича: Цветовые схемы (color schemes)

## Цель

Дать пользователю выбирать тему оформления TUI — набор ANSI-цветов для синтаксиса,
панелей, меню, статус-бара и т.п. Сейчас все цвета захардкожены в `internal/tui/`
как `lipgloss.Color("…")`. Добавить встроенные схемы **Default** (текущая палитра,
чтобы ничего не сломать), **Darkula** (тёмная, по мотивам Darcula из JetBrains) и
**Light** (светлая), а также **пользовательские темы**: файлы `themes/<name>.theme`
(YAML) в рабочей папке, которые автоматически попадают в список схем. Шаблон кастомной
темы генерируется аргументом запуска из встроенной, чтобы пользователь не писал ~30
полей вручную. Аналог — `Preferences → Color Theme` в JetBrains/VS Code, `SET NORMAL`
в Vim, `color scheme` в `~/.gitconfig`; кастомные темы — как themes в `.vim/colors`
или VS Code theme JSON.

## Предпосылки

- Все цвета — это `lipgloss.Color(string)` в ANSI-палитре 256 (0–255), захардкожены в
  файлах пакета `internal/tui`:
  - `highlight.go:35-51` — синтаксис HTTP: `methodStyle`, `urlStyle`, `httpVerStyle`,
    `headerNameStyle`, `headerValStyle`, `bodyStyle`, `commentStyle`, `varStyle`,
    `separatorStyle`, `optionStyle`, `otherStyle`, `cursorStyle`, `selStyle`.
  - `jsoncolor.go:18-25` + `:203` — JSON: `jsonKeyStyle`, `jsonStringStyle`,
    `jsonNumberStyle`, `jsonBoolStyle`, `jsonNullStyle`, `jsonPunctStyle`; выделение
    `Background(lipgloss.Color("24"))`.
  - `render_editor.go:14-29, 42-44, 290-294` — гуттер (`runIconStyle`,
    `iconBlockStyle`, `numCurStyle`, `numMutedStyle`, `scrollbarThumbStyle`,
    `scrollbarTrackStyle`), хедер (`Background("235")/Foreground("252")`), статус-бар
    (`Background("236")/Foreground("252")`), `borderColor()` (`212`/`240`, `:59-64`).
  - `render.go:11-14` — файловая панель (`fileSelStyle`, `fileHoverStyle`).
  - `render_resp.go:12, 152-170` — статус "Выполняется" (`214`), кнопка копирования
    (`copyButtonRow`/`copyButtonRowHover`).
  - `actionmenu.go:33-40` — попап (`menuSelStyle`, `menuHoverStyle`, `menuNormalStyle`).
  - `find.go:33-34, 407-411` — поиск (`findStyle`, `findCurStyle`, строка поиска).
  - `help.go:51-53` — справка (`helpKeyStyle`, `helpDescStyle`, `helpTitleStyle`).
  - `profiles.go:18-30` — профили (`profileSepStyle`, `profileTitleStyle`,
    `profileActiveStyle`, `profileSelStyle`, `profileNewSelStyle`).
  - `dialog.go:161, 174` — диалог (`dialogTitleStyle`, рамка `212`).
- `app.go:196-197` загружает `settings.Load()` и применяет locale; хоткей переключения
  языка `Ctrl+L` — `keys.go:183-185` → `lang.go:toggleLanguage()`. Это образец для
  переключения темы на лету с персистом в `appsettings.yml`.
- `Settings` (internal/settings/settings.go) уже хранит `Lang string yaml:"lang,omitempty"`.
  По аналогии добавим `Theme string yaml:"theme,omitempty"`.
- Прецедент декларативных файлов в рабочей папке — `*.profile`: `filesPanel.loadProfiles()`
  в `files.go`/`profiles.go` подхватывает их и показывает в файловой панели. Точно так же
  будем подхватывать `themes/*.theme`.
- CLI: `cmd/curlyk/main.go` (`initialArgs()`, `os.Args[1:2]`). Сейчас первый аргумент —
  только путь к `.http` файлу. Добавим флаг-слово для генерации шаблона темы.
- i18n: `plans/completed/i18n.md`, ключи статусов в `internal/i18n/i18n.go` (пример —
  `status.lang`, `lang.ru`). UI-строки схем должны добавляться ключами.

## Дизайн

1. **Новый пакет `internal/theme`** со схемами как данными, а не кодом стилей:
   - тип `type Color string` (или просто использ/ string) и `type Scheme struct`
     с полями-цветами для каждого захардкоженного сейчас стиля (перечень из
     Предпосылок). Плюс нормализованные имена полей для хранения/маппинга.
   - `func Default() Scheme` — текущая палитра (тождественное поведение).
   - `func byName(name string) (Scheme, bool)` — lookup "default" | "darkula" | "light".
   - `Darkula()` и `Light()` — две дополнительные схемы.
- Хранение стыкуется с lipgloss: храним строки ANSI-адаптивных 256-цветов;
      lipgloss их рендерит через Style.
   - **Кастомные схемы из YAML**: `func LoadFile(path string) (Scheme, error)`.
      Файл задаёт только изменённые поля (частично): недостающие берутся из `Default()`
      (fallback), так что пользователь меняет минимум. Формат — имя поля:` значение`
      в ANSI-256 (как в коде). YAML-парсер уже в зависимостях (`gopkg.in/yaml.v3`,
      используется в `internal/settings/settings.go`).
   - **Каталог `themes/`**: `func ListFiles() []string` сканирует `themes/*.theme`
      в рабочей папке и возвращает имена без расширения. Имя кастомной схемы не должно
      совпадать с встроенными (`default`/`darkula`/`light`) — при конфликте приоритет у
      встроенной (или валидируем и сообщаем).
   - Все UI-строки схем (названия, описания) — ключи в `internal/i18n` (справка,
      статус переключения, подсказки). Имена кастомных тем — это имя файла.
2. **Одна точка применения — `applyTheme(Scheme)`** в пакете tui. Сейчас стили —
   пакетные `var`, инициализируются один раз при старте. Переключение на лету
   требует пересоздания пакетных стилей, поэтому:
   - заменяем package-level `var XStyle = lipgloss.NewStyle()…` на package-level
     `var XStyle lipgloss.Style`, а цвета читаем из глобального текущего Scheme
     через функцию-геттер (например `schemes.current()`).
   - проще и надёжнее: ввести глобальную `var cur = theme.Default()` в пакете tui
     и помощник `func c(schemeColor string) lipgloss.Color`, который возвращает
     `lipgloss.Color(field)` из `cur`. В каждом месте вместо литерала `"212"`
     подставлять `c(theme.Main)`. Так стили пересчитываются при каждом рендере
     (рендер дешёвый, схема меняется редко), без реинициализации пакетных var.
   - Либо вариант с `applyScheme()` пересоздающим все пакетные var — дороже и
     хрупче (33 стиля). Выбираем геттер-подход: `cur` глобальный, `applyTheme`
     лишь меняет `cur` и статус-строку.
3. **Поле `Settings.Theme string yaml:"theme,omitempty"`** — персист выбора.
   Значением может быть имя встроенной схемы (`default`/`darkula`/`light`) или имя
   кастомного файла `themes/<name>.theme`. Пустое значение → `default`.
4. **Триггер переключения (утверждено)**:
   - **Быстрый цикл хоткеями** `Ctrl+\` (назад) и `Ctrl+]` (вперёд) — работают из
     любой панели, как `Ctrl+L` для языка (`keys.go` → `lang.go:toggleLanguage()`).
     Оба хоткея в `keys.go` свободны (в `knownModifierKey`, `:347-357` их нет).
     Каждое нажатие циклически перебирает **полный список**: встроенные
     `default → darkula → light` + кастомные из `themes/` — по порядку.
   - **Видимый список схем** в action-попапе (`actionmenu.go:43`, `menuItems()`):
     пункт «Цветовая схема», открывает вложенное меню выбора всех схем (встроенные +
     кастомные; текущая отмечена галочкой). Тот же `actionMenu` (рендер/мышь/клавиши).
   - Оба пути приводят к `applyTheme(name)` + `s.Theme = name; s.Save()`; статусная
     строка показывает имя (`Тема: Darkula`), как `status.lang`. `applyTheme` при
     получении кастомного имени загружает файл `themes/<name>.theme`.
5. **Применение на старте**: в `app.go:New()` после `settings.Load()` вызвать
   `applyTheme(theme.ByNameOrFile(s.Theme))`, аналогично `i18n.SetLocale(...)`.
6. **CLI-аргумент шаблона темы** (`cmd/curlyk/main.go`): аргумент
   `themetemplate <name>` (или `--theme-template <name>`) генерирует в рабочей папке
   `themes/<name>.theme` — полный набор полей встроенной схемы (по умолчанию `Darkula`,
   опционально `default`/`light`) с комментариями-подсказками у каждого поля. Не
   перезаписывает существующий файл без явного согласия (или пишет в `themes/` имя с
   суффиксом). После генерации программа не запускает TUI (чистая CLI-операция), пишет
   подтверждение в stdout. Тем самым пользователь получает готовый каркас и меняет
   только нужные поля.
7. **README + справка**: добавить пункт в help-панель (`help.theme`,
   `Ctrl+] / Ctrl+\`) и в README секцию про `appsettings.yml: theme`, файлы
   `themes/*.theme` и аргумент генерации шаблона.

## Чек-лист

- [x] `internal/theme/theme.go`: тип `Scheme`, `Default()` (= текущая палитра),
      `Darkula()`, `Light()`, `ByName(name)`/`ByNameOrFile(name)` с фолбэком на
      default, константы имён.
- [x] Вынести все захардкоженные цвета (перечень из Предпосылок) в поля `Scheme`;
      имена полей YAML = имена полей структуры.
- [x] `internal/theme/load.go`: `LoadFile(path)` (YAML, частичный fallback на
      `Default()`) и `ListFiles()` (сканирует `themes/*.theme`).
- [x] `internal/theme/themetemplate.go`: `WriteTemplate(name, base, outPath)` — пишет
      `themes/<name>.theme` полным набором полей `base` схемы с комментариями-
      подсказками; не перезаписывает без явного переопределения.
- [x] `internal/theme/theme_test.go`: юнит-тесты — `Darkula`/`Light` ≠ `Default`,
      `ByName("darkula")` находит, unknown → default, `LoadFile` частичного файла
      подставляет недостающие из `Default`, `ListFiles` находит `*.theme`,
      `WriteTemplate` создаёт парсящийся файл.
- [x] `settings.go`: поле `Theme string yaml:"theme,omitempty"` + тест persistence
      в `settings_test.go`.
- [x] `internal/tui/theme.go` (новый): `var cur = theme.Default()` + `applyTheme()`
      (для кастомного имени грузит файл `themes/<name>.theme`), помощник `c(field)`,
      чтение с хоткея/меню и на старте; `allSchemeNames()` возвращает встроенные +
      кастомные для цикла и меню.
- [x] i18n: ключи `theme.darkula`, `theme.default`, `theme.light`, статус
      `theme.changed`, пункт меню `theme.menu`, help `help.theme`,
      `theme.templateWritten` (для CLI) + en-варианты.
- [x] Заменить литералы цветов на `c(...)` в: `highlight.go`, `jsoncolor.go`,
      `render_editor.go` (включая `borderColor` и статус/хедер), `render.go`,
      `render_resp.go`, `actionmenu.go`, `find.go`, `help.go`, `profiles.go`,
      `dialog.go`.
- [x] `keys.go`: добавить `ctrl+\` и `ctrl+]` в `knownModifierKey` (:347-357) и в
      `switch` (:146): вызов `cycleTheme(delta)`; `theme.go` — `cycleTheme(±1)` по
      `allSchemeNames()` (встроенные + кастомные), статус + персист (как
      `toggleLanguage`).
- [x] `actionmenu.go`: пункт «Цветовая схема» в `menuItems()` — вложенное меню
      выбора всех схем (`allSchemeNames()`; текущая отмечена), вызывает `applyTheme`.
- [x] `cmd/curlyk/main.go`: распознать аргумент `themetemplate <name> [--base darkula]`
      до запуска TUI; сгенерировать `themes/<name>.theme`, вывести подтверждение,
      не входить в TUI.
- [x] Тесты: `go test ./...`; переключение цикла и пункта меню меняет схему и пишет
      `appsettings.yml.theme`; кастомная тема из `themes/*.theme` подхватывается;
      color-sensitive тесты остаются зелёными при `theme: default`.
- [x] README: секция про `theme` в `appsettings.yml`, файлы `themes/*.theme`,
      аргумент генерации шаблона и переключение схемы.
- [x] Сборка: `go build -o bin/curlyk.exe ./cmd/curlyk`.

## Terms of Done

- [x] `theme: default` (или пусто) даёт ровно текущую палитру — визуально ничего
      не изменилось, все существующие color-sensitive тесты зелёные.
- [x] Выбор `darkula` / `light` переключает весь UI (синтаксис, панели, статус,
      меню, поиск, справка, диалоги) без перезапуска.
- [x] `themes/<name>.theme` (YAML, частичные поля) попадает в список схем и
      применяется; имя совпадающее со встроенным валидируется.
- [x] Аргумент `themetemplate` генерирует рабочий файл кастомной темы без запуска TUI.
- [x] Выбор темы (встроенной или кастомной) персистится в `appsettings.yml` и
      восстанавливается при старте.
- [x] i18n-ключи для схем добавлены в `ru` и `en`.
- [x] `go test ./...` проходят; color-sensitive тесты зелёные при `theme: default`.
- [x] `go build -o bin/curlyk.exe ./cmd/curlyk` собирается.