# Фича: показ save-as и new-profile через модальные диалоги с полем ввода

## Цель

Сейчас создание/сохранение по имени рисуется примитивными строками-промптами под
панелями, а ввод обрабатывается отдельными блоками в `keys.go`. Затронуты две
поверхности:
- **save-as** — Ctrl+S на безымянном буфере (промпт `render.go:76-83`).
- **new-profile** — создание нового окружения-профиля по кнопке "+ Новый профиль"
  в сайдбаре (промпт `render.go:85-92`).

Перевести обе на единый модальный диалог с полем ввода и кнопками, задействовав
готовую абстракцию `dialogBox` (аналог: стандартные диалоги "Save as"/"New
profile" в Postman / VS Code).

Ключевой результат — `dialogBox` получает недостающую возможность **поля ввода**,
которая пригодится и другим фичам (подтверждения, переназвание).

## Предпосылки

- Абстракция `dialogBox` **не используется** ни в одном вызове `newDialog`
  (единственный вызов — тест `internal/tui/dialog_test.go`). Рендер-оверлей
  `renderDialogOverlay`, геометрия `dialogGeom`, хит-тест `dialogButtonAt` и
  `dialogContains`, обработка клавиш `handleDialogKey`, мыши (`mouse.go:53-79`)
  готовы и ждут применения.
- Текущий save-as:
  - `beginSaveAs()` `internal/tui/files.go:312` создаёт `m.saveAs *textinput.Model`.
  - Обработка клавиш — `keys.go:110-125` (отдельный `if m.saveAs != nil`).
  - Рендер промпта — `render.go:76-83` (строка под панелями).
  - `saveBufferAs(name)` `internal/tui/files.go:322` — фактическая запись и bind
    файла с добавлением `.http`.
- Текущий new-profile:
  - `beginProfileAs()` `internal/tui/profiles.go:171` создаёт
    `m.profileAs *textinput.Model`.
  - Обработка клавиш — `keys.go:127-144` (отдельный `if m.profileAs != nil`).
  - Рендер промпта — `render.go:85-92` (строка под панелями).
  - `createProfile(name)` `internal/tui/profiles.go:182` — создание файла
    `<name>.profile` с дефолтным `@var host`, перезагрузка списка и активация.
  - Открывается из `mouse.go:147-151` (клик по "+ Новый профиль").
- i18n: ключи `saveAs.prompt` и `placeholder.filename` (`internal/i18n/i18n.go`
  ru:76,104 / en:210,238); профили — `profile.createPrompt`,
  `placeholder.profileName` (ru:154-155 / en:288-289). Новые строки — по
  `plans/i18n.md`.
- Диалог рисуется через `cellbuf`-оверлей поверх панелей (`renderDialogOverlay`,
  вызывается в `render.go:64-66`); `handleDialogKey` (`dialog.go:278`) приоритетно
  перехватывает клавиши раньше всех (`keys.go:89-91`).

## Дизайн

### Поле ввода внутри dialogBox

`dialogBox` (`internal/tui/dialog.go`) расширяется необязательным полем ввода:

- Новое поле `input *textinput.Model` (nil = диалог без ввода; существующие
  диалоги не меняют поведения).
- `newDialog` оставляем как есть; добавляем конструктор
  `newInputDialog(title string, input *textinput.Model, buttons []dialogButton, onDone dialogHandler)`,
  который строит диалог с полем ввода как строкой body. Либо вводим `setInput`
  вспомогательный — решает реализация, но без изменения существующего API в
  худшую сторону.
- `rows()`: при наличии `input` под заголовком рендерится одна строка ввода
  (`input.View()` / `input.Placeholder`), затем buttonRow. Ширина совпадает с
  `dialogContentWidth`.
- `handleDialogKey`: при наличии `input` печатаемые runes и клавиши
  `"backspace"`, `"delete"`, `"left"`, `"right"`, `"home"`, `"end"` направляются в
  `input.Update`; `Tab`/стрелки остаются для кнопок. `Enter` подтверждает
  (как сейчас), `Esc`/`ctrl+c` отменяет.
- Фокус ввода по умолчанию активен (ввод первым принимает печатаемые клавиши,
  пока пользователь не перешёл Tab'ом на кнопки).

### Save-As на диалоге

- `beginSaveAs()` переписывается на `newInputDialog`:
  - заголовок `saveAs.title` ("Сохранить файл"),
  - поле ввода с `placeholder.filename`,
  - кнопка `dialogConfirm` "saveAs.save" ("Сохранить", default) — при
    подтверждении вызывает `saveBufferAs(имя)`;
  - кнопка `dialogCancel` "saveAs.cancel" ("Отмена") — для клика мышью и
    явности (Esc уже отменяет в `handleDialogKey`).
- `m.saveAs` поле (`app.go:73`), его блок в `keys.go:110-125` и промпт
  `render.go:76-83` удаляются.
- `saveBufferAs()` (`files.go:322`) остаётся прежней.

### New-Profile на диалоге

- `beginProfileAs()` переписывается на тот же `newInputDialog`:
  - заголовок `profile.titleDialog` ("Новый профиль"),
  - поле ввода с `placeholder.profileName`,
  - кнопка `dialogConfirm` "profile.create" ("Создать", default) — при
    подтверждении вызывает `createProfile(имя)`;
  - кнопка `dialogCancel` "profile.cancel" ("Отмена").
- `m.profileAs` поле (`app.go:75`), его блок в `keys.go:127-144`, промпт
  `render.go:85-92` и ключ `profile.createPrompt` удаляются.
- `createProfile()` и логика открытия из `mouse.go:147-151` остаются;
  `beginProfileAs()` больше не трогает `m.active` (диалог модальный и сам
  поднимается поверх панелей — активная панель не важна).

### Удаление мёртвого состояния

После перевода поля `m.saveAs` и `m.profileAs` в `internal/tui/app.go` и их
блоки в `render.go` и `keys.go` убираются полностью (оба инпута живут только в
`input` диалога).

## Чек-лист

- [x] `dialog.go`: добавить поле `input` в `dialogBox`.
- [x] `dialog.go`: конструктор диалога с полем ввода (без ломки `newDialog`).
- [x] `dialog.go`: `rows()` рендерит строку ввода под заголовком.
- [x] `dialog.go`: `handleDialogKey` направляет печатаемые/управляющие клавиши
      в `input.Update`; `Enter` подтверждает.
- [x] `dialog.go`: ширина ввода согласована с `dialogContentWidth`.
- [x] `files.go:312`: `beginSaveAs()` открывает диалог с полем ввода и кнопками.
- [x] `profiles.go:171`: `beginProfileAs()` открывает диалог с полем ввода и
      кнопками; открытие из `mouse.go:147` не трогает `m.active`.
- [x] `keys.go:110-144` и `render.go:76-92`: удалить блоки save-as и
      new-profile и их промпты.
- [x] `app.go:73,75`: удалить поля `m.saveAs` и `m.profileAs`.
- [x] i18n (`internal/i18n/i18n.go` ru/en): ключи `saveAs.title`, `saveAs.save`,
      `saveAs.cancel`, `profile.titleDialog`, `profile.create`, `profile.cancel`;
      переиспользовать `placeholder.filename`/`placeholder.profileName`;
      удалить `saveAs.prompt` и `profile.createPrompt` (см. plans/i18n.md).
- [x] Тесты (`dialog_test.go`, `newfile_test.go`, `profiles_test.go`):
      ввод имени файла через диалог → `saveBufferAs`; ввод имени профиля через
      диалог → `createProfile`; Esc отменяет оба; печатаемые клавиши идут в поле,
      а не в кнопки; обновить `TestNewBufferThenSave` и профильные тесты.
- [x] Тесты мыши: клик по кнопке "Сохранить"/"Создать" подтверждает диалог.
- [x] README: упомянуть диалог save-as, если раздел сохранения есть.

## Terms of Done

- [x] Ctrl+S на безымянном буфере открывает модальный диалог с полем имени и
      кнопками "Сохранить"/"Отмена"; Enter сохраняет, Esc/клик вне рамки/Отмена —
      закрывает без записи.
- [x] "+ Новый профиль" в сайдбаре открывает диалог с полем имени профиля и
      кнопками "Создать"/"Отмена"; Enter создаёт `.profile` и активирует,
      Esc/Отмена закрывает без создания.
- [x] Введённое имя файла сохраняется как `.http` и файл привязывается
      (`saveBufferAs`); имя профиля → `createProfile`.
- [x] `m.saveAs` и `m.profileAs`, промпты `saveAs.prompt`/`profile.createPrompt`
      и их блоки в keys/render удалены без остатков.
- [x] `dialogBox` с полем ввода не ломает существующие (None-input) диалоги —
      `dialog_test.go` проходит.
- [x] `go test ./...` и `go build -o bin/curlyk.exe ./cmd/curlyk` проходят.