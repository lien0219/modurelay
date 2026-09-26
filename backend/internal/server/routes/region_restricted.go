package routes

import (
	"html/template"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

type regionRestrictedPageCopy struct {
	Lang             string
	HTMLLang         string
	Brand            string
	Badge            string
	Title            string
	Subtitle         string
	DescriptionLine1 string
	DescriptionLine2 string
	HowTitle         string
	Step1Title       string
	Step1Line1       string
	Step1Line2       string
	Step2Title       string
	Step2Line1       string
	Step2Line2       string
	Step3Title       string
	Step3Line1       string
	Step3Line2       string
	TipTitle         string
	TipText          string
	ChineseLabel     string
	EnglishLabel     string
	LanguageAria     string
}

type regionRestrictedPageData struct {
	regionRestrictedPageCopy
	Nonce string
}

var regionRestrictedCopies = map[string]regionRestrictedPageCopy{
	"zh": {
		Lang:             "zh",
		HTMLLang:         "zh-CN",
		Brand:            "AI 智能服务",
		Badge:            "访问受限",
		Title:            "暂不对中国大陆地区开放",
		Subtitle:         "当前服务暂未向中国大陆地区提供访问。",
		DescriptionLine1: "如需访问，请在设备上使用 VPN / 代理网络。",
		DescriptionLine2: "为获得更稳定、更流畅的使用体验，建议开启全局代理模式后重新访问。",
		HowTitle:         "如何正常访问？",
		Step1Title:       "1. 开启 VPN / 代理",
		Step1Line1:       "建议使用稳定的海外网络",
		Step1Line2:       "并开启全局代理模式",
		Step2Title:       "2. 切换网络环境",
		Step2Line1:       "确保设备的网络出口",
		Step2Line2:       "已不在中国大陆地区",
		Step3Title:       "3. 重新访问",
		Step3Line1:       "完成网络切换后",
		Step3Line2:       "请刷新当前页面",
		TipTitle:         "温馨提示",
		TipText:          "使用全局代理模式可以避免部分资源加载异常，获得更稳定、更完整的功能体验。",
		ChineseLabel:     "中文",
		EnglishLabel:     "English",
		LanguageAria:     "切换语言",
	},
	"en": {
		Lang:             "en",
		HTMLLang:         "en",
		Brand:            "AI Service",
		Badge:            "Access Restricted",
		Title:            "Temporarily unavailable in Mainland China",
		Subtitle:         "This service is currently unavailable to users in Mainland China.",
		DescriptionLine1: "To access, please use a VPN / proxy network on your device.",
		DescriptionLine2: "For a more stable and smoother experience, enable Global Proxy mode before visiting again.",
		HowTitle:         "How to access normally?",
		Step1Title:       "1. Enable VPN / Proxy",
		Step1Line1:       "Use a stable overseas network",
		Step1Line2:       "and enable Global Proxy mode",
		Step2Title:       "2. Switch network",
		Step2Line1:       "Make sure your device traffic",
		Step2Line2:       "exits outside Mainland China",
		Step3Title:       "3. Visit again",
		Step3Line1:       "After switching networks,",
		Step3Line2:       "refresh the current page",
		TipTitle:         "Tip",
		TipText:          "Global Proxy mode can prevent partial asset-loading issues and provide a more stable, complete experience.",
		ChineseLabel:     "中文",
		EnglishLabel:     "English",
		LanguageAria:     "Switch language",
	},
}

func registerRegionRestrictedRoutes(r *gin.Engine) {
	r.GET("/region-restricted", serveRegionRestrictedPage)
	r.HEAD("/region-restricted", serveRegionRestrictedPage)
}

// serveRegionRestrictedPage is intentionally self-contained: the HTML uses no
// external CSS, JavaScript, fonts, images, or APIs. A Cloudflare country rule
// therefore only needs to exclude this single path to avoid a redirect loop.
func serveRegionRestrictedPage(c *gin.Context) {
	lang := normalizeRegionRestrictedLanguage(c.Query("lang"))
	if c.Query("lang") == "" {
		lang = languageFromAcceptHeader(c.GetHeader("Accept-Language"))
	}
	copy := regionRestrictedCopies[lang]

	c.Header("Cache-Control", "no-store, max-age=0")
	c.Header("Pragma", "no-cache")
	c.Header("X-Robots-Tag", "noindex, nofollow, noarchive")
	c.Header("Content-Language", copy.HTMLLang)
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("Vary", "Accept-Language")

	if c.Request.Method == http.MethodHead {
		c.Status(http.StatusOK)
		return
	}

	c.Status(http.StatusOK)
	if err := regionRestrictedTemplate.Execute(c.Writer, regionRestrictedPageData{
		regionRestrictedPageCopy: copy,
		Nonce:                    middleware.GetNonceFromContext(c),
	}); err != nil {
		_ = c.Error(err)
	}
}

func normalizeRegionRestrictedLanguage(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "en", "en-us", "en-gb":
		return "en"
	default:
		return "zh"
	}
}

