package main

// Native menu and tooltip strings. The web UI has its own, larger catalog
// in frontend/i18n.js; these few cannot come from there because the menu
// bar menu is drawn by macOS.
var nativeText = map[string]map[string]string{
	"en": {
		"qTitle":  "Quit Dikec DPI?",
		"qMsg":    "%d connections are going through Dikec DPI right now. Some apps (for example Claude Desktop sessions) remember the proxy address and lose the internet when Dikec DPI quits until you restart them. Keeping Dikec DPI running in the menu bar avoids that.",
		"qQuit":   "Quit anyway",
		"qKeep":   "Keep running in the menu bar",
		"on":      "Turn protection on",
		"off":     "Turn protection off",
		"open":    "Open Dikec DPI",
		"quit":    "Quit and restore proxy",
		"tipOff":  "Dikec DPI · off",
		"tipOn":   "Dikec DPI · active · %s",
		"tipScan": "Dikec DPI · scanning",
	},
	"tr": {
		"qTitle":  "Dikec DPI kapatılsın mı?",
		"qMsg":    "Şu an %d bağlantı Dikec DPI üzerinden geçiyor. Bazı uygulamalar (ör. Claude Desktop oturumları) proxy adresini hatırlar ve Dikec DPI kapanınca, yeniden başlatılana kadar internete çıkamaz. Dikec DPI’ı menü çubuğunda açık bırakmak bunu önler.",
		"qQuit":   "Yine de kapat",
		"qKeep":   "Menü çubuğunda açık kalsın",
		"on":      "Korumayı aç",
		"off":     "Korumayı kapat",
		"open":    "Dikec DPI’ı aç",
		"quit":    "Çıkış ve proxy’yi geri al",
		"tipOff":  "Dikec DPI · kapalı",
		"tipOn":   "Dikec DPI · aktif · %s",
		"tipScan": "Dikec DPI · taranıyor",
	},
	"ru": {
		"qTitle":  "Закрыть Dikec DPI?",
		"qMsg":    "Сейчас через Dikec DPI идёт соединений: %d. Некоторые приложения (например, сеансы Claude Desktop) запоминают адрес прокси и после закрытия Dikec DPI теряют интернет, пока их не перезапустить. Чтобы этого избежать, оставьте Dikec DPI в строке меню.",
		"qQuit":   "Всё равно закрыть",
		"qKeep":   "Оставить в строке меню",
		"on":      "Включить защиту",
		"off":     "Выключить защиту",
		"open":    "Открыть Dikec DPI",
		"quit":    "Выйти и восстановить прокси",
		"tipOff":  "Dikec DPI · выключено",
		"tipOn":   "Dikec DPI · активно · %s",
		"tipScan": "Dikec DPI · сканирование",
	},
	"fa": {
		"qTitle":  "Dikec DPI بسته شود؟",
		"qMsg":    "اکنون %d اتصال از Dikec DPI عبور می‌کند. برخی برنامه‌ها (مثل نشست‌های Claude Desktop) نشانی پراکسی را به خاطر می‌سپارند و با بسته شدن Dikec DPI تا راه‌اندازی دوباره به اینترنت دسترسی ندارند. باز گذاشتن Dikec DPI در نوار منو از این جلوگیری می‌کند.",
		"qQuit":   "در هر صورت ببند",
		"qKeep":   "در نوار منو بماند",
		"on":      "روشن کردن محافظت",
		"off":     "خاموش کردن محافظت",
		"open":    "باز کردن Dikec DPI",
		"quit":    "خروج و بازگرداندن پراکسی",
		"tipOff":  "Dikec DPI · خاموش",
		"tipOn":   "Dikec DPI · فعال · %s",
		"tipScan": "Dikec DPI · در حال اسکن",
	},
	"ar": {
		"qTitle":  "إغلاق Dikec DPI؟",
		"qMsg":    "يمر الآن %d اتصال عبر Dikec DPI. بعض التطبيقات (مثل جلسات Claude Desktop) تحفظ عنوان الوكيل وتفقد الإنترنت عند إغلاق Dikec DPI حتى تعيد تشغيلها. إبقاء Dikec DPI في شريط القوائم يمنع ذلك.",
		"qQuit":   "أغلق على أي حال",
		"qKeep":   "أبقه في شريط القوائم",
		"on":      "تشغيل الحماية",
		"off":     "إيقاف الحماية",
		"open":    "افتح Dikec DPI",
		"quit":    "خروج واستعادة الوكيل",
		"tipOff":  "Dikec DPI · متوقف",
		"tipOn":   "Dikec DPI · نشط · %s",
		"tipScan": "Dikec DPI · جارٍ الفحص",
	},
	"zh": {
		"qTitle":  "退出 Dikec DPI？",
		"qMsg":    "当前有 %d 个连接经过 Dikec DPI。有些应用（例如 Claude Desktop 会话）会记住代理地址，Dikec DPI 退出后将无法联网，直到重新启动这些应用。让 Dikec DPI 留在菜单栏中运行可避免这种情况。",
		"qQuit":   "仍然退出",
		"qKeep":   "留在菜单栏中运行",
		"on":      "开启保护",
		"off":     "关闭保护",
		"open":    "打开 Dikec DPI",
		"quit":    "退出并恢复代理",
		"tipOff":  "Dikec DPI · 已关闭",
		"tipOn":   "Dikec DPI · 运行中 · %s",
		"tipScan": "Dikec DPI · 扫描中",
	},
	"id": {
		"qTitle":  "Tutup Dikec DPI?",
		"qMsg":    "Saat ini %d koneksi melewati Dikec DPI. Beberapa aplikasi (misalnya sesi Claude Desktop) mengingat alamat proxy dan kehilangan internet saat Dikec DPI ditutup sampai dimulai ulang. Biarkan Dikec DPI berjalan di bilah menu untuk menghindarinya.",
		"qQuit":   "Tetap tutup",
		"qKeep":   "Tetap jalan di bilah menu",
		"on":      "Nyalakan perlindungan",
		"off":     "Matikan perlindungan",
		"open":    "Buka Dikec DPI",
		"quit":    "Keluar dan kembalikan proxy",
		"tipOff":  "Dikec DPI · mati",
		"tipOn":   "Dikec DPI · aktif · %s",
		"tipScan": "Dikec DPI · memindai",
	},
	"es": {
		"qTitle":  "¿Salir de Dikec DPI?",
		"qMsg":    "Ahora mismo pasan %d conexiones por Dikec DPI. Algunas apps (por ejemplo, las sesiones de Claude Desktop) recuerdan la dirección del proxy y se quedan sin internet al cerrar Dikec DPI hasta que las reinicies. Dejar Dikec DPI en la barra de menús lo evita.",
		"qQuit":   "Salir de todos modos",
		"qKeep":   "Seguir en la barra de menús",
		"on":      "Encender la protección",
		"off":     "Apagar la protección",
		"open":    "Abrir Dikec DPI",
		"quit":    "Salir y restaurar el proxy",
		"tipOff":  "Dikec DPI · apagado",
		"tipOn":   "Dikec DPI · activo · %s",
		"tipScan": "Dikec DPI · analizando",
	},
}

func nt(lang, key string) string {
	if m, ok := nativeText[lang]; ok {
		if s, ok := m[key]; ok {
			return s
		}
	}
	return nativeText["en"][key]
}
