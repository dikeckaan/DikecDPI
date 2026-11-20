package main

// Native menu and tooltip strings. The web UI has its own, larger catalog
// in frontend/i18n.js; these few cannot come from there because the menu
// bar menu is drawn by macOS.
var nativeText = map[string]map[string]string{
	"en": {"on": "Turn protection on", "off": "Turn protection off", "open": "Open Dikec DPI", "quit": "Quit and restore proxy", "tipOff": "Dikec DPI · off", "tipOn": "Dikec DPI · active · %s", "tipScan": "Dikec DPI · scanning"},
	"tr": {"on": "Korumayı aç", "off": "Korumayı kapat", "open": "Dikec DPI’ı aç", "quit": "Çıkış ve proxy’yi geri al", "tipOff": "Dikec DPI · kapalı", "tipOn": "Dikec DPI · aktif · %s", "tipScan": "Dikec DPI · taranıyor"},
	"ru": {"on": "Включить защиту", "off": "Выключить защиту", "open": "Открыть Dikec DPI", "quit": "Выйти и восстановить прокси", "tipOff": "Dikec DPI · выключено", "tipOn": "Dikec DPI · активно · %s", "tipScan": "Dikec DPI · сканирование"},
	"fa": {"on": "روشن کردن محافظت", "off": "خاموش کردن محافظت", "open": "باز کردن Dikec DPI", "quit": "خروج و بازگرداندن پراکسی", "tipOff": "Dikec DPI · خاموش", "tipOn": "Dikec DPI · فعال · %s", "tipScan": "Dikec DPI · در حال اسکن"},
	"ar": {"on": "تشغيل الحماية", "off": "إيقاف الحماية", "open": "افتح Dikec DPI", "quit": "خروج واستعادة الوكيل", "tipOff": "Dikec DPI · متوقف", "tipOn": "Dikec DPI · نشط · %s", "tipScan": "Dikec DPI · جارٍ الفحص"},
	"zh": {"on": "开启保护", "off": "关闭保护", "open": "打开 Dikec DPI", "quit": "退出并恢复代理", "tipOff": "Dikec DPI · 已关闭", "tipOn": "Dikec DPI · 运行中 · %s", "tipScan": "Dikec DPI · 扫描中"},
	"id": {"on": "Nyalakan perlindungan", "off": "Matikan perlindungan", "open": "Buka Dikec DPI", "quit": "Keluar dan kembalikan proxy", "tipOff": "Dikec DPI · mati", "tipOn": "Dikec DPI · aktif · %s", "tipScan": "Dikec DPI · memindai"},
	"es": {"on": "Encender la protección", "off": "Apagar la protección", "open": "Abrir Dikec DPI", "quit": "Salir y restaurar el proxy", "tipOff": "Dikec DPI · apagado", "tipOn": "Dikec DPI · activo · %s", "tipScan": "Dikec DPI · analizando"},
}

func nt(lang, key string) string {
	if m, ok := nativeText[lang]; ok {
		if s, ok := m[key]; ok {
			return s
		}
	}
	return nativeText["en"][key]
}