func languageFromAcceptHeader(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if strings.HasPrefix(raw, "en") {
		return "en"
	}
	return "zh"
}

var regionRestrictedTemplate = template.Must(template.New("region-restricted").Parse(`<!doctype html>
<html lang="{{.HTMLLang}}">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
  <meta name="color-scheme" content="light">
  <meta name="robots" content="noindex,nofollow,noarchive">
  <title>{{.Title}}</title>
  <style nonce="{{.Nonce}}">
    :root {
      color-scheme: light;
      --ink: #0c224d;
      --muted: #627797;
      --muted-2: #7e91ad;
      --line: #e7edf7;
      --blue: #4d78ff;
      --blue-deep: #345bf0;
      --cyan: #14c7b1;
      --danger: #e12222;
      --danger-soft: #fff0ef;
      --surface: rgba(255,255,255,.93);
      --panel: rgba(247,250,255,.9);
    }
    * { box-sizing: border-box; }
    html, body { margin: 0; min-height: 100%; }
    body {
      min-height: 100vh;
      min-height: 100dvh;
      padding: 38px;
      color: var(--ink);
      background:
        radial-gradient(circle at 16% 10%, rgba(124,151,255,.13), transparent 33%),
        radial-gradient(circle at 86% 8%, rgba(93,205,255,.11), transparent 29%),
        linear-gradient(135deg, #f4f6ff 0%, #f8fbff 50%, #f1f6ff 100%);
      font-family: "Microsoft YaHei", "PingFang SC", "Hiragino Sans GB", "Noto Sans CJK SC",
        Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      -webkit-font-smoothing: antialiased;
      text-rendering: optimizeLegibility;
    }
    .page {
      width: min(1458px, 100%);
      min-height: calc(100vh - 76px);
      min-height: calc(100dvh - 76px);
      margin: 0 auto;
      overflow: hidden;
      border: 1px solid rgba(230,236,248,.82);
      border-radius: 28px;
      background:
        radial-gradient(circle at 24% 34%, rgba(255,237,239,.42), transparent 24%),
        radial-gradient(circle at 75% 78%, rgba(237,246,255,.72), transparent 30%),
        var(--surface);
      box-shadow: 0 26px 80px rgba(50,74,128,.08);
    }
    .header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 28px;
      padding: 42px 70px 0;
    }
    .brand {
      display: inline-flex;
      align-items: center;
      gap: 20px;
      color: #0a2453;
      font-size: 30px;
      font-weight: 800;
      letter-spacing: -.03em;
      white-space: nowrap;
    }
    .brand-mark {
      width: 52px;
      height: 52px;
      flex: 0 0 52px;
      filter: drop-shadow(0 8px 13px rgba(64,105,255,.15));
    }
    .language {
      display: inline-flex;
      align-items: center;
      gap: 16px;
      color: #566d91;
      font-size: 19px;
      white-space: nowrap;
    }
    .language svg { width: 25px; height: 25px; color: #425a7f; }
    .language a {
      color: #536887;
      text-decoration: none;
      transition: color .16s ease;
    }
    .language a:hover { color: #1d3d74; }
    .language a.active { color: #0f2857; font-weight: 800; }
    .language .divider { width: 1px; height: 25px; background: #d4dce9; }

    .main {
      display: grid;
      grid-template-columns: minmax(450px, 42%) minmax(0, 58%);
      align-items: center;
      gap: 42px;
      padding: 50px 70px 34px;
    }
    .illustration {
      position: relative;
      min-height: 585px;
      display: flex;
      align-items: center;
      justify-content: center;
    }
    .illustration svg {
      display: block;
      width: min(590px, 100%);
      height: auto;
      overflow: visible;
    }
    .content {
      min-width: 0;
      padding: 6px 0 0;
    }
    .badge {
      display: inline-flex;
      align-items: center;
      gap: 11px;
      min-height: 52px;
      padding: 0 22px;
      border-radius: 999px;
      color: #c81f1f;
      background: linear-gradient(180deg, #ffe7e5, #fff0ef);
      font-size: 20px;
      font-weight: 800;
    }
    .badge-icon {
      display: grid;
      width: 29px;
      height: 29px;
      place-items: center;
      border-radius: 50%;
      color: white;
      background: #d91f1f;
      font-weight: 900;
      line-height: 1;
    }
    h1 {
      margin: 28px 0 22px;
      color: #0b234f;
      font-size: clamp(42px, 4vw, 58px);
      font-weight: 900;
      line-height: 1.13;
      letter-spacing: -.045em;
    }
    .subtitle {
      margin: 0 0 22px;
      color: #607594;
      font-size: 27px;
      line-height: 1.5;
      font-weight: 500;
    }
    .description {
      margin: 0;
      color: #5a7090;
      font-size: 21px;
      line-height: 1.75;
    }
    .steps {
      margin-top: 34px;
      padding: 24px 30px 28px;
      border: 1px solid #eef2f8;
      border-radius: 18px;
      background:
        linear-gradient(180deg, rgba(247,250,255,.94), rgba(251,253,255,.92));
      box-shadow: inset 0 1px 0 rgba(255,255,255,.9);
    }
    .steps h2 {
      margin: 0 0 24px;
      color: #112753;
      font-size: 22px;
      line-height: 1.4;
    }
    .step-grid {
      display: grid;
      grid-template-columns: 1fr 44px 1fr 44px 1fr;
      align-items: start;
    }
    .step {
      min-width: 0;
      text-align: center;
    }
    .step-icon {
      display: grid;
      width: 66px;
      height: 66px;
      margin: 0 auto 14px;
      place-items: center;
      filter: drop-shadow(0 8px 14px rgba(54,106,255,.12));
    }
    .step-icon svg { width: 66px; height: 66px; }
    .step-title {
      color: #112650;
      font-size: 18px;
      line-height: 1.45;
      font-weight: 800;
    }
    .step-copy {
      margin-top: 8px;
      color: #657a98;
      font-size: 15px;
      line-height: 1.65;
    }
    .arrow {
      display: flex;
      align-items: center;
      justify-content: center;
      height: 66px;
      color: #8aa0c7;
    }
    .arrow svg { width: 24px; height: 24px; }

    .tip {
      display: flex;
      align-items: center;
      gap: 24px;
      margin: 0 50px 39px;
      padding: 19px 40px;
      border: 1px solid #eaf2fb;
      border-radius: 18px;
      background: linear-gradient(90deg, rgba(240,247,255,.95), rgba(246,251,255,.9));
    }
    .tip-icon {
      display: grid;
      width: 52px;
      height: 52px;
      flex: 0 0 52px;
      place-items: center;
      border-radius: 50%;
      color: #4b6e9e;
      background: rgba(224,239,255,.8);
    }
    .tip-icon svg { width: 26px; height: 26px; }
    .tip-title {
      margin: 0 0 3px;
      color: #10254d;
      font-size: 19px;
      font-weight: 800;
    }
    .tip-text {
      margin: 0;
      color: #627797;
      font-size: 18px;
      line-height: 1.6;
    }

    @media (max-width: 1180px) {
      body { padding: 22px; }
      .page { min-height: calc(100dvh - 44px); }
      .header { padding: 34px 42px 0; }
      .main {
        grid-template-columns: 41% minmax(0, 59%);
        gap: 22px;
        padding: 36px 42px 28px;
      }
      .illustration { min-height: 500px; }
      h1 { font-size: clamp(38px, 4.6vw, 52px); }
      .subtitle { font-size: 23px; }
      .description { font-size: 18px; }
      .steps { padding: 22px 20px 24px; }
      .step-grid { grid-template-columns: 1fr 30px 1fr 30px 1fr; }
      .tip { margin: 0 32px 30px; }
    }

    @media (max-width: 900px) {
      body { padding: 14px; }
      .page { min-height: calc(100dvh - 28px); border-radius: 22px; }
      .header { padding: 24px 24px 0; }
      .brand { gap: 12px; font-size: 22px; }
      .brand-mark { width: 42px; height: 42px; flex-basis: 42px; }
      .language { gap: 10px; font-size: 15px; }
      .language svg { width: 20px; height: 20px; }
      .language .divider { height: 18px; }
      .main {
        grid-template-columns: 1fr;
        gap: 8px;
        padding: 34px 24px 24px;
      }
      .content { order: 1; }
      .illustration {
        order: 2;
        min-height: 0;
        margin-top: 18px;
      }
      .illustration svg { width: min(500px, 92%); }
      .badge { min-height: 44px; padding: 0 16px; font-size: 16px; }
      .badge-icon { width: 24px; height: 24px; }
      h1 { margin: 20px 0 14px; font-size: clamp(34px, 8.2vw, 48px); }
      .subtitle { margin-bottom: 16px; font-size: 20px; }
      .description { font-size: 16px; }
      .steps { margin-top: 24px; }
      .tip { margin: 0 24px 24px; padding: 17px 20px; }
      .tip-text { font-size: 15px; }
    }

    @media (max-width: 640px) {
      .header { align-items: flex-start; }
      .brand { font-size: 19px; }
      .language { padding-top: 7px; }
      .main { padding-top: 28px; }
      .step-grid { grid-template-columns: 1fr; gap: 20px; }
      .arrow { height: 18px; transform: rotate(90deg); }
      .steps { padding: 20px 18px 22px; }
      .tip { align-items: flex-start; }
    }

    @media (prefers-reduced-motion: reduce) {
      *, *::before, *::after { scroll-behavior: auto !important; transition: none !important; }
    }
  </style>
</head>
<body>
  <main class="page">
    <header class="header">
      <div class="brand" aria-label="{{.Brand}}">
        <svg class="brand-mark" viewBox="0 0 64 64" aria-hidden="true">
          <defs>
            <linearGradient id="brandBlue" x1="8" y1="8" x2="56" y2="56">
              <stop offset="0" stop-color="#3768ff"/>
              <stop offset=".58" stop-color="#517eff"/>
              <stop offset="1" stop-color="#7aa6ff"/>
            </linearGradient>
          </defs>
          <path d="M31.9 8.2 13 42.2c-2.4 4.3.7 9.6 5.6 9.6h11.5" fill="none" stroke="url(#brandBlue)" stroke-width="8.5" stroke-linecap="round" stroke-linejoin="round"/>
          <path d="m32 8.2 19.2 34.4c2.3 4.2-.7 9.2-5.5 9.2H34.4" fill="none" stroke="url(#brandBlue)" stroke-width="8.5" stroke-linecap="round" stroke-linejoin="round"/>
          <path d="M17.8 42.5h28.4L32 17.5" fill="none" stroke="url(#brandBlue)" stroke-width="8.5" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
        <span>{{.Brand}}</span>
      </div>

      <nav class="language" aria-label="{{.LanguageAria}}">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
          <circle cx="12" cy="12" r="9"/>
          <path d="M3.7 9h16.6M3.7 15h16.6M12 3c2.35 2.45 3.55 5.45 3.55 9S14.35 18.55 12 21M12 3C9.65 5.45 8.45 8.45 8.45 12S9.65 18.55 12 21"/>
        </svg>
        <a href="/region-restricted?lang=zh" class="{{if eq .Lang "zh"}}active{{end}}">{{.ChineseLabel}}</a>
        <span class="divider" aria-hidden="true"></span>
        <a href="/region-restricted?lang=en" class="{{if eq .Lang "en"}}active{{end}}">{{.EnglishLabel}}</a>
      </nav>
    </header>

    <section class="main">
      <div class="illustration" aria-hidden="true">
        <svg viewBox="0 0 600 580" role="img">
          <defs>
            <linearGradient id="mapFill" x1="120" y1="90" x2="430" y2="330">
              <stop offset="0" stop-color="#ffb9b5"/>
              <stop offset=".55" stop-color="#ffc9c6"/>
              <stop offset="1" stop-color="#ffe0dc"/>
            </linearGradient>
            <linearGradient id="browserTop" x1="0" x2="1">
              <stop offset="0" stop-color="#bfcbff"/>
              <stop offset="1" stop-color="#d7e6ff"/>
            </linearGradient>
            <linearGradient id="globeBlue" x1="0" x2="1" y1="0" y2="1">
              <stop offset="0" stop-color="#6d82ff"/>
              <stop offset="1" stop-color="#83a9ff"/>
            </linearGradient>
            <linearGradient id="shieldGreen" x1="0" x2="1" y1="0" y2="1">
              <stop offset="0" stop-color="#2cd6c6"/>
              <stop offset="1" stop-color="#07b995"/>
            </linearGradient>
            <linearGradient id="banRed" x1="0" x2="1" y1="0" y2="1">
              <stop offset="0" stop-color="#ff6d63"/>
              <stop offset="1" stop-color="#f54c4c"/>
            </linearGradient>
            <filter id="softShadow" x="-30%" y="-30%" width="160%" height="180%">
              <feDropShadow dx="0" dy="12" stdDeviation="12" flood-color="#627cc0" flood-opacity=".13"/>
            </filter>
          </defs>

          <!-- soft cloud field -->
          <g fill="#eaf2ff" opacity=".92">
            <circle cx="112" cy="395" r="68"/><circle cx="57" cy="445" r="50"/><circle cx="146" cy="460" r="62"/>
            <circle cx="431" cy="390" r="76"/><circle cx="505" cy="443" r="54"/><circle cx="407" cy="463" r="62"/>
          </g>
          <g fill="#fff" opacity=".97">
            <circle cx="65" cy="500" r="62"/><circle cx="130" cy="475" r="84"/><circle cx="210" cy="518" r="61"/>
          </g>

          <!-- stylized China silhouette -->
          <path d="M87 190 111 178 121 157 139 151 146 126 166 121 182 132 194 115 215 123
                   235 111 248 92 267 93 275 70 294 62 309 69 325 66 338 78 340 95 361 106
                   379 100 390 116 409 123 426 120 441 105 450 112 445 131 456 145 444 156
                   428 160 421 181 401 188 393 204 372 210 361 225 343 230 328 249 306 246
                   289 262 271 256 255 268 238 257 216 265 201 246 184 242 175 224 157 228
                   146 211 127 213 117 197 98 202Z"
                fill="url(#mapFill)" opacity=".95"/>

          <!-- no-entry symbol -->
          <g transform="translate(313 213)" filter="url(#softShadow)">
            <circle cx="0" cy="0" r="86" fill="rgba(255,255,255,.18)" stroke="url(#banRed)" stroke-width="18"/>
            <path d="M-58-58 58 58" stroke="url(#banRed)" stroke-width="22" stroke-linecap="round"/>
          </g>

          <!-- browser -->
          <g transform="translate(130 340)" filter="url(#softShadow)">
            <rect x="0" y="0" width="305" height="215" rx="17" fill="#fff"/>
            <path d="M0 17C0 7.6 7.6 0 17 0h271c9.4 0 17 7.6 17 17v22H0V17Z" fill="url(#browserTop)"/>
            <circle cx="24" cy="20" r="7" fill="#fff"/><circle cx="48" cy="20" r="7" fill="#fff"/><circle cx="72" cy="20" r="7" fill="#fff"/>
            <g transform="translate(94 68)" fill="none" stroke="url(#globeBlue)" stroke-width="8">
              <circle cx="57" cy="57" r="54"/>
              <path d="M4 57h106M57 3c18 16 28 35 28 54s-10 38-28 54M57 3C39 19 29 38 29 57s10 38 28 54M18 27c25 9 53 9 78 0M18 87c25-9 53-9 78 0"/>
            </g>
          </g>

          <!-- vpn shield -->
          <g transform="translate(388 397)" filter="url(#softShadow)">
            <path d="M0 12 40 0l40 12v38c0 32-21 50-40 60C21 100 0 82 0 50V12Z" fill="url(#shieldGreen)"/>
            <text x="40" y="58" text-anchor="middle" fill="#fff" font-size="22" font-weight="800"
                  font-family="Arial, sans-serif">VPN</text>
          </g>
        </svg>
      </div>

      <div class="content">
        <div class="badge">
          <span class="badge-icon">!</span>
          <span>{{.Badge}}</span>
        </div>

        <h1>{{.Title}}</h1>
        <p class="subtitle">{{.Subtitle}}</p>
        <p class="description">
          {{.DescriptionLine1}}<br>
          {{.DescriptionLine2}}
        </p>

        <section class="steps">
          <h2>{{.HowTitle}}</h2>
          <div class="step-grid">
            <article class="step">
              <div class="step-icon" aria-hidden="true">
                <svg viewBox="0 0 72 72">
                  <defs><linearGradient id="vpnStep" x1="0" x2="1" y1="0" y2="1"><stop stop-color="#6e7cff"/><stop offset="1" stop-color="#3c5ff0"/></linearGradient></defs>
                  <path d="M36 3 62 12v23c0 18-11.8 28.2-26 35C21.8 63.2 10 53 10 35V12L36 3Z" fill="url(#vpnStep)"/>
                  <text x="36" y="40" text-anchor="middle" fill="#fff" font-size="17" font-weight="800" font-family="Arial,sans-serif">VPN</text>
                </svg>
              </div>
              <div class="step-title">{{.Step1Title}}</div>
              <div class="step-copy">{{.Step1Line1}}<br>{{.Step1Line2}}</div>
            </article>

            <div class="arrow" aria-hidden="true">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.8" stroke-linecap="round" stroke-linejoin="round"><path d="m9 5 7 7-7 7"/></svg>
            </div>

            <article class="step">
              <div class="step-icon" aria-hidden="true">
                <svg viewBox="0 0 72 72">
                  <defs><linearGradient id="globeStep" x1="0" x2="1" y1="0" y2="1"><stop stop-color="#16d2bc"/><stop offset="1" stop-color="#0bab9f"/></linearGradient></defs>
                  <circle cx="36" cy="36" r="31" fill="url(#globeStep)"/>
                  <g fill="none" stroke="#fff" stroke-width="3">
                    <circle cx="36" cy="36" r="20"/>
                    <path d="M16 36h40M36 16c7 6 10 13 10 20s-3 14-10 20M36 16c-7 6-10 13-10 20s3 14 10 20"/>
                  </g>
                </svg>
              </div>
              <div class="step-title">{{.Step2Title}}</div>
              <div class="step-copy">{{.Step2Line1}}<br>{{.Step2Line2}}</div>
            </article>

            <div class="arrow" aria-hidden="true">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.8" stroke-linecap="round" stroke-linejoin="round"><path d="m9 5 7 7-7 7"/></svg>
            </div>

            <article class="step">
              <div class="step-icon" aria-hidden="true">
                <svg viewBox="0 0 72 72">
                  <defs><linearGradient id="refreshStep" x1="0" x2="1" y1="0" y2="1"><stop stop-color="#61a7ff"/><stop offset="1" stop-color="#2f6eff"/></linearGradient></defs>
                  <circle cx="36" cy="36" r="31" fill="url(#refreshStep)"/>
                  <path d="M49 29a17 17 0 1 0 2.2 16" fill="none" stroke="#fff" stroke-width="4" stroke-linecap="round"/>
                  <path d="m48 18 2 12-12-1" fill="none" stroke="#fff" stroke-width="4" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
              </div>
              <div class="step-title">{{.Step3Title}}</div>
              <div class="step-copy">{{.Step3Line1}}<br>{{.Step3Line2}}</div>
            </article>
          </div>
        </section>
      </div>
    </section>

    <section class="tip">
      <div class="tip-icon" aria-hidden="true">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
          <path d="M9 18h6M10 21h4M8.6 14.7C7 13.6 6 11.8 6 9.8A6 6 0 0 1 18 9.8c0 2-1 3.8-2.6 4.9-.9.6-1.4 1.3-1.4 2.3h-4c0-1-.5-1.7-1.4-2.3Z"/>
        </svg>
      </div>
      <div>
        <p class="tip-title">{{.TipTitle}}</p>
        <p class="tip-text">{{.TipText}}</p>
      </div>
    </section>
  </main>
</body>
</html>`))

