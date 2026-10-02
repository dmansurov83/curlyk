# Фича: импорт OpenAPI/Swagger и Postman Collection в `.http`

## Цель

Расширить импорт (сейчас только cURL, `internal/curl`) до конвертации OpenAPI 3 /
Swagger 2 / Postman Collection в формат `.http`.

## Предпосылки

- Импорт cURL — `internal/curl` (import.go, to_http.go); паттерн «вставка по буферу →
  конвертация → вставка блока» уже есть.
- YAML-парсер уже в `go.mod` (`gopkg.in/yaml.v3`), JSON — стандартная библиотека.
- Новых тяжёлых зависимостей не нужно.

## Дизайн

- Приоритет форматов: OpenAPI 3.0 (YAML/JSON), Postman Collection v2.1, опц. Swagger 2.
- База URL из `servers[]`/`baseUrl` → переменная `{{baseUrl}}`, в начале файла секция
  `@var`.

## Чек-лист

- [ ] Парсер входного формата (OpenAPI 3.0 — YAML/JSON; Postman v2.1) в единую
      промежуточную модель.
- [ ] Генерация `.http`: `###` + `@name` (operationId / request name) +
      `METHOD path` + заголовки + пример тела (`requestBody` first example).
- [ ] `{{baseUrl}}` из `servers[0]`/Postman констант.
- [ ] UI: пункт в action menu или вставка по буферу (как cURL-паст); возможно флаг/
      file-аргумент.
- [ ] Тесты: фикстуры OpenAPI JSON + YAML, Postman Collection; золотой вывод `.http`.
- [ ] README.
- [ ] Новые UI-строки выносить ключами в i18n (см. plans/i18n.md).

## Terms of Done

- [ ] Подаётся OpenAPI-спека/Postman Collection → получается редактируемый `.http`,
      отражающий пути, методы, заголовки и примеры тел.
- [ ] `go test ./...` и `go build -o bin/curlyk.exe ./cmd/curlyk` проходят.