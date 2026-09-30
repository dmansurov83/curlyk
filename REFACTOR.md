# План рефакторинга HTTP Tool (httptool)

Цель — навести порядок в коде без изменения поведения. Проект зрелый, все тесты
зелёные (`go vet` чист, покрытие 62–85%): рефакторинг идёт как серия безопасных
шагов, после каждого из которых прогоняются `go vet ./...` и `go test ./...`.

> **Статус: выполнено.** Все шаги 1–4 завершены, `go build/vet/test` зелёные,
> бинарник `bin\httptool.exe` собирается, покрытие выросло (runner 84.6%,
> settings 80.0%).

## Шаг 1. Удаление мёртвого кода

Код, который не вызывается ни в проде, ни в тестах, — кандидаты на удаление.

| Код | Где | Примечание |
|---|---|---|
| Весь `json.go`, `jsontree.go` (JSON-дерево) | `internal/tui` | ✅ удалено (не подключено никуда). Тест `json_test.go` тоже удалён. |
| `jsonLooksLikeObject()`, `saveResponse()` | `app.go` | ✅ удалены. |
| `SaveAll()`, `ConfigDir()` | `settings.go` | ✅ удалены. |
| `SaveResponse()` | `runner.go` | ✅ удалён. |
| `editor.LineLen()` | `editor.go` | ✅ удалён. |
| `importMode.active` | `app.go` | ✅ удалён. |
| `selPos` struct | `app.go` | ✅ удалён. |
| `pendingVarSpans` | `lexer.go` | ✅ удалён. |
| `sortInts` | `highlight.go` | ✅ заменён на `slices.Sort`. |

## Шаг 2. Разбить `app.go` (2286 строк) на файлы по ответственности

Поведение не меняется — только перенос кода в одноимённый пакет `tui`.
**✅ Выполнено.** `app.go` сокращён с 2286 до 774 строк; созданы:

- `render.go` (548) — View и все render*, scrollbar, `formatBody`, `exampleHTTP`.
- `mouse.go` (380) — handleMouse, mouseToEditorCell/RespCell, scrollbar-клики.
- `files.go` (272) — filesPanel/filePicker, открытие/сохранение файлов.
- `clipboard.go` (210) — копирование/вставка/cURL-конверсия.
- `respsel.go` (130) — выделение в панели ответа.

## Шаг 3. Унифицировать расчёт геометрии панелей

Формула «left sidebar + editor mid + response right» захардкожена в ~5 местах
разными способами: `filesWidth()`, `half` в `handleMouse`, `mid` в `View`,
`fw+mid` в `paneAtX`. Ввести один пакетный хелпер и использовать везде.
**✅ Выполнено** — введён тип `paneLayout` и метод `model.layout()`
(`files`, `mid`, `half`); используются в `handleMouse`, `paneAtX`,
`mouseToEditorCell`, `mouseToRespCell`, `View`.

## Шаг 4. Мелкие улучшения

- `go.mod`: `go 1.27.1` → `go 1.27`. **✅**
- `sortInts` → `slices.Sort`; `sort.SliceStable` → `slices.SortStableFunc`. **✅**
- `minInt`/`maxInt` → встроенные `min`/`max`. **✅**
- `lipgloss.NewStyle()` из циклов рендера в package-level `var`
  (`runIconStyle`, `iconBlockStyle`, `numCurStyle`, `numMutedStyle`,
  `fileSelStyle`, `scrollbarThumbStyle`, `scrollbarTrackStyle`). **✅**
- `issues.txt` (кодировка windows-1251) перекодирован в UTF-8. **✅**

## Вне объёма (предложения на будущее)

- Тесты для `cmd/` (покрытие 0%) и больше тестов для `runner` (68%).
- Cгенерировать формат всей `.http`-структуры в один проход, не перепарсивая
  файл на каждый рендер (`Lex` + `ParseFile` вызываются многократно).