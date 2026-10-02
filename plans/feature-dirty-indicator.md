# Фича: индикатор dirty-состояния в шапке

## Цель

`model.dirty` (`app.go:78`) уже есть, но в заголовке `renderHeader`
(`render_editor.go`) не видно, что файл не сохранён. Добавить визуальный маркер.

## Предпосылки

- Поле `dirty bool` в `model` (`app.go:78`), выставляется через `markDirty`
  (`session.go`).
- Заголовок — `renderHeader` / `currentFileName` в `render_editor.go`.

## Чек-лист

- [ ] В `currentFileName`/`renderHeader` добавить маркер `*` (или `●`) рядом с именем
      файла, когда `dirty`; без `dirty` — пусто.
- [ ] Сброс маркера после сохранения/автосейва (там же, где `dirty=false`).
- [ ] RUNE-корректный рендер (не ломать ширину шапки и `truncateWidth`).
- [ ] Тесты: заголовок содержит/не содержит маркер по `dirty`.
- [ ] Маркер и строки заголовка выносить в i18n (см. plans/i18n.md).

## Terms of Done

- [ ] Поле визуально показывает «есть несохранённые изменения».
- [ ] `go test ./...` и `go build -o bin/curlyk.exe ./cmd/curlyk` проходят.