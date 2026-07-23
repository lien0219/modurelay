# ModuRelay

**ModuRelay AI Gateway**

Connect once. Route any model.

[English](README.md) | [简体中文](README_CN.md) | [日本語](README_JA.md)

---

[![License: LGPL v3](https://img.shields.io/badge/License-LGPL%20v3-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26.5-00ADD8.svg)](https://golang.org/)
[![Vue](https://img.shields.io/badge/Vue-3.4+-4FC08D.svg)](https://vuejs.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-18-336791.svg)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Redis-8-DC382D.svg)](https://redis.io/)
[![Docker](https://img.shields.io/badge/Docker-Build-2496ED.svg)](https://www.docker.com/)

> **Translation note:** 本文は英語版 README と同じ構成です。一部の表現は *translation pending* として、英語版を正とします。

## Overview

ModuRelay は、複数プロバイダ対応のルーティング、アカウントプール、利用量計測、API 管理を行うオープンソースの AI API ゲートウェイです。

できること：

- 複数の上流 AI プロバイダを一つのゲートウェイに集約
- アカウントプール管理と API Key 配布
- スケジューリングと sticky session によるルーティング
- Token 利用量の計測と同時実行 / レート制限
- 管理コンソールによる運用
- 設定時に内蔵決済とユーザーセルフチャージをサポート
- composite groups で要求モデルを具体的な上流プロバイダへ解決
- チケット管理などの外部システムを iframe で管理画面に埋め込み

ModuRelay は、OpenAI 互換 API やプロバイダ固有 API を使う自作 Agent、IDE プラグイン、その他 AI ツールのモデル接続層としても利用できます。

> Langflow や ComfyUI 向け連携は計画段階です。[Roadmap](#roadmap) を参照してください。

## Important notice

デプロイまたは利用前に以下を確認してください：

- 本ソフトウェアを上流プロバイダと併用すると、各プロバイダの利用規約に抵触する可能性があります。各自で確認してください。
- 所在国・地域の法令に従って利用してください。
- 設定したアカウント、API Key、認証情報の管理責任は利用者にあります。
- 上流アカウントの安定性とプロバイダの可用性は保証されません。
- 本プロジェクトは AI プロバイダからの公式認可を提供しません。
- デプロイと運用リスクは利用者が負担します。
- 違法用途には使用しないでください。
- ModuRelay は、本プロジェクト名義で第三者が商用運用を行うことを許可していません。

## Project relationship

ModuRelay は [Sub2API](https://github.com/Wei-Shaw/sub2api) を基にした、独立して保守される派生オープンソースプロジェクトです。

- ModuRelay は Sub2API 公式プロジェクトではありません
- 上流メンテナによる公式 endorsement はありません
- 上流リポジトリ: [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api)
- ライセンスと著作権: [LICENSE](LICENSE) および [NOTICE.md](NOTICE.md)

## Core features

| Feature | Description |
| --- | --- |
| Multi-account management | 対応プロバイダの上流アカウントを管理 |
| Credential types | プロバイダが対応する場合、OAuth / API Key などを利用 |
| API Key management | エンドユーザー向け Key の発行・更新・制御 |
| Smart scheduling | 負荷を考慮したアカウント選択と sticky session |
| Usage metering | Token とリクエスト統計を記録 |
| Precise billing | Token レベルの利用追跡、倍率、残高などの課金設定 |
| Concurrency control | ユーザー / グループ / アカウント単位の同時実行制限 |
| RPM / rate limits | RPM などのレート制限 |
| Admin console | Vue ベースの管理 UI |
| Payments | 設定時、EasyPay、Alipay、WeChat Pay、Stripe などの内蔵決済を利用可能 |
| Composite groups | 複数プロバイダグループで要求モデルを具体的なプロバイダへ解決（[operator guide](docs/COMPOSITE_GROUPS.md)） |
| External system integration | チケット管理などの外部システムを iframe で管理画面に埋め込み |
| Docker deployment | Docker Compose でソースから構築可能 |

## Architecture

```mermaid
flowchart LR
  clients[Clients / Agents / IDE plugins]
  gateway[ModuRelay Gateway]
  control[Auth / Routing / Rate limit / Metering]
  providers[OpenAI / Claude / Gemini / Grok / Other providers]
  data[(PostgreSQL)]
  cache[(Redis)]

  clients --> gateway
  gateway --> control
  control --> providers
  gateway --> data
  gateway --> cache
```

## Tech stack

| Layer | Technology |
| --- | --- |
| Backend | Go `1.26.5` (`backend/go.mod`) |
| Frontend | Vue `^3.4`, Vite, TypeScript, pnpm (`frontend/package.json`) |
| Database | PostgreSQL（Compose: `postgres:18-alpine`） |
| Cache | Redis（Compose: `redis:8-alpine`） |
| Deployment | Docker / Docker Compose、Linux systemd、ソースビルド |

## Quick start

公開済みの ModuRelay Docker Hub / GHCR イメージはまだありません。ソースからビルドしてください。

### Prerequisites

- Docker と Docker Compose v2+
- またはローカルの Go + Node.js 環境（[Local development](#local-development)）

### Docker Compose でビルドして起動

```bash
git clone https://github.com/lien0219/modurelay.git
cd modurelay/deploy
cp .env.example .env
# .env を編集し、少なくとも POSTGRES_PASSWORD を設定（ADMIN_PASSWORD / JWT_SECRET も推奨）
docker compose -f docker-compose.dev.yml up --build -d
```

`SERVER_PORT`（既定 `8080`）でアクセスします。

> 現在の Compose サービス名、ボリューム、既定 DB 識別子は互換性のため旧名称を保持しています。目標命名は [BRANDING.md](BRANDING.md) を参照。未公開の ModuRelay イメージを `docker pull` しないでください。

詳細: [deploy/README.md](deploy/README.md)

## Local development

[DEV_GUIDE.md](DEV_GUIDE.md) も参照してください。

### Backend

必要: Go `1.26.5+`、PostgreSQL、Redis。

```bash
cd backend
go run ./cmd/server/
```

```bash
make -C backend build
make -C backend test-unit
cd backend && go generate ./ent
```

### Frontend

必要: Node.js と pnpm（`packageManager` は `pnpm@10.33.2`）。

```bash
cd frontend
pnpm install
pnpm dev
pnpm typecheck
pnpm build
pnpm test:run
```

```bash
make build
make test-frontend
make test-backend
```

> **注意:** `-tags embed` フラグはフロントエンドをバイナリに組み込みます。このフラグがない場合、バイナリはフロントエンド UI を提供しません。

**`config.yaml` の主要設定:**

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  mode: "release"

database:
  host: "localhost"
  port: 5432
  user: "postgres"
  password: "your_password"
  dbname: "sub2api"

redis:
  host: "localhost"
  port: 6379
  username: ""
  password: ""

jwt:
  secret: "change-this-to-a-secure-random-string"
  expire_hour: 24

default:
  user_concurrency: 5
  user_balance: 0
  api_key_prefix: "sk-"
  rate_multiplier: 1.0
```

`config.yaml` では CORS 許可リスト、上流 URL 許可リスト、レスポンスヘッダーフィルタリング、CSP、課金サーキットブレーカー、信頼済みプロキシ、カスタム転送クライアント IP ヘッダー、Turnstile 要件などのセキュリティ関連オプションも利用できます。

カスタムクライアント IP ヘッダーは環境変数でも設定できます:

```bash
SECURITY_FORWARDED_CLIENT_IP_HEADERS=True-Client-IP,X-CDN-Client-IP
```

本番環境では、ネットワーク境界が明確に制御されている場合を除き、安全でない HTTP 上流 URL を許可しないでください:

```bash
SECURITY_URL_ALLOWLIST_ENABLED=false
SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP=false
```

### Nginx reverse proxy note

Nginx で ModuRelay をリバースプロキシし、Codex CLI などのクライアントと組み合わせる場合は、アンダースコアを含むヘッダーが破棄されないよう Nginx の `http` ブロックに以下を追加してください:

```nginx
underscores_in_headers on;
```

## Branches and contribution

| Branch | Role |
| --- | --- |
| `develop` | 日常統合 |
| `main` | 安定リリース |
| `upstream-main` | 上流 `main` のミラーのみ（二開禁止） |
| `feature/*` / `fix/*` | `develop` へマージする作業ブランチ |

1. `develop` からブランチ作成
2. PR は `develop` 向け
3. 検証後に `main` へ昇格

ドキュメント:

- [docs/BRANCHING.md](docs/BRANCHING.md)
- [UPSTREAM.md](UPSTREAM.md)
- [CUSTOM_CHANGELOG.md](CUSTOM_CHANGELOG.md)
- [BRANDING.md](BRANDING.md)
- [NOTICE.md](NOTICE.md)

## Configuration

`deploy/.env.example` または `deploy/config.example.yaml` を使用してください。秘密情報をコミットしないでください。

| Variable | Purpose |
| --- | --- |
| `SERVER_PORT` | HTTP ポート（既定 `8080`） |
| `SERVER_MODE` | 例: `debug` |
| `RUN_MODE` | `standard` または `simple` |
| `DATABASE_HOST` / `DATABASE_PORT` / `DATABASE_USER` / `DATABASE_PASSWORD` / `DATABASE_DBNAME` | PostgreSQL 接続 |
| `REDIS_HOST` / `REDIS_PORT` / `REDIS_PASSWORD` / `REDIS_DB` | Redis 接続 |
| `ADMIN_EMAIL` / `ADMIN_PASSWORD` | 自動セットアップ時の管理者 |
| `JWT_SECRET` | JWT 署名秘密鍵 |
| `TOTP_ENCRYPTION_KEY` | 任意の TOTP 暗号化鍵 |
| `TZ` | タイムゾーン |

現時点で `MODURELAY_` プレフィックスの環境変数は未実装です。[BRANDING.md](BRANDING.md) を参照。

## Deployment

| Method | Notes |
| --- | --- |
| Docker Compose（ソースビルド） | 公式イメージ公開までは `deploy/docker-compose.dev.yml` を推奨 |
| Docker image build | ルート `Dockerfile` |
| Linux / systemd | `deploy/` 配下の unit。ModuRelay への改名は未完了 |
| Source run | `go run` / `make -C backend build` + フロントエンドビルド |

> *translation pending:* 正式なバイナリ / パッケージ / イメージ改名の進捗は [BRANDING.md](BRANDING.md) を正とします。

## Roadmap

計画（未実装として明示）:

- ModuRelay ブランド資産とデプロイ識別子の移行完了
- マルチテナント
- Langflow 連携ガイド
- ComfyUI 向けワークフロー接続パターン
- より高度なモデルルーティング戦略
- コスト分析
- 企業向けプライベート導入パッケージ
- Provider / プラグイン拡張点

## Security and compliance

これは ModuRelay プロジェクトとしての注意事項です。上流の商用認可に関する主張を再掲するものではありません。デプロイ前に [Important notice](#important-notice) を確認してください。

## Sponsors

上流 Sub2API のスポンサー広告・招待コード・プロモーションはここに転載しません。これらは ModuRelay のスポンサー関係を表すものではありません。

上流のスポンサー情報は [Sub2API リポジトリ](https://github.com/Wei-Shaw/sub2api) を参照してください。

## Contact

- GitHub Issues: [lien0219/modurelay/issues](https://github.com/lien0219/modurelay/issues)
- Repository: [lien0219/modurelay](https://github.com/lien0219/modurelay)

現時点で ModuRelay 専用の公式サイト、メール、Discord、チャットグループは公開していません。

## License and attribution

- [LICENSE](LICENSE)（GNU LGPL v3）に従います
- 上流の著作権とライセンス表示を保持します
- ModuRelay 固有の変更は [NOTICE.md](NOTICE.md) と [CUSTOM_CHANGELOG.md](CUSTOM_CHANGELOG.md) を参照
- `LICENSE` と上流著作権表示を削除しないでください
- ModuRelay は Sub2API 上流と公式な所属関係を持ちません
