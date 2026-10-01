// Package i18n provides a lightweight user-interface string catalog with a
// simple translation function. It supports two locales, "ru" (the default) and
// "en", selected via the CURLYK_LANG environment variable or the optional
// settings.Lang preference.
//
// The only translation helper is T(key, args...): a label lookup with optional
// fmt.Sprintf argument substitution. Error text is always passed as a %s/%v
// argument, never as the format string, so an attacker-controlled message can
// never inject format verbs.
package i18n

import (
	"fmt"
	"os"
)

// current is the active locale code. It defaults to "ru"; SetLocale selects
// another supported locale and silently keeps "ru" for unknown codes.
var current = "ru"

// catalogs maps a locale code to its key->string translation table. An unknown
// key falls back to the "ru" entry (then to the key itself).
var catalogs = map[string]map[string]string{
	"ru": {
		// app.go statuses & messages
		"ready.initial":       "Готов. Ctrl+Enter — выполнить, ^Y — импорт cURL, ^K — copy as cURL, Tab — панель, ^D — дамп",
		"ready.main":          "Готов. Ctrl+Enter — выполнить, 2xEsc/F10 — выход, ^C/^X/^V — буфер, ^+стрелки — слова",
		"err.debugWrite":      "Ошибка записи debug.txt: %s",
		"status.debugDumped":  "Дамп записан в debug.txt",
		"err.noRequestCursor": "Нет HTTP-запроса под курсором",
		"status.running":      "Выполняется %s %s ...",
		"err.network":         "Сетевая ошибка: %s",
		"resp.netFailed":      "Сетевой запрос не выполнен",
		"resp.duration":       "Время: %s",
		"resp.statusCode":     "Ответ %d",
		"err.startup":         "Ошибка запуска: %s",
		"status.lang":         "Язык: %s",
		"lang.ru":             "Русский",
		"lang.en":             "Английский",

		// render.go
		"loading.editor": "Загрузка редактора...",
		"saveAs.prompt":  "Сохранить как: %s",
		"search.label":   "поиск: %s|",
		"file.new":       "+ Новый файл",

		// render_editor.go
		"header.file":  "Файл: %s",
		"file.unnamed": "новый файл",

		// render_resp.go
		"status.executing": "Выполняется...",
		"resp.empty":       "Выполните запрос: Ctrl+Enter\nПереключение панелей: Tab",
		"resp.copyButton":  "⧉  Скопировать ответ",

		// files.go
		"err.openFile":         "Не удалось открыть %s: %s",
		"status.opened":        "Открыт %s",
		"status.newFile":       "Новый файл (Ctrl+S — сохранить)",
		"err.save":             "Не удалось сохранить: %s",
		"status.saved":         "Сохранено в %s",
		"placeholder.filename": "имя файла.http",

		// actionmenu.go
		"menu.run":             "▶ Выполнить",
		"menu.copyCurl":        "⧉ Копировать как cURL",
		"menu.formatJson":      "{} Форматировать JSON",
		"err.noBodyFormat":     "Нет тела запроса для форматирования",
		"err.notJson":          "Тело запроса не является валидным JSON",
		"err.formatJson":       "Ошибка форматирования JSON: %s",
		"err.bodyRange":        "Не удалось определить строки тела запроса",
		"status.jsonFormatted": "JSON тела отформатирован",

		// clipboard.go
		"err.noSelection":      "Нет выделения для копирования",
		"err.copy":             "Не удалось скопировать: %s",
		"status.copiedChars":   "Скопировано (%d симв.)",
		"err.noRequest":        "Нет запроса под курсором",
		"err.copyCurl":         "Не удалось скопировать cURL: %s",
		"err.noRespSelection":  "Нет выделения в ответе для копирования",
		"err.copyGeneric":      "Не удалось копировать: %s",
		"status.copiedResp":    "Скопировано из ответа (%d симв.)",
		"err.noResponse":       "Нет ответа для копирования",
		"status.respCopied":    "Ответ скопирован (%d симв.)",
		"err.cut":              "Не удалось вырезать: %s",
		"status.cut":           "Вырезано",
		"status.lineDeleted":   "Строка удалена",
		"err.clipboardEmpty":   "Буфер обмена пуст",
		"err.convertCurl":      "Не удалось конвертировать cURL: %s",
		"status.pasted":        "Вставлено",
		"status.curlCopied":    "cURL скопирован",
		"status.curlConverted": "cURL конвертирован в запрос",
	},
	"en": {
		// app.go statuses & messages
		"ready.initial":       "Ready. Ctrl+Enter — run, ^Y — import cURL, ^K — copy as cURL, Tab — switch pane, ^D — dump",
		"ready.main":          "Ready. Ctrl+Enter — run, 2xEsc/F10 — quit, ^C/^X/^V — clipboard, ^+arrows — words",
		"err.debugWrite":      "Error writing debug.txt: %s",
		"status.debugDumped":  "Dump written to debug.txt",
		"err.noRequestCursor": "No HTTP request under cursor",
		"status.running":      "Running %s %s ...",
		"err.network":         "Network error: %s",
		"resp.netFailed":      "Network request failed",
		"resp.duration":       "Time: %s",
		"resp.statusCode":     "Response %d",
		"err.startup":         "Startup error: %s",
		"status.lang":         "Language: %s",
		"lang.ru":             "Russian",
		"lang.en":             "English",

		// render.go
		"loading.editor": "Loading editor...",
		"saveAs.prompt":  "Save as: %s",
		"search.label":   "search: %s|",
		"file.new":       "+ New file",

		// render_editor.go
		"header.file":  "File: %s",
		"file.unnamed": "new file",

		// render_resp.go
		"status.executing": "Running...",
		"resp.empty":       "Run a request: Ctrl+Enter\nSwitch panes: Tab",
		"resp.copyButton":  "⧉  Copy response",

		// files.go
		"err.openFile":         "Failed to open %s: %s",
		"status.opened":        "Opened %s",
		"status.newFile":       "New file (Ctrl+S — save)",
		"err.save":             "Failed to save: %s",
		"status.saved":         "Saved to %s",
		"placeholder.filename": "filename.http",

		// actionmenu.go
		"menu.run":             "▶ Run",
		"menu.copyCurl":        "⧉ Copy as cURL",
		"menu.formatJson":      "{} Format JSON",
		"err.noBodyFormat":     "No request body to format",
		"err.notJson":          "Request body is not valid JSON",
		"err.formatJson":       "Error formatting JSON: %s",
		"err.bodyRange":        "Could not determine request body lines",
		"status.jsonFormatted": "JSON body formatted",

		// clipboard.go
		"err.noSelection":      "No selection to copy",
		"err.copy":             "Failed to copy: %s",
		"status.copiedChars":   "Copied (%d chars)",
		"err.noRequest":        "No request under cursor",
		"err.copyCurl":         "Failed to copy cURL: %s",
		"err.noRespSelection":  "No response selection to copy",
		"err.copyGeneric":      "Failed to copy: %s",
		"status.copiedResp":    "Copied from response (%d chars)",
		"err.noResponse":       "No response to copy",
		"status.respCopied":    "Response copied (%d chars)",
		"err.cut":              "Failed to cut: %s",
		"status.cut":           "Cut",
		"status.lineDeleted":   "Line deleted",
		"err.clipboardEmpty":   "Clipboard is empty",
		"err.convertCurl":      "Failed to convert cURL: %s",
		"status.pasted":        "Pasted",
		"status.curlCopied":    "cURL copied",
		"status.curlConverted": "cURL converted to request",
	},
}

