#!/usr/bin/env python3
"""Generate Android strings.xml for all languages from the desktop catalog.

The desktop app's gui/frontend/i18n.js is the single source of truth; this
turns its keys into Android resource names (nav.panel -> nav_panel) and its
{name} placeholders into positional %1$s arguments, ordered as in English.
Android-only strings live in ANDROID below. Run from anywhere:

    python3 android/tools/gen_strings.py
"""
import os
import re
from xml.sax.saxutils import escape

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
SRC = os.path.join(ROOT, "gui", "frontend", "i18n.js")
RES = os.path.join(ROOT, "android", "app", "src", "main", "res")

LOCALE_DIRS = {
    "en": "values", "tr": "values-tr", "ru": "values-ru", "fa": "values-fa",
    "ar": "values-ar", "zh": "values-zh", "id": "values-in", "es": "values-es",
}

ANDROID = {
    "en": {
        "android.vpnNote": "Android shows the protection as a VPN. Your traffic does not leave the phone through another server; only the first packet of each secure connection is reshaped.",
        "android.notifTitle": "Dikec DPI is active",
        "android.notifText": "Profile: {p}",
        "android.notifChannel": "Protection status",
        "android.stopAction": "Stop",
        "android.consentDenied": "Android did not allow the VPN connection.",
        "android.otherVpn": "Another VPN app was active; Android lets only one run at a time.",
        "android.language": "The app follows your phone's language.",
        "android.openReleases": "Open release page",
    },
    "tr": {
        "android.vpnNote": "Android korumayı bir VPN olarak gösterir. Trafiğiniz başka bir sunucuya gitmez; yalnızca her güvenli bağlantının ilk paketi yeniden biçimlendirilir.",
        "android.notifTitle": "Dikec DPI aktif",
        "android.notifText": "Profil: {p}",
        "android.notifChannel": "Koruma durumu",
        "android.stopAction": "Durdur",
        "android.consentDenied": "Android VPN bağlantısına izin vermedi.",
        "android.otherVpn": "Başka bir VPN uygulaması açıktı; Android aynı anda yalnızca birine izin verir.",
        "android.language": "Uygulama telefonunuzun dilini kullanır.",
        "android.openReleases": "Sürüm sayfasını aç",
    },
    "ru": {
        "android.vpnNote": "Android показывает защиту как VPN. Трафик не уходит через сторонний сервер; изменяется только первый пакет каждого защищённого соединения.",
        "android.notifTitle": "Dikec DPI активен",
        "android.notifText": "Профиль: {p}",
        "android.notifChannel": "Состояние защиты",
        "android.stopAction": "Остановить",
        "android.consentDenied": "Android не разрешил VPN-подключение.",
        "android.otherVpn": "Было активно другое VPN-приложение; Android допускает только одно.",
        "android.language": "Приложение использует язык телефона.",
        "android.openReleases": "Открыть страницу релиза",
    },
    "fa": {
        "android.vpnNote": "اندروید محافظت را به‌صورت VPN نشان می‌دهد. ترافیک شما از سرور دیگری عبور نمی‌کند؛ فقط اولین بستهٔ هر اتصال امن تغییر شکل می‌دهد.",
        "android.notifTitle": "Dikec DPI فعال است",
        "android.notifText": "پروفایل: {p}",
        "android.notifChannel": "وضعیت محافظت",
        "android.stopAction": "توقف",
        "android.consentDenied": "اندروید اجازهٔ اتصال VPN را نداد.",
        "android.otherVpn": "برنامهٔ VPN دیگری فعال بود؛ اندروید فقط یکی را مجاز می‌داند.",
        "android.language": "برنامه از زبان گوشی شما استفاده می‌کند.",
        "android.openReleases": "باز کردن صفحهٔ انتشار",
    },
    "ar": {
        "android.vpnNote": "يعرض أندرويد الحماية كاتصال VPN. لا تمر حركتك عبر خادم آخر؛ يُعاد تشكيل الحزمة الأولى فقط من كل اتصال آمن.",
        "android.notifTitle": "Dikec DPI نشط",
        "android.notifText": "الملف: {p}",
        "android.notifChannel": "حالة الحماية",
        "android.stopAction": "إيقاف",
        "android.consentDenied": "لم يسمح أندرويد باتصال VPN.",
        "android.otherVpn": "كان تطبيق VPN آخر نشطًا؛ يسمح أندرويد بواحد فقط.",
        "android.language": "يستخدم التطبيق لغة هاتفك.",
        "android.openReleases": "افتح صفحة الإصدار",
    },
    "zh": {
        "android.vpnNote": "Android 会把保护显示为 VPN。您的流量不会经过其他服务器；只会改写每个安全连接的第一个数据包。",
        "android.notifTitle": "Dikec DPI 运行中",
        "android.notifText": "方案：{p}",
        "android.notifChannel": "保护状态",
        "android.stopAction": "停止",
        "android.consentDenied": "Android 未允许 VPN 连接。",
        "android.otherVpn": "另一个 VPN 应用正在运行；Android 同一时间只允许一个。",
        "android.language": "应用跟随手机语言。",
        "android.openReleases": "打开发布页面",
    },
    "id": {
        "android.vpnNote": "Android menampilkan perlindungan sebagai VPN. Lalu lintas Anda tidak lewat server lain; hanya paket pertama tiap koneksi aman yang dibentuk ulang.",
        "android.notifTitle": "Dikec DPI aktif",
        "android.notifText": "Profil: {p}",
        "android.notifChannel": "Status perlindungan",
        "android.stopAction": "Hentikan",
        "android.consentDenied": "Android tidak mengizinkan koneksi VPN.",
        "android.otherVpn": "Aplikasi VPN lain sedang aktif; Android hanya mengizinkan satu.",
        "android.language": "Aplikasi mengikuti bahasa ponsel Anda.",
        "android.openReleases": "Buka halaman rilis",
    },
    "es": {
        "android.vpnNote": "Android muestra la protección como una VPN. Tu tráfico no pasa por otro servidor; solo se reformatea el primer paquete de cada conexión segura.",
        "android.notifTitle": "Dikec DPI está activo",
        "android.notifText": "Perfil: {p}",
        "android.notifChannel": "Estado de la protección",
        "android.stopAction": "Detener",
        "android.consentDenied": "Android no permitió la conexión VPN.",
        "android.otherVpn": "Había otra app de VPN activa; Android solo permite una a la vez.",
        "android.language": "La app usa el idioma del teléfono.",
        "android.openReleases": "Abrir la página de la versión",
    },
}

