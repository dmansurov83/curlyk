# Фича: десктопный редактор .http на Fyne

## Цель

Кроме TUI — оконный редактор тех же `.http`-файлов: свой виджет на Fyne, подсветка из
собственного лексера `internal/httpfile`, цвета из `internal/theme`, общий
`appsettings.yml` с TUI. GUI переиспользует `httpfile`/`settings`/`theme`/`runner`, а не
дублирует их.

## Решения (принято)

- Лежит в основном модуле: `internal/gui` + `cmd/curlyk-gui`. Fyne добавлен в корневой
  `go.mod`; сборка TUI (`./cmd/curlyk`) по-прежнему не требует cgo.
- Свой виджет редактора, без вендоренного чужого кода (первый прототип на
  `github.com/ispapp/psshclient/pkg/codeditor` оставлен как сравнительная база в
  `D:\workspace\go-ui-test\curlyk-proto`, в репозиторий не переносится).
- Виджет не знает про сеть: запуск запросов — отдельный слой поверх `internal/runner`.
- Позиции и `MinSize` считаются по измеренным метрикам шрифта
  (`fyne.MeasureText`), а не по «ширина символа ≈ 0.6 × кегль».

## Что уже сделано

- `internal/gui/highlight.go` — сегменты строк из `httpfile.Lex`: абсолютные смещения,
  заполнение пропусков, `{{...}}` отдельным сегментом (`TokVariable`).
- `internal/gui/ansi.go`, `internal/gui/theme.go` — палитра из `theme.Scheme`
  (ANSI-256 → RGB), `theme.ByNameOrFile` для кастомных `themes/*.theme`.
- `internal/gui/doc.go` — буфер и каретка: правки, движения, колонки в рунах.
- `internal/gui/editor.go`, `internal/gui/renderer.go` — виджет: ввод, клик по позиции,
  каретка, гуттер, статус; `OnChanged`/`OnCaretMoved`.
- `internal/gui/app.go`, `cmd/curlyk-gui/main.go` — окно: открыть/сохранить, смена темы,
  номера строк, кегль, строка статуса, подкрутка к каретке, Ctrl+S/O/T.
- Тесты: `internal/gui/doc_test.go`, `internal/httpfile/lexer_test.go`.
- Побочно: `internal/httpfile/lexer.go` больше не создаёт токены строки запроса с
  `Line: 0` — из-за этого в TUI не подсвечивались метод, URL и версия HTTP. Тест
  `internal/tui/editor_test.go` (TestRenderEditorNoPanic) переведён на `stripANSI`,
  так как раньше проходил только благодаря тому, что строка запроса была неокрашенной.

## Чек-лист

- [x] Выделение мышью и клипборд: `fyne.Shortcutable` + `fyne.Clipboard`, Ctrl+C/X/V/A.
- [x] Undo/redo (Ctrl+Z / Ctrl+Y): стек правок в `doc.go`.
- [x] JSON-подсветка тела запроса: вынести логику `internal/tui/jsoncolor.go` в общий
      пакет (`internal/jsonlex`) и подключить в `internal/gui/highlight.go`.
- [x] Поиск (Ctrl+F): отделить модель поиска от lipgloss-рендера
      (`internal/search`, используется и TUI, и GUI).
- [x] Автодополнение методов/заголовков/`{{переменных}}` на базе `internal/completion`
      (сборщик контекста — общий `completion.ContextForSource`).
- [x] Запуск запроса (Ctrl+Enter) и панель ответа через `internal/runner`.
- [x] Профили (`settings.ActiveProfile`) и переключение окружения из GUI
      (общий пакет `internal/profiles`).
- [x] Восстановление каретки и скролла на файл (`settings.FileCursors`).
- [x] Виртуализация рендера: редактор скроллится сам и рисует только видимые строки.
- [x] Инкрементальный re-lex по блокам вместо полного на каждое нажатие.
- [x] UI-строки — ключами i18n; в `app.go` нет зашитых русских строк.
- [x] Горячие клавиши: `primaryModifier()` даёт Cmd на macOS и Ctrl на Windows/Linux;
      дополнительно учтена русская раскладка ЙЦУКЕН (Ctrl+Ы = Ctrl+S и т.п.).
- [x] CI: `.github/workflows/gui.yml` — job GUI-сборки (ubuntu + `mingw-w64`,
      `CGO_ENABLED=1`, `go test ./...`); `release.yml` не затронут.
- [x] README: раздел про сборку и запуск GUI, требование cgo + MinGW.

## Известные ограничения

- Клик и каретка измеряются по рунам шрифта, но жирные сегменты (метод) измеряются
  обычным начертанием, поэтому в строках, где жирные и обычные сегменты чередуются
  (`GET {{host}}/x`), клик всё ещё может попасть на соседний символ.
- `internal/gui` требует cgo: `go build ./...` на машине без gcc не пройдёт, нужно
  собирать цель явно (`go build ./cmd/curlyk` для TUI).
- Выполнение запроса в GUI переиспользует `internal/runner`; ответ показывается как
  текст без встроенной подсветки.

## Terms of Done

- [x] `go test ./...` зелёный; TUI собирается с `CGO_ENABLED=0`, GUI — с `CGO_ENABLED=1`.
- [x] Правка, сохранение и подсветка `.http` в окне работают без расхождения колонок и
      наложения строк (проверено скриншотом окна: ввод и Ctrl+S сохранили файл на диск).
- [x] Правки в общих пакетах не меняют поведение TUI, кроме исправленной подсветки
      строки запроса.
