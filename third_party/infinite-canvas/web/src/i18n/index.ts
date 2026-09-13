import i18n from "i18next";
import { initReactI18next } from "react-i18next";

import enUS from "@/i18n/locales/en-US";
import zhCN from "@/i18n/locales/zh-CN";

export type AppLocale = "zh-CN" | "en-US";

const LOCALE_STORAGE_KEY = "infinite-canvas:locale";
const MODURELAY_LOCALE_STORAGE_KEY = "sub2api_locale";

function initialLocale(): AppLocale {
    const modurelayLocale = localStorage.getItem(MODURELAY_LOCALE_STORAGE_KEY);
    if (modurelayLocale === "zh" || modurelayLocale === "en") return modurelayLocale === "zh" ? "zh-CN" : "en-US";
    return (localStorage.getItem(LOCALE_STORAGE_KEY) as AppLocale) || "zh-CN";
}

i18n.use(initReactI18next).init({
    resources: {
        "zh-CN": { translation: zhCN },
        "en-US": { translation: enUS },
    },
    lng: initialLocale(),
    fallbackLng: "zh-CN",
    supportedLngs: ["zh-CN", "en-US"],
    initAsync: false,
    interpolation: { escapeValue: false },
    react: { useSuspense: false },
});

export function changeAppLocale(locale: AppLocale) {
    localStorage.setItem(LOCALE_STORAGE_KEY, locale);
    localStorage.setItem(MODURELAY_LOCALE_STORAGE_KEY, locale === "zh-CN" ? "zh" : "en");
    return i18n.changeLanguage(locale);
}

export default i18n;