TR_FOLD = str.maketrans({"ü": "u", "ğ": "g", "ı": "i", "ö": "o", "ş": "s", "ç": "c",
                         "Ü": "u", "Ğ": "g", "İ": "i", "Ö": "o", "Ş": "s", "Ç": "c"})


def res_name(key):
    k = key.translate(TR_FOLD).lower()
    return re.sub(r"[^a-z0-9]+", "_", k).strip("_")


def parse_catalog(text):
    langs = {}
    for m in re.finditer(r"D\.(\w+) = \{(.*?)\n\};", text, re.S):
        pairs = re.findall(r'"([^"\\]+)":\s*"((?:[^"\\]|\\.)*)"', m.group(2))
        langs[m.group(1)] = {k: bytes(v, "utf-8").decode("unicode_escape").encode("latin-1").decode("utf-8")
                             if "\\u" in v else v.replace('\\"', '"') for k, v in pairs}
    return langs


def android_string(value, order):
    names = re.findall(r"\{(\w+)\}", value)
    if not names:
        s = escape(value)
    else:
        s = escape(value.replace("%", "%%"))
        for n in names:
            idx = order.index(n) + 1 if n in order else 1
            s = s.replace("{%s}" % n, "%%%d$s" % idx)
    s = s.replace("\\", "\\\\").replace("'", "\\'").replace('"', '\\"')
    if s.startswith(("@", "?")):
        s = "\\" + s
    return s


def main():
    catalog = parse_catalog(open(SRC, encoding="utf-8").read())
    for lang in LOCALE_DIRS:
        catalog.setdefault(lang, {}).update(ANDROID.get(lang, {}))
    english = catalog["en"]
    orders = {k: re.findall(r"\{(\w+)\}", v) for k, v in english.items()}
    for lang, folder in LOCALE_DIRS.items():
        entries = catalog.get(lang, {})
        lines = ['<?xml version="1.0" encoding="utf-8"?>',
                 "<!-- Generated by android/tools/gen_strings.py from gui/frontend/i18n.js. Do not edit. -->",
                 "<resources>"]
        seen = set()
        for key in sorted(entries):
            name = res_name(key)
            if not name or name in seen:
                continue
            seen.add(name)
            lines.append('    <string name="%s">%s</string>' % (name, android_string(entries[key], orders.get(key, []))))
        lines.append("</resources>\n")
        os.makedirs(os.path.join(RES, folder), exist_ok=True)
        with open(os.path.join(RES, folder, "strings.xml"), "w", encoding="utf-8") as f:
            f.write("\n".join(lines))
        print(f"{folder:12} {len(seen)} strings")


if __name__ == "__main__":
    main()