// SetLocale selects the active locale. Unknown or empty codes keep the current
// (default "ru") locale.
func SetLocale(lang string) {
	if lang == "" {
		return
	}
	if _, ok := catalogs[lang]; ok {
		current = lang
	}
}

// Locale returns the active locale code.
func Locale() string { return current }

// Toggle returns the other supported locale code (the one current is not).
func Toggle() string {
	if current == "en" {
		return "ru"
	}
	return "en"
}

// LangName returns the human-readable name of a locale code in the active
// language, used by the language-switch status message.
func LangName(code string) string {
	return T("lang." + code)
}

// Resolve picks the effective locale: the CURLYK_LANG environment variable
// wins, then the given preference (e.g. settings.Lang), then "ru".
func Resolve(pref string) string {
	if v, ok := os.LookupEnv("CURLYK_LANG"); ok && v != "" {
		if _, ok := catalogs[v]; ok {
			return v
		}
	}
	if _, ok := catalogs[pref]; ok {
		return pref
	}
	return "ru"
}

// T returns the translated string for key, substituting args with fmt.Sprintf
// on templates containing %v/%s verbs. Unknown keys fall back to the "ru"
// catalog and then to the key text itself.
func T(key string, args ...any) string {
	cat := catalogs[current]
	if cat == nil {
		cat = catalogs["ru"]
	}
	tmpl, ok := cat[key]
	if !ok {
		if ru, ok := catalogs["ru"][key]; ok {
			tmpl = ru
		} else {
			tmpl = key
		}
	}
	if len(args) > 0 {
		return fmt.Sprintf(tmpl, args...)
	}
	return tmpl
}
